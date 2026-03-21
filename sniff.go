package inkbite

import (
	"io"
	"mime"
	"net/http"
	"strings"
)

func enrichStreamInfo(r io.ReadSeeker, info StreamInfo) (StreamInfo, error) {
	info = info.Merge(StreamInfo{})

	if info.MIMEType == "" && info.Extension != "" {
		if guessed := mime.TypeByExtension(info.Extension); guessed != "" {
			mediaType, params := splitMediaType(guessed)
			info = info.Merge(StreamInfo{
				MIMEType: mediaType,
				Charset:  params["charset"],
			})
		}
	}

	cur, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return info, err
	}
	defer func() {
		_, _ = r.Seek(cur, io.SeekStart)
	}()

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return info, err
	}

	buf := make([]byte, 512)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return info, err
	}

	if n > 0 {
		mediaType, params := splitMediaType(http.DetectContentType(buf[:n]))
		if info.MIMEType == "" && mediaType != "" && mediaType != "application/octet-stream" {
			info = info.Merge(StreamInfo{
				MIMEType: mediaType,
				Charset:  params["charset"],
			})
		}
	}

	if info.Extension == "" && info.MIMEType != "" {
		if exts, err := mime.ExtensionsByType(info.MIMEType); err == nil && len(exts) > 0 {
			info = info.Merge(StreamInfo{Extension: strings.ToLower(exts[0])})
		}
	}

	return info, nil
}

func splitMediaType(value string) (string, map[string]string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", map[string]string{}
	}

	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil {
		return strings.ToLower(value), map[string]string{}
	}

	return strings.ToLower(mediaType), params
}
