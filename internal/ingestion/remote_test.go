package ingestion

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRemoteDisabledHasZeroAuthorityCalls(t *testing.T) {
	t.Parallel()

	var resolves, dials, transports atomic.Int64
	config := RemoteConfig{
		Enabled:  false,
		MaxBytes: 64,
		Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
			resolves.Add(1)
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		}),
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dials.Add(1)
			return nil, errors.New("unexpected dial")
		},
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			transports.Add(1)
			return nil, errors.New("unexpected transport")
		})},
	}

	for _, raw := range []string{
		"https://example.test/looks-valid",
		"http://[::1",
		"https://user:secret@example.test/path",
		"https://example.test/%zz",
	} {
		if _, err := AcquireRemote(context.Background(), raw, config); !errors.Is(err, ErrRemoteDisabled) {
			t.Fatalf("AcquireRemote(%q) error = %v, want disabled", raw, err)
		}
	}
	if resolves.Load() != 0 || dials.Load() != 0 || transports.Load() != 0 {
		t.Fatalf("disabled authority calls: resolve=%d dial=%d transport=%d", resolves.Load(), dials.Load(), transports.Load())
	}
}

func TestRemoteAddressAdmissionMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		address string
		allow   bool
	}{
		{address: "8.8.8.8", allow: true},
		{address: "2606:4700:4700::1111", allow: true},
		{address: "0.0.0.0"},
		{address: "10.0.0.1"},
		{address: "100.64.0.1"},
		{address: "127.0.0.1"},
		{address: "169.254.1.1"},
		{address: "172.16.0.1"},
		{address: "192.0.2.1"},
		{address: "192.168.0.1"},
		{address: "198.18.0.1"},
		{address: "198.51.100.1"},
		{address: "203.0.113.1"},
		{address: "224.0.0.1"},
		{address: "240.0.0.1"},
		{address: "::"},
		{address: "::1"},
		{address: "::ffff:127.0.0.1"},
		{address: "64:ff9b:1::1"},
		{address: "100::1"},
		{address: "2001:db8::1"},
		{address: "2001:2::1"},
		{address: "2002:7f00:1::"},
		{address: "fc00::1"},
		{address: "fe80::1"},
		{address: "ff00::1"},
	}
	for _, tc := range tests {
		address := netip.MustParseAddr(tc.address)
		if got := IsAllowedRemoteAddress(address); got != tc.allow {
			t.Errorf("IsAllowedRemoteAddress(%s) = %v, want %v", address, got, tc.allow)
		}
	}
}

