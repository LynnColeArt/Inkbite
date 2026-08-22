package ingestion

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
)

func TestReadBoundedBoundaryAndIdentity(t *testing.T) {
	t.Parallel()

	const limit = int64(8)
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "empty", input: ""},
		{name: "below", input: "1234567"},
		{name: "at limit", input: "12345678"},
		{name: "one over", input: "123456789", wantErr: ErrLimitExceeded},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadBounded(context.Background(), strings.NewReader(tc.input), limit)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ReadBounded() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.Bytes != nil || got.ByteLength != 0 || got.Identity != "" {
					t.Fatalf("failed read returned partial object: %#v", got)
				}
				return
			}
			if string(got.Bytes) != tc.input || got.ByteLength != int64(len(tc.input)) {
				t.Fatalf("ReadBounded() = %#v, want exact input", got)
			}
			if got.Identity != Identity([]byte(tc.input)) {
				t.Fatalf("identity = %q, want digest over exact bytes", got.Identity)
			}
		})
	}
}

func TestReadBoundedOwnsAcceptedBytes(t *testing.T) {
	t.Parallel()

	source := []byte("owned source")
	got, err := OwnBounded(context.Background(), source, int64(len(source)))
	if err != nil {
		t.Fatal(err)
	}
	source[0] = 'X'
	if string(got.Bytes) != "owned source" {
		t.Fatalf("returned bytes alias caller input: %q", got.Bytes)
	}

	clone := got.Clone()
	got.Bytes[1] = 'Y'
	if string(clone.Bytes) != "owned source" {
		t.Fatalf("Clone() aliases original: %q", clone.Bytes)
	}
}

type terminalReader struct {
	data []byte
	err  error
}

func (r *terminalReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.err
	}
	return n, nil
}

func TestReadBoundedRejectsPartialAndInvalidReaders(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("SENSITIVE-SOURCE-BYTES")
	tests := []struct {
		name   string
		reader io.Reader
	}{
		{name: "terminal error with bytes", reader: &terminalReader{data: []byte("partial"), err: sentinel}},
		{name: "no progress", reader: zeroReader{}},
		{name: "nil reader", reader: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadBounded(context.Background(), tc.reader, 64)
			if !errors.Is(err, ErrIntegrityFailure) {
				t.Fatalf("ReadBounded() error = %v, want integrity", err)
			}
			if strings.Contains(err.Error(), sentinel.Error()) {
				t.Fatalf("error leaked reader payload: %q", err)
			}
			if got.Bytes != nil || got.ByteLength != 0 || got.Identity != "" {
				t.Fatalf("failed read returned partial object: %#v", got)
			}
		})
	}
}

type zeroReader struct{}

func (zeroReader) Read([]byte) (int, error) { return 0, nil }

func TestReadBoundedShortReadsAndOverflowProbe(t *testing.T) {
	t.Parallel()

	reader := &oneByteReader{data: []byte("12345")}
	got, err := ReadBounded(context.Background(), reader, 4)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("ReadBounded() error = %v, want limit", err)
	}
	if reader.reads != 5 {
		t.Fatalf("reads = %d, want exact limit+1 probe", reader.reads)
	}
	if got.Bytes != nil || got.ByteLength != 0 || got.Identity != "" {
		t.Fatalf("overflow returned partial object: %#v", got)
	}
}

func TestReadBoundedDoesNotOverallocateReturnedStorage(t *testing.T) {
	t.Parallel()

	const limit = 33_000
	got, err := ReadBounded(context.Background(), strings.NewReader(strings.Repeat("x", limit)), limit)
	if err != nil {
		t.Fatal(err)
	}
	if got.ByteLength != limit {
		t.Fatalf("byte length = %d, want %d", got.ByteLength, limit)
	}
	if int64(cap(got.Bytes)) > limit {
		t.Fatalf("returned capacity = %d, exceeds policy limit %d", cap(got.Bytes), limit)
	}
}

