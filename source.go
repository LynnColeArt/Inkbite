package inkbite

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
)

type resolvedSource struct {
	reader          *bytes.Reader
	info            StreamInfo
	owned           internalingestion.OwnedBytes
	kind            SourceKind
	display         string
	facts           []internalingestion.Fact
	callerTransport bool
}

func (e *Engine) resolveSource(
	ctx context.Context,
	src any,
	info *StreamInfo,
	opts ConvertOptions,
) (resolvedSource, error) {
	return e.acquireSource(ctx, src, info, opts, DefaultMaxSourceBytes)
}

func (e *Engine) acquireSource(
	ctx context.Context,
	src any,
	info *StreamInfo,
	opts ConvertOptions,
	limit int64,
) (resolvedSource, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := internalingestion.Checkpoint(ctx); err != nil {
		return resolvedSource{}, sourceAcquisitionError("source-start", err)
	}
	caller := dereferenceInfo(info)

	switch value := src.(type) {
	case string:
		if looksLikeURI(value) {
			return e.acquireURI(ctx, value, caller, opts, limit)
		}
		file, err := os.Open(value)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-open", err)
		}
		defer file.Close()
		owned, err := readSourceBounded(ctx, file, limit)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-read", err)
		}
		sourceInfo := StreamInfo{
			Filename:  filepath.Base(value),
			Extension: filepath.Ext(value),
		}
		return sealResolvedSource(owned, SourceKindFile, sourceInfo, caller, sourceInfo.Filename, false), nil
	case []byte:
		owned, err := internalingestion.OwnBounded(ctx, value, limit)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-own", err)
		}
		return sealResolvedSource(owned, SourceKindBytes, StreamInfo{}, caller, "", false), nil
	case io.ReadSeeker:
		if _, err := value.Seek(0, io.SeekStart); err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-seek", err)
		}
		if err := internalingestion.Checkpoint(ctx); err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-seek", err)
		}
		owned, err := readSourceBounded(ctx, value, limit)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-read", err)
		}
		return sealResolvedSource(owned, SourceKindReader, StreamInfo{}, caller, "", false), nil
	case io.Reader:
		owned, err := readSourceBounded(ctx, value, limit)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-read", err)
		}
		return sealResolvedSource(owned, SourceKindReader, StreamInfo{}, caller, "", false), nil
	default:
		return resolvedSource{}, InvalidSourceError{Value: src}
	}
}

func (e *Engine) resolveURI(
	ctx context.Context,
	raw string,
	info *StreamInfo,
	opts ConvertOptions,
) (resolvedSource, error) {
	return e.acquireURI(ctx, raw, dereferenceInfo(info), opts, DefaultMaxSourceBytes)
}

func (e *Engine) acquireURI(
	ctx context.Context,
	raw string,
	caller StreamInfo,
	opts ConvertOptions,
	limit int64,
) (resolvedSource, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return resolvedSource{}, sourceAcquisitionError("source-uri", internalingestion.ErrIntegrityFailure)
	}

	switch strings.ToLower(parsed.Scheme) {
	case "file":
		filePath, err := fileURIToPath(parsed)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-uri", internalingestion.ErrIntegrityFailure)
		}
		file, err := os.Open(filePath)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-open", err)
		}
		defer file.Close()
		owned, err := readSourceBounded(ctx, file, limit)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-read", err)
		}
		sourceInfo := StreamInfo{Filename: filepath.Base(filePath), Extension: filepath.Ext(filePath)}
		return sealResolvedSource(owned, SourceKindFile, sourceInfo, caller, sourceInfo.Filename, false), nil
	case "data":
		mediaType, attributes, reader, err := dataURIReader(raw)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-data-uri", internalingestion.ErrIntegrityFailure)
		}
		owned, err := internalingestion.ReadBounded(ctx, reader, limit)
		if err != nil {
			return resolvedSource{}, sourceAcquisitionError("source-data-uri", err)
		}
		sourceInfo := StreamInfo{
			MIMEType: mediaType,
			Charset:  attributes["charset"],
		}
		return sealResolvedSource(owned, SourceKindDataURI, sourceInfo, caller, "", false), nil
	case "http", "https":
		remoteLimit := limit
		if optionLimit := opts.maxHTTPBytes(); optionLimit < remoteLimit {
			remoteLimit = optionLimit
		}
		client, callerTransport := callerHTTPClient(e.httpClient)
		var timeout time.Duration
		if e.httpClient != nil {
			timeout = e.httpClient.Timeout
		}
		remote, err := internalingestion.AcquireRemote(ctx, raw, internalingestion.RemoteConfig{
			Enabled:  opts.EnableHTTP,
			MaxBytes: remoteLimit,
			Client:   client,
			Timeout:  timeout,
		})
		if err != nil {
			return resolvedSource{}, publicRemoteError(err)
		}
		sourceInfo := StreamInfo{
			MIMEType:  remote.MediaType,
			Charset:   remote.Charset,
			Extension: remote.Extension,
			Filename:  remote.SafeName,
		}
		return sealResolvedSource(remote.Owned, SourceKindRemote, sourceInfo, caller, remote.Display, callerTransport || remote.CallerTransport), nil
	default:
		return resolvedSource{}, fmt.Errorf("%w: unsupported URI scheme", ErrInvalidSource)
	}
}