func TestRemoteAddressPolicyMirrorsIANA20251009(t *testing.T) {
	t.Parallel()

	if specialPurposeRegistryRevision != "2025-10-09" {
		t.Fatalf("special-purpose registry revision = %q, update the audited table and tests together", specialPurposeRegistryRevision)
	}
	tests := []struct {
		name    string
		address string
		allow   bool
	}{
		// IANA IPv4 Special-Purpose Address Space, revision 2025-10-09.
		{name: "v4 this network", address: "0.1.2.3"},
		{name: "v4 private 10", address: "10.0.0.1"},
		{name: "v4 shared", address: "100.64.0.1"},
		{name: "v4 loopback", address: "127.0.0.1"},
		{name: "v4 link local", address: "169.254.1.1"},
		{name: "v4 private 172", address: "172.16.0.1"},
		{name: "v4 protocol umbrella", address: "192.0.0.11"},
		{name: "v4 service continuity", address: "192.0.0.1"},
		{name: "v4 dummy", address: "192.0.0.8"},
		{name: "v4 PCP exception", address: "192.0.0.9", allow: true},
		{name: "v4 TURN exception", address: "192.0.0.10", allow: true},
		{name: "v4 NAT64 discovery", address: "192.0.0.170"},
		{name: "v4 documentation one", address: "192.0.2.1"},
		{name: "v4 AS112", address: "192.31.196.1", allow: true},
		{name: "v4 AMT", address: "192.52.193.1", allow: true},
		{name: "v4 deprecated 6to4", address: "192.88.99.1"},
		{name: "v4 6a44", address: "192.88.99.2"},
		{name: "v4 private 192", address: "192.168.0.1"},
		{name: "v4 direct AS112", address: "192.175.48.1", allow: true},
		{name: "v4 benchmarking", address: "198.18.0.1"},
		{name: "v4 documentation two", address: "198.51.100.1"},
		{name: "v4 documentation three", address: "203.0.113.1"},
		{name: "v4 reserved", address: "240.0.0.1"},
		{name: "v4 broadcast", address: "255.255.255.255"},

		// IANA IPv6 Special-Purpose Address Space, revision 2025-10-09.
		{name: "v6 unspecified", address: "::"},
		{name: "v6 loopback", address: "::1"},
		{name: "v6 mapped private", address: "::ffff:127.0.0.1"},
		{name: "v6 translation globally reachable", address: "64:ff9b::808:808", allow: true},
		{name: "v6 local translation", address: "64:ff9b:1::1"},
		{name: "v6 discard", address: "100::1"},
		{name: "v6 dummy", address: "100:0:0:1::1"},
		{name: "v6 teredo non-boolean reachability", address: "2001::1"},
		{name: "v6 PCP exception", address: "2001:1::1", allow: true},
		{name: "v6 TURN exception", address: "2001:1::2", allow: true},
		{name: "v6 DNS-SD exception", address: "2001:1::3", allow: true},
		{name: "v6 unallocated beside exact exceptions", address: "2001:1::4"},
		{name: "v6 benchmarking", address: "2001:2::1"},
		{name: "v6 AMT exception", address: "2001:3::1", allow: true},
		{name: "v6 AS112 exception", address: "2001:4:112::1", allow: true},
		{name: "v6 unallocated protocol assignment", address: "2001:5::1"},
		{name: "v6 deprecated ORCHID", address: "2001:10::1"},
		{name: "v6 ORCHIDv2 exception", address: "2001:20::1", allow: true},
		{name: "v6 drone RID exception", address: "2001:30::1", allow: true},
		{name: "v6 documentation", address: "2001:db8::1"},
		{name: "v6 6to4 non-boolean reachability", address: "2002::1"},
		{name: "v6 direct AS112", address: "2620:4f:8000::1", allow: true},
		{name: "v6 documentation two", address: "3fff::1"},
		{name: "v6 SRv6", address: "5f00::1"},
		{name: "v6 unique local", address: "fc00::1"},
		{name: "v6 link local", address: "fe80::1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			address := netip.MustParseAddr(tc.address)
			if got := IsAllowedRemoteAddress(address); got != tc.allow {
				t.Fatalf("IsAllowedRemoteAddress(%s) = %v, want %v", address, got, tc.allow)
			}
		})
	}
}

func TestRemoteURLShapeDeniedBeforeCallerTransport(t *testing.T) {
	t.Parallel()

	secret := "SENSITIVE-REMOTE-URL"
	var calls atomic.Int64
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("unexpected transport")
	})}
	for _, raw := range []string{
		"ftp://example.test/" + secret,
		"https://user:" + secret + "@example.test/file",
		"https://example.test:65536/" + secret,
		"https://éxample.test/" + secret,
		"http://[::ffff:127.0.0.1]/" + secret,
	} {
		_, err := AcquireRemote(context.Background(), raw, RemoteConfig{Enabled: true, MaxBytes: 64, Client: client})
		if !errors.Is(err, ErrRemoteDenied) || strings.Contains(err.Error(), secret) {
			t.Fatalf("AcquireRemote(%q) error = %v", raw, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("denied URLs reached caller transport %d times", calls.Load())
	}
}

func TestRemoteRejectsMixedDNSAnswersBeforeDial(t *testing.T) {
	t.Parallel()

	var dials atomic.Int64
	_, err := AcquireRemote(context.Background(), "http://mixed.test/file", RemoteConfig{
		Enabled:  true,
		MaxBytes: 64,
		Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{
				netip.MustParseAddr("93.184.216.34"),
				netip.MustParseAddr("127.0.0.1"),
			}, nil
		}),
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dials.Add(1)
			return nil, errors.New("unexpected dial")
		},
	})
	if !errors.Is(err, ErrRemoteDenied) {
		t.Fatalf("AcquireRemote() error = %v, want denied", err)
	}
	if dials.Load() != 0 {
		t.Fatalf("dial calls = %d, want zero", dials.Load())
	}
}

