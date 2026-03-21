package inkbite

import (
	"mime"
	"path/filepath"
	"strings"
)

// StreamInfo contains hints or inferred metadata about a source stream.
type StreamInfo struct {
	MIMEType  string
	Extension string
	Charset   string
	Filename  string
	LocalPath string
	URL       string
}

// Merge overlays the non-empty fields from override onto the receiver.
func (s StreamInfo) Merge(override StreamInfo) StreamInfo {
	if override.MIMEType != "" {
		s.MIMEType = override.MIMEType
	}
	if override.Extension != "" {
		s.Extension = override.Extension
	}
	if override.Charset != "" {
		s.Charset = override.Charset
	}
	if override.Filename != "" {
		s.Filename = override.Filename
	}
	if override.LocalPath != "" {
		s.LocalPath = override.LocalPath
	}
	if override.URL != "" {
		s.URL = override.URL
	}

	return s.normalize()
}

func (s StreamInfo) normalize() StreamInfo {
	if s.Extension != "" {
		s.Extension = strings.ToLower(s.Extension)
		if !strings.HasPrefix(s.Extension, ".") {
			s.Extension = "." + s.Extension
		}
	}

	if s.LocalPath != "" && s.Filename == "" {
		s.Filename = filepath.Base(s.LocalPath)
	}

	if s.Filename != "" && s.Extension == "" {
		s.Extension = strings.ToLower(filepath.Ext(s.Filename))
	}

	if s.MIMEType != "" {
		mediaType, params, err := mime.ParseMediaType(s.MIMEType)
		if err == nil {
			s.MIMEType = strings.ToLower(mediaType)
			if s.Charset == "" && params["charset"] != "" {
				s.Charset = strings.ToLower(params["charset"])
			}
		} else {
			s.MIMEType = strings.ToLower(strings.TrimSpace(s.MIMEType))
		}
	}

	if s.Charset != "" {
		s.Charset = strings.ToLower(strings.TrimSpace(s.Charset))
	}

	return s
}
