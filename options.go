package inkbite

import "net/http"

const DefaultMaxHTTPBytes int64 = 32 << 20

// ConvertOptions controls per-conversion behavior.
type ConvertOptions struct {
	KeepDataURIs bool
	EnableHTTP   bool
	MaxHTTPBytes int64
	PDFBackend   string
}

// Option customizes engine-wide behavior.
type Option func(*Engine)

// WithHTTPClient installs a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(e *Engine) {
		if client != nil {
			e.httpClient = client
		}
	}
}

func (o ConvertOptions) maxHTTPBytes() int64 {
	if o.MaxHTTPBytes > 0 {
		return o.MaxHTTPBytes
	}
	return DefaultMaxHTTPBytes
}