func TestPinnedTransportPreservesTLSAuthorityAndDisablesAmbientFeatures(t *testing.T) {
	t.Parallel()

	var dialAddress string
	transport := NewPinnedTransport("Example.TEST", netip.MustParseAddr("93.184.216.34"), func(_ context.Context, _, address string) (net.Conn, error) {
		dialAddress = address
		return nil, errors.New("stop")
	})
	if transport.Proxy != nil {
		t.Fatal("pinned transport inherited ambient proxy authority")
	}
	if !transport.DisableCompression {
		t.Fatal("pinned transport permits transparent decompression")
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.ServerName != "example.test" {
		t.Fatalf("TLS ServerName = %#v, want example.test", transport.TLSClientConfig)
	}
	_, _ = transport.DialContext(context.Background(), "tcp", "ignored.test:8443")
	if dialAddress != "93.184.216.34:8443" {
		t.Fatalf("dial target = %q, want admitted IP", dialAddress)
	}
}

func TestRemoteOwnedTransportIgnoresProxyEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:65535")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:65535")

	transport := NewPinnedTransport("example.test", netip.MustParseAddr("93.184.216.34"), func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("stop")
	})
	if transport.Proxy != nil {
		t.Fatal("owned transport consulted ambient proxy configuration")
	}
}

