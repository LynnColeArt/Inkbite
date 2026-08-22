package inkbite

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestParseDataURIBase64(t *testing.T) {
	mediaType, attributes, data, err := parseDataURI("data:text/plain;charset=utf-8;base64,SGVsbG8=")
	if err != nil {
		t.Fatalf("parseDataURI() error = %v", err)
	}
	if mediaType != "text/plain" {
		t.Fatalf("expected text/plain, got %q", mediaType)
	}
	if attributes["charset"] != "utf-8" {
		t.Fatalf("expected utf-8 charset, got %q", attributes["charset"])
	}
	if string(data) != "Hello" {
		t.Fatalf("expected Hello, got %q", string(data))
	}
}

func TestParseDataURIPercentEncoding(t *testing.T) {
	mediaType, attributes, data, err := parseDataURI("data:text/plain;charset=UTF-8,A%20z%2B")
	if err != nil {
		t.Fatal(err)
	}
	if mediaType != "text/plain" || attributes["charset"] != "utf-8" || string(data) != "A z+" {
		t.Fatalf("parseDataURI() = %q %#v %q", mediaType, attributes, data)
	}
	secret := "SENSITIVE-PERCENT-PAYLOAD"
	_, _, _, err = parseDataURI("data:text/plain,%G0" + secret)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("percent diagnostic leaked: %v", err)
	}
}

func TestRemoteHTTPDisabledByDefault(t *testing.T) {
	engine := New()

	_, err := engine.ConvertURI(context.Background(), "https://example.com", nil, ConvertOptions{})
	if !errors.Is(err, ErrRemoteDisabled) {
		t.Fatalf("expected ErrRemoteDisabled, got %v", err)
	}
}

func TestFileURIToPath(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "unix path",
			raw:  "file:///tmp/hello.txt",
			want: "/tmp/hello.txt",
		},
	}

	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name string
			raw  string
			want string
		}{
			name: "windows drive path",
			raw:  "file:///C:/Users/test/hello.txt",
			want: "C:/Users/test/hello.txt",
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := url.Parse(test.raw)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			got, err := fileURIToPath(parsed)
			if err != nil {
				t.Fatalf("fileURIToPath() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestSourceLocatorFailuresAreRedacted(t *testing.T) {
	t.Parallel()

	secret := "SENSITIVE-SOURCE-LOCATOR"
	_, err := New().acquireSource(context.Background(), filepath.Join(t.TempDir(), secret, "missing.txt"), nil, ConvertOptions{}, 64)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("path diagnostic leaked: %v", err)
	}
	for _, raw := range []string{
		"file://user:" + secret + "@localhost/tmp/file.txt",
		"file://localhost/tmp/file.txt?token=" + secret,
		"file://" + secret + "/tmp/file.txt",
	} {
		_, err := New().acquireSource(context.Background(), raw, nil, ConvertOptions{}, 64)
		if err == nil || strings.Contains(err.Error(), secret) {
			t.Fatalf("file URI diagnostic leaked for %q: %v", raw, err)
		}
	}
}

func TestResolveURIRemoteHTTPWithinLimit(t *testing.T) {
	engine := New(WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/hello.txt" {
				t.Fatalf("expected /hello.txt path, got %q", req.URL.Path)
			}
			return &http.Response{
				StatusCode:    http.StatusOK,
				Header:        http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
				Body:          io.NopCloser(strings.NewReader("hello remote")),
				ContentLength: int64(len("hello remote")),
			}, nil
		}),
	}))
	resolved, err := engine.resolveURI(context.Background(), "https://example.com/hello.txt", nil, ConvertOptions{
		EnableHTTP:   true,
		MaxHTTPBytes: 64,
	})
	if err != nil {
		t.Fatalf("resolveURI() error = %v", err)
	}

	data, err := io.ReadAll(resolved.reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "hello remote" {
		t.Fatalf("expected hello remote, got %q", string(data))
	}
	if resolved.info.MIMEType != "text/plain" {
		t.Fatalf("expected text/plain MIME type, got %q", resolved.info.MIMEType)
	}
	if resolved.info.Filename != "hello.txt" {
		t.Fatalf("expected hello.txt filename, got %q", resolved.info.Filename)
	}
	if resolved.kind != SourceKindRemote || !resolved.callerTransport {
		t.Fatalf("remote kind/transport = %q/%v", resolved.kind, resolved.callerTransport)
	}
	if resolved.owned.Identity != internalingestion.Identity([]byte("hello remote")) || resolved.display != "https://example.com" {
		t.Fatalf("remote identity/display = %q/%q", resolved.owned.Identity, resolved.display)
	}
	if resolved.info.URL != "" || resolved.info.LocalPath != "" {
		t.Fatalf("remote retained authority-bearing info: %#v", resolved.info)
	}
}