func looksLikeURI(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(lower, "file:") ||
		strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://")
}

func splitContentType(contentType string) (string, map[string]string) {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return "", map[string]string{}
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.ToLower(contentType), map[string]string{}
	}

	return strings.ToLower(mediaType), params
}

func fileURIToPath(u *url.URL) (string, error) {
	if u == nil {
		return "", errors.New("nil file URI")
	}
	if u.Scheme != "file" {
		return "", errors.New("expected file URI")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("unsafe file URI")
	}
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", errors.New("unsupported file host")
	}

	p, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", err
	}
	if p == "" {
		return "", errors.New("empty file URI path")
	}
	if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && isWindowsDriveLetter(p[1]) && p[2] == ':' {
		return p[1:], nil
	}

	return p, nil
}

func isWindowsDriveLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func parseDataURI(raw string) (string, map[string]string, []byte, error) {
	mediaType, attributes, reader, err := dataURIReader(raw)
	if err != nil {
		return "", nil, nil, err
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", nil, nil, errors.New("invalid data URI encoding")
	}
	return mediaType, attributes, decoded, nil
}

func dataURIReader(raw string) (string, map[string]string, io.Reader, error) {
	if !strings.HasPrefix(strings.ToLower(raw), "data:") {
		return "", nil, nil, errors.New("not a data URI")
	}
	payload := raw[len("data:"):]
	meta, data, found := strings.Cut(payload, ",")
	if !found {
		return "", nil, nil, errors.New("invalid data URI")
	}
	attributes := map[string]string{}
	mediaType := ""
	isBase64 := false
	for idx, token := range strings.Split(meta, ";") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		switch {
		case idx == 0 && strings.Contains(token, "/"):
			if canonical, err := internalingestion.CanonicalMediaType(token); err == nil {
				mediaType = canonical
			}
		case strings.EqualFold(token, "base64"):
			isBase64 = true
		case strings.Contains(token, "="):
			key, value, _ := strings.Cut(token, "=")
			key = strings.ToLower(strings.TrimSpace(key))
			value = strings.TrimSpace(value)
			if key == "charset" {
				if fact, err := internalingestion.NewFact("charset", value, internalingestion.OriginSource); err == nil {
					attributes[key] = fact.Value
				}
			}
		}
	}
	if isBase64 {
		return mediaType, attributes, base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)), nil
	}
	return mediaType, attributes, &percentDataReader{raw: data}, nil
}

type percentDataReader struct {
	raw   string
	index int
}

func (r *percentDataReader) Read(output []byte) (int, error) {
	if r.index >= len(r.raw) {
		return 0, io.EOF
	}
	written := 0
	for written < len(output) && r.index < len(r.raw) {
		value := r.raw[r.index]
		if value == '%' {
			if r.index+2 >= len(r.raw) {
				return written, errors.New("invalid data URI encoding")
			}
			high, okHigh := hexNibble(r.raw[r.index+1])
			low, okLow := hexNibble(r.raw[r.index+2])
			if !okHigh || !okLow {
				return written, errors.New("invalid data URI encoding")
			}
			value = high<<4 | low
			r.index += 3
		} else {
			r.index++
		}
		output[written] = value
		written++
	}
	return written, nil
}