func TestRemoteRedirectReadmissionPinnedDialAndExactRepresentation(t *testing.T) {
	var mu sync.Mutex
	var resolutions []string
	var dials []string
	var requests []*http.Request

	resolver := resolverFunc(func(_ context.Context, _, host string) ([]netip.Addr, error) {
		mu.Lock()
		resolutions = append(resolutions, host)
		mu.Unlock()
		switch host {
		case "alpha.test":
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		case "beta.test":
			return []netip.Addr{netip.MustParseAddr("93.184.216.35")}, nil
		default:
			return nil, errors.New("unknown host")
		}
	})
	dialer := func(_ context.Context, _, address string) (net.Conn, error) {
		client, server := net.Pipe()
		mu.Lock()
		dials = append(dials, address)
		mu.Unlock()
		go func() {
			defer server.Close()
			req, err := http.ReadRequest(bufio.NewReader(server))
			if err != nil {
				return
			}
			mu.Lock()
			requests = append(requests, req)
			mu.Unlock()
			if req.Host == "alpha.test" {
				_, _ = io.WriteString(server, "HTTP/1.1 302 Found\r\nLocation: http://beta.test/final?token=SENSITIVE\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
				return
			}
			_, _ = io.WriteString(server, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Encoding: gzip\r\nContent-Length: 4\r\nConnection: close\r\n\r\n\x1f\x8b\x00\x00")
		}()
		return client, nil
	}

	got, err := AcquireRemote(context.Background(), "http://alpha.test/start", RemoteConfig{
		Enabled:     true,
		MaxBytes:    4,
		Resolver:    resolver,
		DialContext: dialer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Owned.Bytes) != "\x1f\x8b\x00\x00" || got.Owned.Identity != Identity(got.Owned.Bytes) {
		t.Fatalf("retained bytes = %x, want exact encoded representation", got.Owned.Bytes)
	}
	if got.Display != "http://beta.test" || strings.Contains(got.Display, "SENSITIVE") {
		t.Fatalf("safe display = %q", got.Display)
	}
	if got.CallerTransport {
		t.Fatal("owned transport reported caller authority")
	}
	if fmt.Sprint(resolutions) != "[alpha.test beta.test]" {
		t.Fatalf("resolutions = %v", resolutions)
	}
	if fmt.Sprint(dials) != "[93.184.216.34:80 93.184.216.35:80]" {
		t.Fatalf("pinned dials = %v", dials)
	}
	for _, req := range requests {
		if req.Header.Get("Accept-Encoding") != "identity" {
			t.Fatalf("Accept-Encoding = %q, want identity", req.Header.Get("Accept-Encoding"))
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("Proxy-Authorization") != "" {
			t.Fatal("redirect retained sensitive headers")
		}
	}
}

func TestRemoteRebindingAndRedirectCapFailClosed(t *testing.T) {
	t.Parallel()

	var lookups atomic.Int64
	var dials atomic.Int64
	resolver := resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		if lookups.Add(1) == 1 {
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})
	dialer := func(context.Context, string, string) (net.Conn, error) {
		dials.Add(1)
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			_, _ = http.ReadRequest(bufio.NewReader(server))
			_, _ = io.WriteString(server, "HTTP/1.1 302 Found\r\nLocation: http://rebind.test/again\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		}()
		return client, nil
	}
	_, err := AcquireRemote(context.Background(), "http://rebind.test/start", RemoteConfig{
		Enabled: true, MaxBytes: 8, Resolver: resolver, DialContext: dialer,
	})
	if !errors.Is(err, ErrRemoteDenied) || dials.Load() != 1 {
		t.Fatalf("rebind error=%v dials=%d, want denied after one pinned dial", err, dials.Load())
	}

	redirecting := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"http://example.test/again"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	})}
	_, err = AcquireRemote(context.Background(), "http://example.test/start", RemoteConfig{
		Enabled: true, MaxBytes: 8, MaxRedirects: 2, Client: redirecting,
	})
	if !errors.Is(err, ErrRemoteDenied) {
		t.Fatalf("redirect-cap error = %v, want denied", err)
	}
}

func TestCallerTransportProvenanceRedirectAdmissionAndRedaction(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"http://127.0.0.1/private?token=SENSITIVE"}},
			Body:       io.NopCloser(strings.NewReader("backend SENSITIVE")),
			Request:    req,
		}, nil
	})}
	_, err := AcquireRemote(context.Background(), "https://example.test/start?token=SENSITIVE", RemoteConfig{
		Enabled: true, MaxBytes: 64, Client: client,
	})
	if !errors.Is(err, ErrRemoteDenied) || strings.Contains(err.Error(), "SENSITIVE") {
		t.Fatalf("redirect error = %q, want sanitized denial", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("caller transport calls = %d, want one", calls.Load())
	}

	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Body:          io.NopCloser(strings.NewReader("caller bytes")),
			ContentLength: int64(len("caller bytes")),
			Request:       req,
		}, nil
	})
	got, err := AcquireRemote(context.Background(), "https://example.test/safe.txt", RemoteConfig{
		Enabled: true, MaxBytes: 64, Client: client,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.CallerTransport || got.SafeName != "safe.txt" || got.MediaType != "text/plain" || got.Charset != "utf-8" {
		t.Fatalf("caller result = %#v", got)
	}

	secret := "URL-CREDENTIAL-BODY-BACKEND-SENSITIVE"
	client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New(secret)
	})
	_, err = AcquireRemote(context.Background(), "https://example.test/?token="+secret, RemoteConfig{
		Enabled: true, MaxBytes: 64, Client: client,
	})
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("transport diagnostic leaked: %v", err)
	}
}

