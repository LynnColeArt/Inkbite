package inkbite

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type resolvedSource struct {
	reader *bytes.Reader
	info   StreamInfo
}

func (e *Engine) resolveSource(
	ctx context.Context,
	src any,
	info *StreamInfo,
	opts ConvertOptions,
) (resolvedSource, error) {
	var base StreamInfo
	var data []byte
	var err error

	switch value := src.(type) {
	case string:
		if looksLikeURI(value) {
			return e.resolveURI(ctx, value, info, opts)
		}

		data, err = os.ReadFile(value)
		if err != nil {
			return resolvedSource{}, err
		}
		base = StreamInfo{
			LocalPath: value,
			Filename:  filepath.Base(value),
			Extension: filepath.Ext(value),
		}
	case []byte:
		data = value
	case io.ReadSeeker:
		if _, err = value.Seek(0, io.SeekStart); err != nil {
			return resolvedSource{}, err
		}
		data, err = io.ReadAll(value)
		if err != nil {
			return resolvedSource{}, err
		}
	case io.Reader:
		data, err = io.ReadAll(value)
		if err != nil {
			return resolvedSource{}, err
		}
	default:
		return resolvedSource{}, InvalidSourceError{Value: src}
	}

	if info != nil {
		base = base.Merge(*info)
	}

	return resolvedSource{
		reader: bytes.NewReader(data),
		info:   base.normalize(),
	}, nil
}

func (e *Engine) resolveURI(
	ctx context.Context,
	raw string,
	info *StreamInfo,
	opts ConvertOptions,
) (resolvedSource, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return resolvedSource{}, err
	}

	switch parsed.Scheme {
	case "file":
		filePath, err := fileURIToPath(parsed)
		if err != nil {
			return resolvedSource{}, err
		}

		source, err := e.resolveSource(ctx, filePath, info, opts)
		if err != nil {
			return resolvedSource{}, err
		}
		source.info.URL = raw
		return source, nil
	case "data":
		mediaType, attributes, data, err := parseDataURI(raw)
		if err != nil {
			return resolvedSource{}, err
		}

		base := StreamInfo{
			MIMEType: mediaType,
			Charset:  attributes["charset"],
			URL:      raw,
		}
		if info != nil {
			base = base.Merge(*info)
		}

		return resolvedSource{
			reader: bytes.NewReader(data),
			info:   base.normalize(),
		}, nil
	case "http", "https":
		if !opts.EnableHTTP {
			return resolvedSource{}, ErrRemoteDisabled
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return resolvedSource{}, err
		}
		req.Header.Set("Accept", "text/markdown, text/html;q=0.9, text/plain;q=0.8, */*;q=0.1")

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return resolvedSource{}, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			return resolvedSource{}, fmt.Errorf("http %d for %s", resp.StatusCode, raw)
		}

		limit := opts.maxHTTPBytes()
		if resp.ContentLength > limit {
			return resolvedSource{}, fmt.Errorf("%w: %s exceeds %d bytes", ErrRemoteTooLarge, raw, limit)
		}

		data, err := readAllWithLimit(resp.Body, limit)
		if err != nil {
			return resolvedSource{}, err
		}

		mediaType, params := splitContentType(resp.Header.Get("Content-Type"))
		filename := path.Base(parsed.Path)
		if filename == "." || filename == "/" {
			filename = ""
		}

		base := StreamInfo{
			MIMEType:  mediaType,
			Charset:   params["charset"],
			Extension: strings.ToLower(filepath.Ext(parsed.Path)),
			Filename:  filename,
			URL:       raw,
		}
		if info != nil {
			base = base.Merge(*info)
		}

		return resolvedSource{
			reader: bytes.NewReader(data),
			info:   base.normalize(),
		}, nil
	default:
		return resolvedSource{}, fmt.Errorf("%w: unsupported URI scheme %q", ErrInvalidSource, parsed.Scheme)
	}
}

func readAllWithLimit(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return io.ReadAll(r)
	}

	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: exceeds %d bytes", ErrRemoteTooLarge, limit)
	}
	return data, nil
}

func looksLikeURI(raw string) bool {
	return strings.HasPrefix(raw, "file:") ||
		strings.HasPrefix(raw, "data:") ||
		strings.HasPrefix(raw, "http://") ||
		strings.HasPrefix(raw, "https://")
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
		return "", fmt.Errorf("expected file URI, got %q", u.Scheme)
	}
	if u.Host != "" && u.Host != "localhost" {
		return "", fmt.Errorf("unsupported file host %q", u.Host)
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
	if !strings.HasPrefix(raw, "data:") {
		return "", nil, nil, errors.New("not a data URI")
	}

	payload := strings.TrimPrefix(raw, "data:")
	meta, data, found := strings.Cut(payload, ",")
	if !found {
		return "", nil, nil, errors.New("invalid data URI")
	}

	attributes := map[string]string{}
	var mediaType string
	isBase64 := false

	if meta != "" {
		for idx, token := range strings.Split(meta, ";") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}

			switch {
			case idx == 0 && strings.Contains(token, "/"):
				mediaType = strings.ToLower(token)
			case strings.EqualFold(token, "base64"):
				isBase64 = true
			case strings.Contains(token, "="):
				key, value, _ := strings.Cut(token, "=")
				attributes[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
			}
		}
	}

	var decoded []byte
	var err error
	if isBase64 {
		decoded, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", nil, nil, err
		}
	} else {
		unescaped, err := url.PathUnescape(data)
		if err != nil {
			return "", nil, nil, err
		}
		decoded = []byte(unescaped)
	}

	return mediaType, attributes, decoded, nil
}
