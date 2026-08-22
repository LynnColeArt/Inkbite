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
			clone := *client
			transport := clone.Transport
			if transport == nil {
				transport = http.DefaultTransport
			}
			clone.Transport = callerRoundTripper{base: transport}
			e.httpClient = &clone
		}
	}
}

// callerRoundTripper marks a caller-supplied HTTP client as an explicit trusted
// transport capability. The marker is request-local engine configuration; it
// does not grant the default client ambient network authority.
type callerRoundTripper struct {
	base http.RoundTripper
}

func (t callerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return t.base.RoundTrip(request)
}

func callerHTTPClient(client *http.Client) (*http.Client, bool) {
	if client == nil {
		return nil, false
	}
	if _, ok := client.Transport.(callerRoundTripper); !ok {
		return nil, false
	}
	return client, true
}

func (o ConvertOptions) maxHTTPBytes() int64 {
	if o.MaxHTTPBytes > 0 {
		return o.MaxHTTPBytes
	}
	return DefaultMaxHTTPBytes
}