func TestReadBoundedClipsReaderWindowsAndAcceptedCapacity(t *testing.T) {
	t.Parallel()

	const chunkSize = 32 << 10
	tests := []struct {
		name    string
		limit   int64
		input   string
		wantErr error
	}{
		{name: "zero", limit: 0},
		{name: "small short EOF", limit: 8, input: "abc"},
		{name: "chunk boundary", limit: chunkSize, input: strings.Repeat("x", chunkSize)},
		{name: "exact small limit", limit: 8, input: "12345678"},
		{name: "small limit plus one", limit: 8, input: "123456789", wantErr: ErrLimitExceeded},
		{name: "maximum integer short EOF", limit: math.MaxInt64, input: "max"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &windowRecordingReader{data: []byte(tc.input)}
			got, err := ReadBounded(context.Background(), reader, tc.limit)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ReadBounded() error = %v, want %v", err, tc.wantErr)
			}
			if len(reader.windows) == 0 {
				t.Fatal("reader received no bounded window")
			}
			wantFirstWindow := int64(chunkSize)
			if tc.limit < chunkSize {
				wantFirstWindow = tc.limit + 1
			}
			if gotWindow := reader.windows[0]; int64(gotWindow.length) != wantFirstWindow {
				t.Fatalf("first reader window length = %d, want %d", gotWindow.length, wantFirstWindow)
			}
			for i, window := range reader.windows {
				if window.capacity != window.length {
					t.Errorf("reader window %d len/cap = %d/%d, want equal", i, window.length, window.capacity)
				}
				if int64(window.length) > wantFirstWindow {
					t.Errorf("reader window %d length = %d, exceeds %d", i, window.length, wantFirstWindow)
				}
			}
			if reader.accessedBeyondWindow {
				t.Fatal("hostile reader could access scratch storage beyond its advertised window")
			}
			if tc.wantErr != nil {
				if got.Bytes != nil || got.ByteLength != 0 || got.Identity != "" {
					t.Fatalf("failed read returned partial object: %#v", got)
				}
				return
			}
			if string(got.Bytes) != tc.input || got.ByteLength != int64(len(tc.input)) {
				t.Fatalf("ReadBounded() = %#v, want exact input", got)
			}
			if cap(got.Bytes) != len(got.Bytes) {
				t.Fatalf("accepted bytes len/cap = %d/%d, want clipped", len(got.Bytes), cap(got.Bytes))
			}
		})
	}
}

func TestBoundedScratchCapacityArithmetic(t *testing.T) {
	t.Parallel()

	const chunkSize = 32 << 10
	tests := []struct {
		limit int64
		want  int
	}{
		{limit: 0, want: 1},
		{limit: 8, want: 9},
		{limit: chunkSize - 1, want: chunkSize},
		{limit: chunkSize, want: chunkSize},
		{limit: math.MaxInt64, want: chunkSize},
	}
	for _, tc := range tests {
		if got := boundedScratchCapacity(tc.limit); got != tc.want {
			t.Errorf("boundedScratchCapacity(%d) = %d, want %d", tc.limit, got, tc.want)
		}
	}
}

type readerWindow struct {
	length   int
	capacity int
}

type windowRecordingReader struct {
	data                 []byte
	windows              []readerWindow
	accessedBeyondWindow bool
}

func (r *windowRecordingReader) Read(p []byte) (int, error) {
	r.windows = append(r.windows, readerWindow{length: len(p), capacity: cap(p)})
	if cap(p) > len(p) {
		exposed := p[:cap(p)]
		exposed[len(p)] = 0xa5
		r.accessedBeyondWindow = true
	}
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

type oneByteReader struct {
	data  []byte
	reads int
}

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	p[0] = r.data[0]
	r.data = r.data[1:]
	r.reads++
	return 1, nil
}

func TestOwnBoundedRejectsInvalidLimitAndCancellation(t *testing.T) {
	t.Parallel()

	if _, err := OwnBounded(context.Background(), []byte("x"), -1); !errors.Is(err, ErrPolicyViolation) {
		t.Fatalf("negative limit error = %v, want policy", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := OwnBounded(ctx, []byte("x"), 1)
	if !errors.Is(err, ErrCancellation) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v, want wrapped cancellation", err)
	}
	if got.Bytes != nil || got.ByteLength != 0 || got.Identity != "" {
		t.Fatalf("cancelled operation returned object: %#v", got)
	}
}

func TestIdentityCanonicalSHA256(t *testing.T) {
	t.Parallel()

	const emptySHA256 = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got := Identity(nil); got != emptySHA256 {
		t.Fatalf("Identity(nil) = %q, want %q", got, emptySHA256)
	}
	if !bytes.Equal([]byte(Identity([]byte("a"))[:7]), []byte("sha256:")) {
		t.Fatal("identity lost canonical prefix")
	}
}
