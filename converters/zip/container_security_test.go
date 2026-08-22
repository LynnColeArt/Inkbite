package zipconv_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

type zipMember struct {
	name   string
	body   []byte
	method uint16
	mode   os.FileMode
}

func TestZIPRejectsUnsafeMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		members []zipMember
	}{
		{name: "traversal", members: []zipMember{{name: "../secret.txt", body: []byte("secret")}}},
		{name: "backslash", members: []zipMember{{name: `folder\secret.txt`, body: []byte("secret")}}},
		{name: "absolute", members: []zipMember{{name: "/secret.txt", body: []byte("secret")}}},
		{name: "nul", members: []zipMember{{name: "secret\x00.txt", body: []byte("secret")}}},
		{name: "duplicate", members: []zipMember{{name: "note.txt", body: []byte("one")}, {name: "note.txt", body: []byte("two")}}},
		{name: "portable case collision", members: []zipMember{{name: "Note.txt", body: []byte("one")}, {name: "note.txt", body: []byte("two")}}},
		{name: "symlink", members: []zipMember{{name: "link.txt", body: []byte("target"), mode: os.ModeSymlink | 0o777}}},
		{name: "special file", members: []zipMember{{name: "pipe", mode: os.ModeNamedPipe | 0o600}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ingestZIP(t, context.Background(), buildZIP(t, tc.members), inkbite.DefaultIngestionPolicy())
			if !errors.Is(err, inkbite.ErrMalformedInput) {
				t.Fatalf("Ingest() error = %v, want malformed input", err)
			}
		})
	}
}

func TestZIPPolicyBoundariesUseActualBytes(t *testing.T) {
	t.Parallel()

	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxContainerEntries = 2
	policy.MaxContainerEntryBytes = 5
	policy.MaxExpandedBytes = 5
	policy.MaxContainerDepth = 2
	policy.MaxExpansionRatio = 1000

	tests := []struct {
		name    string
		members []zipMember
		wantErr bool
	}{
		{name: "entry and total at limit", members: []zipMember{{name: "note.txt", body: []byte("12345"), method: zip.Store}}},
		{name: "entry and total limit plus one", members: []zipMember{{name: "note.txt", body: []byte("123456"), method: zip.Store}}, wantErr: true},
		{name: "entry count at limit", members: []zipMember{{name: "a.txt", body: []byte("12"), method: zip.Store}, {name: "b.txt", body: []byte("345"), method: zip.Store}}},
		{name: "entry count limit plus one", members: []zipMember{{name: "a.txt", body: []byte("1"), method: zip.Store}, {name: "b.txt", body: []byte("2"), method: zip.Store}, {name: "c.txt", body: []byte("3"), method: zip.Store}}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ingestZIP(t, context.Background(), buildZIP(t, tc.members), policy)
			if tc.wantErr && !errors.Is(err, inkbite.ErrLimitExceeded) {
				t.Fatalf("Ingest() error = %v, want limit exceeded", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Ingest() error = %v", err)
			}
		})
	}
}

func TestZIPRejectsExpansionRatioLimitPlusOne(t *testing.T) {
	t.Parallel()

	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxExpansionRatio = 1
	archive := buildZIP(t, []zipMember{{name: "note.txt", body: bytes.Repeat([]byte("a"), 4096), method: zip.Deflate}})
	_, err := ingestZIP(t, context.Background(), archive, policy)
	if !errors.Is(err, inkbite.ErrLimitExceeded) {
		t.Fatalf("Ingest() error = %v, want expansion ratio limit", err)
	}
}

func TestZIPForgedHeaderCannotBypassActualByteLimit(t *testing.T) {
	t.Parallel()

	archive := buildZIP(t, []zipMember{{name: "note.txt", body: []byte("123456"), method: zip.Store}})
	forgeCentralUncompressedSize(t, archive, 1)
	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxContainerEntryBytes = 5

	_, err := ingestZIP(t, context.Background(), archive, policy)
	if !errors.Is(err, inkbite.ErrIntegrityFailure) {
		t.Fatalf("Ingest() error = %v, want integrity failure for contradictory size claim", err)
	}
}