func TestResolveURIRemoteHTTPRejectsOversizedBody(t *testing.T) {
	engine := New(WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/too-large.txt" {
				t.Fatalf("expected /too-large.txt path, got %q", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("a", 9))),
			}, nil
		}),
	}))
	_, err := engine.resolveURI(context.Background(), "https://example.com/too-large.txt", nil, ConvertOptions{
		EnableHTTP:   true,
		MaxHTTPBytes: 8,
	})
	if !errors.Is(err, ErrRemoteTooLarge) {
		t.Fatalf("expected ErrRemoteTooLarge, got %v", err)
	}
}

func TestAcquireSourceFormsUseOneBoundAndOwnExactBytes(t *testing.T) {
	t.Parallel()

	const limit = int64(8)
	temp := t.TempDir()
	atPath := filepath.Join(temp, "brief.txt")
	overPath := filepath.Join(temp, "over.txt")
	if err := os.WriteFile(atPath, []byte("12345678"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overPath, []byte("123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	atURIPath := filepath.ToSlash(atPath)
	overURIPath := filepath.ToSlash(overPath)
	if filepath.VolumeName(atPath) != "" {
		atURIPath = "/" + atURIPath
		overURIPath = "/" + overURIPath
	}
	atFileURI := (&url.URL{Scheme: "file", Path: atURIPath}).String()
	overFileURI := (&url.URL{Scheme: "file", Path: overURIPath}).String()

	tests := []struct {
		name     string
		at       any
		over     any
		wantKind SourceKind
	}{
		{name: "bytes", at: []byte("12345678"), over: []byte("123456789"), wantKind: SourceKindBytes},
		{name: "reader", at: strings.NewReader("12345678"), over: strings.NewReader("123456789"), wantKind: SourceKindReader},
		{name: "seeker", at: bytes.NewReader([]byte("12345678")), over: bytes.NewReader([]byte("123456789")), wantKind: SourceKindReader},
		{name: "path", at: atPath, over: overPath, wantKind: SourceKindFile},
		{name: "file URI", at: atFileURI, over: overFileURI, wantKind: SourceKindFile},
		{name: "data URI", at: "data:text/plain,12345678", over: "data:text/plain,123456789", wantKind: SourceKindDataURI},
		{name: "base64 data URI", at: "data:text/plain;base64,MTIzNDU2Nzg=", over: "data:text/plain;base64,MTIzNDU2Nzg5", wantKind: SourceKindDataURI},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := New()
			got, err := engine.acquireSource(context.Background(), tc.at, nil, ConvertOptions{}, limit)
			if err != nil {
				t.Fatal(err)
			}
			if string(got.owned.Bytes) != "12345678" || got.owned.ByteLength != limit || got.owned.Identity != internalingestion.Identity(got.owned.Bytes) {
				t.Fatalf("owned source = %#v", got.owned)
			}
			if got.kind != tc.wantKind || cap(got.owned.Bytes) != len(got.owned.Bytes) {
				t.Fatalf("kind/cap = %q/%d, want %q/%d", got.kind, cap(got.owned.Bytes), tc.wantKind, len(got.owned.Bytes))
			}
			if tc.wantKind == SourceKindBytes {
				tc.at.([]byte)[0] = 'X'
				if got.owned.Bytes[0] != '1' {
					t.Fatal("acquired bytes alias caller storage")
				}
			}

			failed, err := engine.acquireSource(context.Background(), tc.over, nil, ConvertOptions{}, limit)
			if !errors.Is(err, ErrLimitExceeded) {
				t.Fatalf("+1 error = %v, want limit", err)
			}
			if failed.owned.Bytes != nil || failed.reader != nil {
				t.Fatalf("+1 returned partial source: %#v", failed)
			}
		})
	}
}

type countingConverter struct {
	acceptCalls atomic.Int64
}

func (*countingConverter) Name() string      { return "source-boundary-counter" }
func (*countingConverter) Priority() float64 { return 1 }
func (c *countingConverter) Accepts(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) bool {
	c.acceptCalls.Add(1)
	return false
}
func (*countingConverter) Convert(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) (Result, error) {
	return Result{}, errors.New("unexpected converter dispatch")
}

func TestSourceDefaultLimitStopsConverterDispatchAtLimitPlusOne(t *testing.T) {
	converter := &countingConverter{}
	engine := New()
	engine.RegisterConverter(converter)

	atLimit := make([]byte, DefaultMaxSourceBytes)
	_, err := engine.Convert(context.Background(), atLimit, nil, ConvertOptions{})
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("at-limit conversion error = %v, want unsupported after dispatch", err)
	}
	if converter.acceptCalls.Load() != 1 {
		t.Fatalf("at-limit Accepts calls = %d, want one", converter.acceptCalls.Load())
	}

	_, err = engine.Convert(context.Background(), make([]byte, DefaultMaxSourceBytes+1), nil, ConvertOptions{})
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("limit+1 conversion error = %v, want limit", err)
	}
	if converter.acceptCalls.Load() != 1 {
		t.Fatalf("limit+1 reached converter: Accepts calls = %d", converter.acceptCalls.Load())
	}
}

