package inkbite

import "net/http"

// ConvertOptions controls per-conversion behavior.
type ConvertOptions struct {
	KeepDataURIs bool
	EnableHTTP   bool
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
