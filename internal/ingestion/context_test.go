package ingestion

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestCheckpointWrapsContextErrorWithoutDetail(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("SENSITIVE-CANCELLATION-CAUSE"))
	err := Checkpoint(ctx)
	if !errors.Is(err, ErrCancellation) || !errors.Is(err, context.Canceled) {
		t.Fatalf("Checkpoint() error = %v, want cancellation and context cause", err)
	}
	if strings.Contains(err.Error(), "SENSITIVE") {
		t.Fatalf("Checkpoint() leaked cause: %q", err)
	}
}

func TestCheckpointNilAndLiveContexts(t *testing.T) {
	t.Parallel()

	var nilContext context.Context
	if err := Checkpoint(nilContext); err != nil {
		t.Fatalf("Checkpoint(nil) = %v", err)
	}
	if err := Checkpoint(context.Background()); err != nil {
		t.Fatalf("Checkpoint(live) = %v", err)
	}
}

func TestReadBoundedChecksCancellationBetweenReads(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelAfterRead{cancel: cancel}
	started := time.Now()
	got, err := ReadBounded(ctx, reader, 64)
	if !errors.Is(err, ErrCancellation) || !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadBounded() error = %v, want wrapped cancellation", err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("cooperative cancellation took %v", elapsed)
	}
	if got.Bytes != nil || got.ByteLength != 0 || got.Identity != "" {
		t.Fatalf("cancelled read returned object: %#v", got)
	}
}

func TestReadBoundedClassifiesReaderContextErrorsAsCancellation(t *testing.T) {
	t.Parallel()

	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		got, err := ReadBounded(context.Background(), &terminalReader{data: []byte("partial"), err: cause}, 64)
		if !errors.Is(err, ErrCancellation) || !errors.Is(err, cause) {
			t.Fatalf("reader context error %v classified as %v", cause, err)
		}
		if errors.Is(err, ErrIntegrityFailure) {
			t.Fatalf("reader context error %v misclassified as integrity", cause)
		}
		if got.Bytes != nil || got.ByteLength != 0 || got.Identity != "" {
			t.Fatalf("reader context error returned partial object: %#v", got)
		}
	}
}

type cancelAfterRead struct {
	cancel context.CancelFunc
	done   bool
}

func (r *cancelAfterRead) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	copy(p, "partial")
	r.cancel()
	return len("partial"), nil
}