func TestAcquireSourceFactsAreSafeAndOriginPreserving(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	secret := "SENSITIVE-ABSOLUTE-PATH"
	filePath := filepath.Join(temp, secret, "Brief.TXT")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := New().acquireSource(context.Background(), filePath, &StreamInfo{
		MIMEType: "text/plain; charset=utf-8",
		Filename: "caller.txt",
	}, ConvertOptions{}, 64)
	if err != nil {
		t.Fatal(err)
	}
	if got.display != "Brief.TXT" || strings.Contains(got.display, secret) || got.info.LocalPath != "" || got.info.URL != "" {
		t.Fatalf("unsafe retained display/info: display=%q info=%#v", got.display, got.info)
	}
	wantFacts := []internalingestion.Fact{
		{Kind: "filename", Value: "Brief.TXT", Origin: internalingestion.OriginSource},
		{Kind: "extension", Value: ".txt", Origin: internalingestion.OriginSource},
		{Kind: "media_type", Value: "text/plain", Origin: internalingestion.OriginCaller},
		{Kind: "charset", Value: "utf-8", Origin: internalingestion.OriginCaller},
		{Kind: "filename", Value: "caller.txt", Origin: internalingestion.OriginCaller},
	}
	if fmtFacts(got.facts) != fmtFacts(wantFacts) {
		t.Fatalf("facts = %#v, want %#v", got.facts, wantFacts)
	}

	sniffed := StreamInfo{MIMEType: "application/pdf", Extension: ".pdf"}
	facts := safeStreamFacts(StreamInfo{}, StreamInfo{}, sniffed)
	for _, fact := range facts {
		if fact.Origin != internalingestion.OriginSniff {
			t.Fatalf("sniff fact origin = %q", fact.Origin)
		}
	}
}