func TestZIPReadsThroughEOFAuthoritatively(t *testing.T) {
	t.Parallel()

	archive := buildZIP(t, []zipMember{{name: "note.txt", body: []byte("checksum payload"), method: zip.Store}})
	corruptStoredPayload(t, archive, []byte("checksum payload"))
	_, err := ingestZIP(t, context.Background(), archive, inkbite.DefaultIngestionPolicy())
	if !errors.Is(err, inkbite.ErrIntegrityFailure) {
		t.Fatalf("Ingest() error = %v, want integrity failure", err)
	}
}

func TestZIPDetailedWarningsExposeMemberDegradation(t *testing.T) {
	t.Parallel()

	archive := buildZIP(t, []zipMember{
		{name: "unsupported.bin", body: []byte{0xff, 0xfe, 0xfd}, method: zip.Store},
		{name: "broken.pdf", body: []byte("not a pdf"), method: zip.Store},
		{name: "good.txt", body: []byte("kept"), method: zip.Store},
	})
	envelope, err := ingestZIP(t, context.Background(), archive, inkbite.DefaultIngestionPolicy())
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if !strings.Contains(string(envelope.Primary.Bytes), "kept") {
		t.Fatalf("primary markdown = %q, want supported member", envelope.Primary.Bytes)
	}
	want := []inkbite.WarningRecord{
		{Category: "converter_fallback", Converter: "pdf", Location: "broken.pdf", Detail: "converter failure"},
		{Category: "unsupported_member", Converter: "zip", Location: "unsupported.bin"},
	}
	if len(envelope.Warnings) != len(want) {
		t.Fatalf("warnings = %#v, want %#v", envelope.Warnings, want)
	}
	for index := range want {
		if envelope.Warnings[index] != want[index] {
			t.Fatalf("warnings[%d] = %#v, want %#v", index, envelope.Warnings[index], want[index])
		}
	}
}

func TestZIPCancellationReturnsNoEnvelope(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	envelope, err := ingestZIP(t, ctx, buildZIP(t, []zipMember{{name: "note.txt", body: []byte("ignored")}}), inkbite.DefaultIngestionPolicy())
	if !errors.Is(err, inkbite.ErrCancellation) {
		t.Fatalf("Ingest() error = %v, want cancellation", err)
	}
	if !reflect.DeepEqual(envelope, inkbite.IngestionEnvelope{}) {
		t.Fatalf("Ingest() envelope = %#v, want zero", envelope)
	}
}

func TestZIPNestedConversionDoesNotDoubleAccountSourceOrPrimary(t *testing.T) {
	t.Parallel()

	archive := buildZIP(t, []zipMember{{name: "note.txt", body: []byte("ok"), method: zip.Store}})
	wantMarkdown := "Content from zip file `bundle.zip`\n\n## File: note.txt\n\nok"
	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxSourceBytes = int64(len(archive))
	policy.MaxPrimaryBytes = int64(len(wantMarkdown))

	envelope, err := ingestZIP(t, context.Background(), archive, policy)
	if err != nil {
		t.Fatalf("Ingest() at exact outer source/primary limits error = %v", err)
	}
	if got := string(envelope.Primary.Bytes); got != wantMarkdown {
		t.Fatalf("primary markdown = %q, want %q", got, wantMarkdown)
	}
}

func TestZIPNestedConversionInheritsOuterContainerPolicy(t *testing.T) {
	t.Parallel()

	inner := buildZIP(t, []zipMember{{name: "leaf.txt", body: []byte("leaf"), method: zip.Store}})
	outer := buildZIP(t, []zipMember{{name: "nested.zip", body: inner, method: zip.Store}})
	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxContainerDepth = 2
	if _, err := ingestZIP(t, context.Background(), outer, policy); err != nil {
		t.Fatalf("Ingest() at nested depth limit error = %v", err)
	}
	policy.MaxContainerDepth = 1

	_, err := ingestZIP(t, context.Background(), outer, policy)
	if !errors.Is(err, inkbite.ErrLimitExceeded) {
		t.Fatalf("Ingest() nested depth error = %v, want outer policy limit", err)
	}
}

