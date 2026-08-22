package ingestion

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultMaxRedirects = 10

// specialPurposeRegistryRevision pins the IANA registry snapshot audited by
// specialPurposeAddressRules. Update the rules, revision, and mirror test in
// one change whenever either registry changes.
const specialPurposeRegistryRevision = "2025-10-09"

var (
	// ErrRemoteDisabled proves the request had no remote authority.
	ErrRemoteDisabled = errors.New("remote fetching is disabled")
	// ErrRemoteDenied classifies a malformed URL, denied destination, redirect,
	// or response status without retaining its authority-bearing detail.
	ErrRemoteDenied = errors.New("remote destination denied")
	// ErrRemoteTooLarge classifies an over-limit remote representation.
	ErrRemoteTooLarge = errors.New("remote response exceeds size limit")
	// ErrRemoteFailure classifies resolver, dial, TLS, transport, and body
	// failures without exposing backend diagnostic text.
	ErrRemoteFailure = errors.New("remote acquisition failed")
)

// RemoteResolver is the only name-resolution authority used by the owned
// transport. Tests and hosts cannot accidentally fall back to a second lookup.
type RemoteResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

// RemoteConfig grants and bounds one remote acquisition.
type RemoteConfig struct {
	Enabled      bool
	MaxBytes     int64
	MaxRedirects int
	Resolver     RemoteResolver
	DialContext  func(context.Context, string, string) (net.Conn, error)
	Client       *http.Client
	Timeout      time.Duration
}

// RemoteResult owns the exact received representation and retains only safe
// scalar source facts. CallerTransport distinguishes a trusted injected client
// from the proxy-free, admission-pinned engine transport.
type RemoteResult struct {
	Owned           OwnedBytes
	MediaType       string
	Charset         string
	SafeName        string
	Extension       string
	Display         string
	CallerTransport bool
}

// AcquireRemote fetches one explicitly authorized HTTP(S) source. Redirects
// are manual so every destination is re-admitted before another transport call.
func AcquireRemote(ctx context.Context, raw string, config RemoteConfig) (RemoteResult, error) {
	if !config.Enabled {
		return RemoteResult{}, newClassifiedError(ErrRemoteDisabled, nil)
	}
	if config.MaxBytes < 0 {
		return RemoteResult{}, newClassifiedError(ErrPolicyViolation, nil)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	current, err := parseRemoteURL(raw)
	if err != nil {
		return RemoteResult{}, err
	}
	maxRedirects := config.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = defaultMaxRedirects
	}

	for redirects := 0; ; redirects++ {
		admitted, err := admitRemoteURL(ctx, current, config)
		if err != nil {
			return RemoteResult{}, err
		}
		response, ownedTransport, err := remoteRoundTrip(ctx, current, admitted, config)
		if err != nil {
			return RemoteResult{}, err
		}

		if isRedirectStatus(response.StatusCode) {
			location := response.Header.Get("Location")
			_ = response.Body.Close()
			if redirects >= maxRedirects || location == "" {
				return RemoteResult{}, newClassifiedError(ErrRemoteDenied, nil)
			}
			next, err := current.Parse(location)
			if err != nil {
				return RemoteResult{}, newClassifiedError(ErrRemoteDenied, nil)
			}
			current, err = parseRemoteURL(next.String())
			if err != nil {
				return RemoteResult{}, err
			}
			continue
		}

		result, err := consumeRemoteResponse(ctx, current, response, config.MaxBytes)
		if err != nil {
			return RemoteResult{}, err
		}
		result.CallerTransport = !ownedTransport
		return result, nil
	}
}

type remoteAdmission struct {
	address netip.Addr
	host    string
}

func admitRemoteURL(ctx context.Context, parsed *url.URL, config RemoteConfig) (remoteAdmission, error) {
	host := strings.ToLower(parsed.Hostname())
	if host == "" || strings.ContainsAny(host, "\x00\r\n%") {
		return remoteAdmission{}, newClassifiedError(ErrRemoteDenied, nil)
	}
	if !safeHostname(host) {
		return remoteAdmission{}, newClassifiedError(ErrRemoteDenied, nil)
	}
	if literal, err := netip.ParseAddr(host); err == nil {
		literal = literal.Unmap()
		if !IsAllowedRemoteAddress(literal) {
			return remoteAdmission{}, newClassifiedError(ErrRemoteDenied, nil)
		}
		return remoteAdmission{address: literal, host: host}, nil
	}
	if config.Client != nil {
		// A supplied client is an explicit caller transport capability. URL shape,
		// literal destinations, redirects, limits, and diagnostics remain owned
		// here; the caller owns hostname resolution and connection semantics.
		return remoteAdmission{host: host}, nil
	}

	resolver := config.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addresses, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return remoteAdmission{}, classifyRemoteFailure(ctx)
	}
	var selected netip.Addr
	seen := make(map[netip.Addr]struct{}, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		if !IsAllowedRemoteAddress(address) {
			return remoteAdmission{}, newClassifiedError(ErrRemoteDenied, nil)
		}
		if _, duplicate := seen[address]; duplicate {
			continue
		}
		seen[address] = struct{}{}
		if !selected.IsValid() {
			selected = address
		}
	}
	if !selected.IsValid() {
		return remoteAdmission{}, newClassifiedError(ErrRemoteDenied, nil)
	}
	return remoteAdmission{address: selected, host: host}, nil
}