func fmtFacts(facts []internalingestion.Fact) string {
	var builder strings.Builder
	for _, fact := range facts {
		builder.WriteString(string(fact.Origin))
		builder.WriteByte(':')
		builder.WriteString(fact.Kind)
		builder.WriteByte('=')
		builder.WriteString(fact.Value)
		builder.WriteByte(';')
	}
	return builder.String()
}

type preCancelReader struct {
	readCalls atomic.Int64
}

func (r *preCancelReader) Read([]byte) (int, error) {
	r.readCalls.Add(1)
	return 0, io.EOF
}

type preCancelReadSeeker struct {
	preCancelReader
	seekCalls atomic.Int64
}

func (r *preCancelReadSeeker) Seek(int64, int) (int64, error) {
	r.seekCalls.Add(1)
	return 0, nil
}

func TestSourcePreCanceledPerformsNoCallerCalls(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := &preCancelReader{}
	result, err := New().Convert(ctx, reader, nil, ConvertOptions{})
	assertCanceledZeroResult(t, result, err)
	if reader.readCalls.Load() != 0 {
		t.Fatalf("pre-canceled reader Read calls = %d, want zero", reader.readCalls.Load())
	}

	seeker := &preCancelReadSeeker{}
	result, err = New().Convert(ctx, seeker, nil, ConvertOptions{})
	assertCanceledZeroResult(t, result, err)
	if seeker.seekCalls.Load() != 0 || seeker.readCalls.Load() != 0 {
		t.Fatalf("pre-canceled read-seeker calls seek/read = %d/%d, want zero", seeker.seekCalls.Load(), seeker.readCalls.Load())
	}
}

type synchronizedReadCloser struct {
	entered     chan struct{}
	release     chan struct{}
	readExited  chan struct{}
	closeExited chan struct{}
	enterOnce   sync.Once
	closeOnce   sync.Once
}

func newSynchronizedReadCloser() *synchronizedReadCloser {
	return &synchronizedReadCloser{
		entered:     make(chan struct{}),
		release:     make(chan struct{}),
		readExited:  make(chan struct{}),
		closeExited: make(chan struct{}),
	}
}

func (r *synchronizedReadCloser) Read([]byte) (int, error) {
	r.enterOnce.Do(func() { close(r.entered) })
	<-r.release
	close(r.readExited)
	return 0, errors.New("SENSITIVE cooperative blocking reader")
}

func (r *synchronizedReadCloser) Close() error {
	r.closeOnce.Do(func() {
		close(r.release)
		close(r.closeExited)
	})
	return nil
}

type synchronizedReadSeekCloser struct {
	*synchronizedReadCloser
	seekCalls atomic.Int64
}

func (r *synchronizedReadSeekCloser) Seek(int64, int) (int64, error) {
	r.seekCalls.Add(1)
	return 0, nil
}

func TestSourceCooperativeBlockingCallsCloseAndJoin(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		source func(*synchronizedReadCloser) any
	}{
		{name: "reader", source: func(r *synchronizedReadCloser) any { return r }},
		{name: "read seeker", source: func(r *synchronizedReadCloser) any {
			return &synchronizedReadSeekCloser{synchronizedReadCloser: r}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := newSynchronizedReadCloser()
			t.Cleanup(func() { _ = reader.Close() })
			ctx, cancel := context.WithCancel(context.Background())
			done := convertAsync(ctx, tc.source(reader))
			<-reader.entered
			cancel()
			outcome := waitConvertOutcome(t, done, "cooperative source did not terminate after cancellation")
			assertCanceledZeroResult(t, outcome.result, outcome.err)
			assertChannelClosed(t, reader.readExited, "Read was not joined before API return")
			assertChannelClosed(t, reader.closeExited, "Close was not joined before API return")
			if strings.Contains(outcome.err.Error(), "SENSITIVE") {
				t.Fatalf("cooperative cancellation leaked diagnostic: %v", outcome.err)
			}
		})
	}
}

type contextAwareReader struct {
	ctx     context.Context
	entered chan struct{}
	exited  chan struct{}
}