func TestRemoteSensitiveHeaderStripping(t *testing.T) {
	t.Parallel()

	header := http.Header{
		"Authorization":       []string{"Bearer SENSITIVE"},
		"Proxy-Authorization": []string{"Basic SENSITIVE"},
		"Cookie":              []string{"token=SENSITIVE"},
		"Accept":              []string{"text/plain"},
	}
	stripSensitiveHeaders(header)
	if header.Get("Authorization") != "" || header.Get("Proxy-Authorization") != "" || header.Get("Cookie") != "" {
		t.Fatalf("sensitive headers retained: %#v", header)
	}
	if header.Get("Accept") != "text/plain" {
		t.Fatalf("non-sensitive header removed: %#v", header)
	}
}

func TestRemoteBodyLimitUsesHintAndLimitPlusOne(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          string
		contentLength int64
		wantErr       error
	}{
		{name: "at limit", body: "12345678", contentLength: 8},
		{name: "declared over", body: "", contentLength: 9, wantErr: ErrRemoteTooLarge},
		{name: "streamed over", body: "123456789", contentLength: -1, wantErr: ErrRemoteTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body)), ContentLength: tc.contentLength, Request: req}, nil
			})}
			got, err := AcquireRemote(context.Background(), "http://example.test/file", RemoteConfig{Enabled: true, MaxBytes: 8, Client: client})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("AcquireRemote() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && got.Owned.Bytes != nil {
				t.Fatalf("failed acquisition returned bytes: %#v", got)
			}
		})
	}
}

type blockingBody struct {
	entered     chan struct{}
	closed      chan struct{}
	readExited  chan struct{}
	closeExited chan struct{}
	enterOnce   sync.Once
	closeOnce   sync.Once
}

func (b *blockingBody) Read([]byte) (int, error) {
	b.enterOnce.Do(func() { close(b.entered) })
	<-b.closed
	close(b.readExited)
	return 0, errors.New("SENSITIVE blocked body")
}

func (b *blockingBody) Close() error {
	b.closeOnce.Do(func() {
		close(b.closed)
		close(b.closeExited)
	})
	return nil
}

func TestRemoteCancellationClosesAndJoinsBody(t *testing.T) {
	t.Parallel()

	body := &blockingBody{
		entered:     make(chan struct{}),
		closed:      make(chan struct{}),
		readExited:  make(chan struct{}),
		closeExited: make(chan struct{}),
	}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: body, ContentLength: -1, Request: req}, nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := AcquireRemote(ctx, "https://example.test/body", RemoteConfig{Enabled: true, MaxBytes: 64, Client: client})
		done <- err
	}()
	<-body.entered
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, ErrCancellation) || !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "SENSITIVE") {
			t.Fatalf("cancellation error = %v", err)
		}
		select {
		case <-body.readExited:
		default:
			t.Fatal("remote body Read was not joined")
		}
		select {
		case <-body.closeExited:
		default:
			t.Fatal("remote body Close was not joined")
		}
	case <-time.After(time.Second):
		t.Fatal("remote cancellation did not close and join the blocking body")
	}
}

func TestRemoteConcurrentRequestIsolation(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		payload := req.URL.Query().Get("payload")
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Content-Type": []string{"text/plain"}},
			Body:          io.NopCloser(strings.NewReader(payload)),
			ContentLength: int64(len(payload)),
			Request:       req,
		}, nil
	})}

	var wait sync.WaitGroup
	errorsSeen := make(chan error, 100)
	for index := 0; index < 100; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			payload := fmt.Sprintf("request-%03d", index)
			got, err := AcquireRemote(context.Background(), "https://example.test/file?payload="+payload, RemoteConfig{
				Enabled: true, MaxBytes: 64, Client: client,
			})
			if err != nil {
				errorsSeen <- err
				return
			}
			if string(got.Owned.Bytes) != payload || got.Owned.Identity != Identity([]byte(payload)) || !got.CallerTransport {
				errorsSeen <- fmt.Errorf("request %d crossed state boundary: %#v", index, got)
			}
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Error(err)
	}
}