func hexNibble(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func sealResolvedSource(owned internalingestion.OwnedBytes, kind SourceKind, sourceInfo, caller StreamInfo, display string, callerTransport bool) resolvedSource {
	safeInfo := safeMergedInfo(sourceInfo, caller)
	return resolvedSource{
		reader:          bytes.NewReader(owned.Bytes),
		info:            safeInfo,
		owned:           owned,
		kind:            kind,
		display:         display,
		facts:           safeStreamFacts(sourceInfo, caller, StreamInfo{}),
		callerTransport: callerTransport,
	}
}

func safeMergedInfo(source, caller StreamInfo) StreamInfo {
	var safe StreamInfo
	for _, fact := range safeStreamFacts(source, caller, StreamInfo{}) {
		switch fact.Kind {
		case "media_type":
			safe.MIMEType = fact.Value
		case "extension":
			safe.Extension = fact.Value
		case "charset":
			safe.Charset = fact.Value
		case "filename":
			safe.Filename = fact.Value
		}
	}
	return safe.normalize()
}

func safeStreamFacts(source, caller, sniff StreamInfo) []internalingestion.Fact {
	facts := make([]internalingestion.Fact, 0, 12)
	facts = appendSafeInfoFacts(facts, source, internalingestion.OriginSource)
	facts = appendSafeInfoFacts(facts, caller, internalingestion.OriginCaller)
	facts = appendSafeInfoFacts(facts, sniff, internalingestion.OriginSniff)
	return facts
}

func appendSafeInfoFacts(facts []internalingestion.Fact, info StreamInfo, origin internalingestion.FactOrigin) []internalingestion.Fact {
	mediaType, params := splitContentType(info.MIMEType)
	values := []struct {
		kind  string
		value string
	}{
		{kind: "media_type", value: mediaType},
		{kind: "charset", value: firstNonempty(info.Charset, params["charset"])},
		{kind: "filename", value: info.Filename},
		{kind: "extension", value: info.Extension},
	}
	for _, value := range values {
		if value.value == "" {
			continue
		}
		if fact, err := internalingestion.NewFact(value.kind, value.value, origin); err == nil {
			facts = append(facts, fact)
		}
	}
	return facts
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func dereferenceInfo(info *StreamInfo) StreamInfo {
	if info == nil {
		return StreamInfo{}
	}
	return *info
}

func readSourceBounded(ctx context.Context, reader io.Reader, limit int64) (internalingestion.OwnedBytes, error) {
	closer, canClose := reader.(io.Closer)
	if !canClose {
		return internalingestion.ReadBounded(ctx, reader, limit)
	}
	stop := make(chan struct{})
	joined := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(joined)
		select {
		case <-ctx.Done():
			once.Do(func() { _ = closer.Close() })
		case <-stop:
		}
	}()
	owned, err := internalingestion.ReadBounded(ctx, reader, limit)
	close(stop)
	<-joined
	if ctx.Err() != nil {
		return internalingestion.OwnedBytes{}, fmt.Errorf("%w: %w", internalingestion.ErrCancellation, ctx.Err())
	}
	return owned, err
}

func sourceAcquisitionError(operation string, err error) error {
	switch {
	case errors.Is(err, internalingestion.ErrLimitExceeded):
		return &FailureError{Category: FailureLimit, Operation: operation, Cause: err}
	case errors.Is(err, internalingestion.ErrCancellation):
		return &FailureError{Category: FailureCancellation, Operation: operation, Cause: err}
	case errors.Is(err, internalingestion.ErrPolicyViolation):
		return &FailureError{Category: FailurePolicy, Operation: operation, Cause: err}
	default:
		return &FailureError{Category: FailureIntegrity, Operation: operation, Cause: err}
	}
}

func publicRemoteError(err error) error {
	switch {
	case errors.Is(err, internalingestion.ErrRemoteDisabled):
		return ErrRemoteDisabled
	case errors.Is(err, internalingestion.ErrRemoteTooLarge):
		return ErrRemoteTooLarge
	case errors.Is(err, internalingestion.ErrRemoteDenied):
		return &FailureError{Category: FailurePolicy, Operation: "remote-admission", Cause: err}
	case errors.Is(err, internalingestion.ErrCancellation):
		return &FailureError{Category: FailureCancellation, Operation: "remote-read", Cause: err}
	default:
		return &FailureError{Category: FailureIntegrity, Operation: "remote-read", Cause: err}
	}
}