func (r *contextAwareReader) Read([]byte) (int, error) {
	close(r.entered)
	<-r.ctx.Done()
	close(r.exited)
	return 0, r.ctx.Err()
}

func TestSourceContextAwareNonClosingReaderJoins(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	reader := &contextAwareReader{ctx: ctx, entered: make(chan struct{}), exited: make(chan struct{})}
	done := convertAsync(ctx, reader)
	<-reader.entered
	cancel()
	outcome := waitConvertOutcome(t, done, "context-aware reader did not terminate")
	assertCanceledZeroResult(t, outcome.result, outcome.err)
	assertChannelClosed(t, reader.exited, "context-aware Read was not joined")
}

type synchronizedNonCooperativeReader struct {
	entered chan struct{}
	release chan struct{}
	exited  chan struct{}
}

func newSynchronizedNonCooperativeReader() *synchronizedNonCooperativeReader {
	return &synchronizedNonCooperativeReader{entered: make(chan struct{}), release: make(chan struct{}), exited: make(chan struct{})}
}

func (r *synchronizedNonCooperativeReader) Read([]byte) (int, error) {
	close(r.entered)
	<-r.release
	close(r.exited)
	return 0, io.EOF
}

type synchronizedNonCooperativeReadSeeker struct {
	*synchronizedNonCooperativeReader
}

func (*synchronizedNonCooperativeReadSeeker) Seek(int64, int) (int64, error) { return 0, nil }

func TestSourceNonCooperativeReadRemainsJoinedUntilRelease(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		source func(*synchronizedNonCooperativeReader) any
	}{
		{name: "reader", source: func(r *synchronizedNonCooperativeReader) any { return r }},
		{name: "read seeker", source: func(r *synchronizedNonCooperativeReader) any {
			return &synchronizedNonCooperativeReadSeeker{r}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := newSynchronizedNonCooperativeReader()
			t.Cleanup(func() { closeChannelIfOpen(reader.release) })
			ctx, cancel := context.WithCancel(context.Background())
			done := convertAsync(ctx, tc.source(reader))
			<-reader.entered
			cancel()
			assertCallStillJoined(t, done)
			close(reader.release)
			outcome := waitConvertOutcome(t, done, "non-cooperative Read did not return after release")
			assertCanceledZeroResult(t, outcome.result, outcome.err)
			assertChannelClosed(t, reader.exited, "non-cooperative Read was abandoned")
		})
	}
}

type synchronizedBlockingSeeker struct {
	seekEntered chan struct{}
	seekRelease chan struct{}
	seekExited  chan struct{}
	readCalls   atomic.Int64
}

func (r *synchronizedBlockingSeeker) Seek(int64, int) (int64, error) {
	close(r.seekEntered)
	<-r.seekRelease
	close(r.seekExited)
	return 0, nil
}

func (r *synchronizedBlockingSeeker) Read([]byte) (int, error) {
	r.readCalls.Add(1)
	return 0, io.EOF
}

func TestSourceNonCooperativeSeekRemainsJoinedUntilRelease(t *testing.T) {
	t.Parallel()

	seeker := &synchronizedBlockingSeeker{seekEntered: make(chan struct{}), seekRelease: make(chan struct{}), seekExited: make(chan struct{})}
	t.Cleanup(func() { closeChannelIfOpen(seeker.seekRelease) })
	ctx, cancel := context.WithCancel(context.Background())
	done := convertAsync(ctx, seeker)
	<-seeker.seekEntered
	cancel()
	assertCallStillJoined(t, done)
	close(seeker.seekRelease)
	outcome := waitConvertOutcome(t, done, "non-cooperative Seek did not return after release")
	assertCanceledZeroResult(t, outcome.result, outcome.err)
	assertChannelClosed(t, seeker.seekExited, "non-cooperative Seek was abandoned")
	if seeker.readCalls.Load() != 0 {
		t.Fatalf("Read calls after canceled Seek = %d, want zero", seeker.readCalls.Load())
	}
}