func remoteRoundTrip(ctx context.Context, parsed *url.URL, admission remoteAdmission, config RemoteConfig) (*http.Response, bool, error) {
	var client *http.Client
	owned := config.Client == nil
	if owned {
		dial := config.DialContext
		if dial == nil {
			dialer := &net.Dialer{}
			dial = dialer.DialContext
		}
		transport := NewPinnedTransport(admission.host, admission.address, dial)
		defer transport.CloseIdleConnections()
		client = &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	} else {
		clone := *config.Client
		clone.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		client = &clone
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, owned, newClassifiedError(ErrRemoteDenied, nil)
	}
	request.Header.Set("Accept", "text/markdown, text/html;q=0.9, text/plain;q=0.8, */*;q=0.1")
	request.Header.Set("Accept-Encoding", "identity")
	stripSensitiveHeaders(request.Header)
	response, err := client.Do(request)
	if err != nil {
		return nil, owned, classifyRemoteFailure(ctx)
	}
	if response == nil || response.Body == nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, owned, newClassifiedError(ErrRemoteFailure, nil)
	}
	return response, owned, nil
}

func consumeRemoteResponse(ctx context.Context, parsed *url.URL, response *http.Response, limit int64) (RemoteResult, error) {
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_ = response.Body.Close()
		return RemoteResult{}, newClassifiedError(ErrRemoteDenied, nil)
	}
	if response.ContentLength > limit {
		_ = response.Body.Close()
		return RemoteResult{}, newClassifiedError(ErrRemoteTooLarge, nil)
	}
	owned, err := readRemoteBody(ctx, response.Body, limit)
	if err != nil {
		if errors.Is(err, ErrLimitExceeded) {
			return RemoteResult{}, newClassifiedError(ErrRemoteTooLarge, nil)
		}
		return RemoteResult{}, err
	}

	mediaType, params := splitRemoteContentType(response.Header.Get("Content-Type"))
	safeName := path.Base(parsed.Path)
	if safeName == "." || safeName == "/" {
		safeName = ""
	}
	if canonical, err := CanonicalLogicalName(safeName); err == nil {
		safeName = canonical
	} else {
		safeName = ""
	}
	extension := ""
	if safeName != "" {
		if canonical, err := CanonicalExtension(filepath.Ext(safeName)); err == nil {
			extension = canonical
		}
	}
	display, err := CanonicalURLDisplay(parsed.String())
	if err != nil {
		return RemoteResult{}, newClassifiedError(ErrRemoteDenied, nil)
	}
	return RemoteResult{
		Owned:     owned,
		MediaType: mediaType,
		Charset:   params["charset"],
		SafeName:  safeName,
		Extension: extension,
		Display:   display,
	}, nil
}

func readRemoteBody(ctx context.Context, body io.ReadCloser, limit int64) (OwnedBytes, error) {
	done := make(chan struct{})
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		select {
		case <-ctx.Done():
			_ = body.Close()
		case <-done:
		}
	}()
	owned, err := ReadBounded(ctx, body, limit)
	close(done)
	_ = body.Close()
	<-joined
	if ctx.Err() != nil {
		return OwnedBytes{}, newClassifiedError(ErrCancellation, ctx.Err())
	}
	return owned, err
}

func classifyRemoteFailure(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return newClassifiedError(ErrCancellation, ctx.Err())
	}
	return newClassifiedError(ErrRemoteFailure, nil)
}

func parseRemoteURL(raw string) (*url.URL, error) {
	if containsControl(raw) {
		return nil, newClassifiedError(ErrRemoteDenied, nil)
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" {
		return nil, newClassifiedError(ErrRemoteDenied, nil)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, newClassifiedError(ErrRemoteDenied, nil)
	}
	if _, err := remotePort(parsed); err != nil {
		return nil, err
	}
	parsed.Fragment = ""
	return parsed, nil
}

func remotePort(parsed *url.URL) (string, error) {
	port := parsed.Port()
	if port == "" {
		if strings.Contains(parsed.Host, ":") && !strings.HasPrefix(parsed.Host, "[") {
			return "", newClassifiedError(ErrRemoteDenied, nil)
		}
		if parsed.Scheme == "https" {
			return "443", nil
		}
		return "80", nil
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", newClassifiedError(ErrRemoteDenied, nil)
	}
	return strconv.Itoa(number), nil
}

// NewPinnedTransport constructs the no-proxy, no-decompression transport used
// after admission. The approved address, rather than the hostname, is always
// the dial target; the hostname remains the TLS verification and SNI authority.
func NewPinnedTransport(host string, address netip.Addr, dial func(context.Context, string, string) (net.Conn, error)) *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = nil
	base.DisableCompression = true
	base.DisableKeepAlives = true
	base.DialTLSContext = nil
	tlsConfig := base.TLSClientConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	} else {
		tlsConfig = tlsConfig.Clone()
	}
	tlsConfig.ServerName = strings.ToLower(host)
	base.TLSClientConfig = tlsConfig
	base.DialContext = func(ctx context.Context, network, requested string) (net.Conn, error) {
		_, port, err := net.SplitHostPort(requested)
		if err != nil {
			return nil, newClassifiedError(ErrRemoteDenied, nil)
		}
		return dial(ctx, network, net.JoinHostPort(address.String(), port))
	}
	return base
}