func TestZIPNestedConversionSharesAggregateLedger(t *testing.T) {
	t.Parallel()

	inner := buildZIP(t, []zipMember{{name: "leaf.txt", body: []byte("leaf"), method: zip.Store}})
	outer := buildZIP(t, []zipMember{{name: "nested.zip", body: inner, method: zip.Store}})

	tests := []struct {
		name   string
		policy func() inkbite.IngestionPolicy
	}{
		{name: "entry count", policy: func() inkbite.IngestionPolicy {
			policy := inkbite.DefaultIngestionPolicy()
			policy.MaxContainerEntries = 1
			return policy
		}},
		{name: "expanded bytes", policy: func() inkbite.IngestionPolicy {
			policy := inkbite.DefaultIngestionPolicy()
			policy.MaxExpandedBytes = int64(len(inner))
			return policy
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ingestZIP(t, context.Background(), outer, tc.policy())
			if !errors.Is(err, inkbite.ErrLimitExceeded) {
				t.Fatalf("Ingest() error = %v, want shared aggregate limit", err)
			}
		})
	}
}

func TestZIPRequestBudgetsAreIsolated(t *testing.T) {
	t.Parallel()

	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)
	accepted := buildZIP(t, []zipMember{{name: "note.txt", body: []byte("accepted"), method: zip.Store}})
	rejected := buildZIP(t, []zipMember{{name: "a.txt", body: []byte("a"), method: zip.Store}, {name: "b.txt", body: []byte("b"), method: zip.Store}})
	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxContainerEntries = 1

	const requests = 64
	var wait sync.WaitGroup
	wait.Add(requests)
	for index := 0; index < requests; index++ {
		index := index
		go func() {
			defer wait.Done()
			archive := accepted
			wantLimit := index%2 == 1
			if wantLimit {
				archive = rejected
			}
			envelope, err := engine.Ingest(context.Background(), archive, &inkbite.StreamInfo{Extension: ".zip", Filename: "race.zip"}, inkbite.IngestOptions{Policy: policy})
			if wantLimit {
				if !errors.Is(err, inkbite.ErrLimitExceeded) {
					t.Errorf("request %d error = %v, want limit", index, err)
				}
				return
			}
			if err != nil {
				t.Errorf("request %d error = %v", index, err)
				return
			}
			if !strings.Contains(string(envelope.Primary.Bytes), "accepted") {
				t.Errorf("request %d primary = %q", index, envelope.Primary.Bytes)
			}
		}()
	}
	wait.Wait()
}

func ingestZIP(t *testing.T, ctx context.Context, archive []byte, policy inkbite.IngestionPolicy) (inkbite.IngestionEnvelope, error) {
	t.Helper()
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)
	return engine.Ingest(ctx, archive, &inkbite.StreamInfo{Extension: ".zip", Filename: "bundle.zip"}, inkbite.IngestOptions{Policy: policy})
}

func buildZIP(t *testing.T, members []zipMember) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, member := range members {
		header := &zip.FileHeader{Name: member.name, Method: member.method}
		if member.mode != 0 {
			header.SetMode(member.mode)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("CreateHeader(%q) error = %v", member.name, err)
		}
		if _, err := entry.Write(member.body); err != nil {
			t.Fatalf("Write(%q) error = %v", member.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return bytes.Clone(buffer.Bytes())
}

func forgeCentralUncompressedSize(t *testing.T, archive []byte, size uint32) {
	t.Helper()
	index := bytes.Index(archive, []byte{'P', 'K', 1, 2})
	if index < 0 || index+28 > len(archive) {
		t.Fatal("central directory header not found")
	}
	binary.LittleEndian.PutUint32(archive[index+24:index+28], size)
}

func corruptStoredPayload(t *testing.T, archive, payload []byte) {
	t.Helper()
	index := bytes.Index(archive, payload)
	if index < 0 {
		t.Fatal("stored payload not found")
	}
	archive[index] ^= 0xff
}