type partialThenBlockingReader struct {
	reads   atomic.Int64
	entered chan struct{}
	release chan struct{}
	exited  chan struct{}
}

func (r *partialThenBlockingReader) Read(output []byte) (int, error) {
	if r.reads.Add(1) == 1 {
		return copy(output, "partial-SENSITIVE"), nil
	}
	close(r.entered)
	<-r.release
	close(r.exited)
	return 0, io.EOF
}

func TestSourceCancellationDiscardsPartialPrefixAndJoins(t *testing.T) {
	t.Parallel()

	reader := &partialThenBlockingReader{entered: make(chan struct{}), release: make(chan struct{}), exited: make(chan struct{})}
	t.Cleanup(func() { closeChannelIfOpen(reader.release) })
	ctx, cancel := context.WithCancel(context.Background())
	done := convertAsync(ctx, reader)
	<-reader.entered
	cancel()
	assertCallStillJoined(t, done)
	close(reader.release)
	outcome := waitConvertOutcome(t, done, "partial reader did not return after release")
	assertCanceledZeroResult(t, outcome.result, outcome.err)
	assertChannelClosed(t, reader.exited, "partial reader was abandoned")
	if strings.Contains(outcome.err.Error(), "partial-SENSITIVE") {
		t.Fatalf("partial source escaped in diagnostic: %v", outcome.err)
	}
}

func TestSourceReadBoundaryHasOnlyJoinedCloseWatcher(t *testing.T) {
	t.Parallel()

	parsed, err := parser.ParseFile(token.NewFileSet(), "source.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	workers := 0
	found := false
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "readSourceBounded" {
			continue
		}
		found = true
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if _, ok := node.(*ast.GoStmt); ok {
				workers++
			}
			return true
		})
	}
	if !found {
		t.Fatal("readSourceBounded production boundary not found")
	}
	// The sole worker is the cancellation watcher for a cooperative Closer;
	// the runtime matrix proves it is joined. A second worker would race an
	// arbitrary Read and permit the public call to abandon it on cancellation.
	if workers != 1 {
		t.Fatalf("readSourceBounded worker count = %d, want one joined close watcher", workers)
	}
}

type convertOutcome struct {
	result Result
	err    error
}

func convertAsync(ctx context.Context, source any) <-chan convertOutcome {
	done := make(chan convertOutcome, 1)
	go func() {
		result, err := New().Convert(ctx, source, nil, ConvertOptions{})
		done <- convertOutcome{result: result, err: err}
	}()
	return done
}

func waitConvertOutcome(t *testing.T, done <-chan convertOutcome, message string) convertOutcome {
	t.Helper()
	select {
	case outcome := <-done:
		return outcome
	case <-time.After(time.Second):
		t.Fatal(message)
		return convertOutcome{}
	}
}

func assertCallStillJoined(t *testing.T, done <-chan convertOutcome) {
	t.Helper()
	select {
	case outcome := <-done:
		t.Fatalf("API returned before caller-owned method exited: result=%#v err=%v", outcome.result, outcome.err)
	default:
	}
}

func assertCanceledZeroResult(t *testing.T, result Result, err error) {
	t.Helper()
	if result != (Result{}) {
		t.Fatalf("canceled conversion returned partial result: %#v", result)
	}
	if !errors.Is(err, ErrCancellation) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v, want ErrCancellation and context.Canceled", err)
	}
}

func assertChannelClosed(t *testing.T, channel <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-channel:
	default:
		t.Fatal(message)
	}
}

func closeChannelIfOpen(channel chan struct{}) {
	select {
	case <-channel:
	default:
		close(channel)
	}
}

func TestParseDataURIRedactsPayloadFailures(t *testing.T) {
	t.Parallel()

	secret := "SENSITIVE-DATA-PAYLOAD"
	_, err := New().acquireSource(context.Background(), "data:text/plain;base64,"+secret, nil, ConvertOptions{}, 64)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("data URI diagnostic leaked: %v", err)
	}
}