// IsAllowedRemoteAddress rejects non-global and special-purpose address
// classes before the admitted address can become a dial target.
func IsAllowedRemoteAddress(address netip.Addr) bool {
	if !address.IsValid() {
		return false
	}
	address = address.Unmap()
	if globallyReachable, registered := specialPurposeReachability(address); registered {
		return globallyReachable
	}
	if address.IsUnspecified() || address.IsLoopback() || address.IsMulticast() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsPrivate() ||
		!address.IsGlobalUnicast() {
		return false
	}
	// IPv6 site-local unicast was deprecated before the current IANA registry,
	// but it remains non-global address space and must not become a dial target.
	if netip.MustParsePrefix("fec0::/10").Contains(address) {
		return false
	}
	return true
}

type specialPurposeAddressRule struct {
	prefix            netip.Prefix
	globallyReachable bool
}

func specialPurposeReachability(address netip.Addr) (bool, bool) {
	matchedBits := -1
	globallyReachable := false
	for _, rule := range specialPurposeAddressRules() {
		if rule.prefix.Contains(address) && rule.prefix.Bits() > matchedBits {
			matchedBits = rule.prefix.Bits()
			globallyReachable = rule.globallyReachable
		}
	}
	return globallyReachable, matchedBits >= 0
}

// specialPurposeAddressRules mirrors the Globally Reachable column of the
// IANA IPv4 and IPv6 Special-Purpose Address Space registries. Blank and N/A
// reachability values fail closed. Longest-prefix matching lets explicit
// globally reachable assignments override a non-global registry umbrella.
//
// Sources (revision 2025-10-09):
//   - https://www.iana.org/assignments/iana-ipv4-special-registry/
//   - https://www.iana.org/assignments/iana-ipv6-special-registry/
func specialPurposeAddressRules() []specialPurposeAddressRule {
	rule := func(prefix string, globallyReachable bool) specialPurposeAddressRule {
		return specialPurposeAddressRule{prefix: netip.MustParsePrefix(prefix), globallyReachable: globallyReachable}
	}
	return []specialPurposeAddressRule{
		// IPv4 registry.
		rule("0.0.0.0/8", false),
		rule("10.0.0.0/8", false),
		rule("100.64.0.0/10", false),
		rule("127.0.0.0/8", false),
		rule("169.254.0.0/16", false),
		rule("172.16.0.0/12", false),
		rule("192.0.0.0/24", false),
		rule("192.0.0.9/32", true),
		rule("192.0.0.10/32", true),
		rule("192.0.2.0/24", false),
		rule("192.31.196.0/24", true),
		rule("192.52.193.0/24", true),
		rule("192.88.99.0/24", false),
		rule("192.88.99.2/32", false),
		rule("192.168.0.0/16", false),
		rule("192.175.48.0/24", true),
		rule("198.18.0.0/15", false),
		rule("198.51.100.0/24", false),
		rule("203.0.113.0/24", false),
		rule("240.0.0.0/4", false),
		rule("255.255.255.255/32", false),

		// IPv6 registry.
		rule("::/128", false),
		rule("::1/128", false),
		rule("::ffff:0:0/96", false),
		rule("64:ff9b::/96", true),
		rule("64:ff9b:1::/48", false),
		rule("100::/64", false),
		rule("100:0:0:1::/64", false),
		rule("2001::/23", false),
		rule("2001::/32", false),
		rule("2001:1::1/128", true),
		rule("2001:1::2/128", true),
		rule("2001:1::3/128", true),
		rule("2001:2::/48", false),
		rule("2001:3::/32", true),
		rule("2001:4:112::/48", true),
		rule("2001:10::/28", false),
		rule("2001:20::/28", true),
		rule("2001:30::/28", true),
		rule("2001:db8::/32", false),
		rule("2002::/16", false),
		rule("2620:4f:8000::/48", true),
		rule("3fff::/20", false),
		rule("5f00::/16", false),
		rule("fc00::/7", false),
		rule("fe80::/10", false),
	}
}

func isRedirectStatus(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func stripSensitiveHeaders(header http.Header) {
	for _, name := range []string{"Authorization", "Proxy-Authorization", "Cookie"} {
		header.Del(name)
	}
}

func splitRemoteContentType(value string) (string, map[string]string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", map[string]string{}
	}
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil {
		return "", map[string]string{}
	}
	canonical, err := CanonicalMediaType(mediaType)
	if err != nil {
		return "", map[string]string{}
	}
	charset := strings.ToLower(strings.TrimSpace(params["charset"]))
	if charset != "" {
		if _, err := NewFact("charset", charset, OriginSource); err != nil {
			charset = ""
		}
	}
	return canonical, map[string]string{"charset": charset}
}
