package ingestion

import (
	"errors"
	"mime"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrUnsafeMetadata classifies metadata that is ambiguous, authority-bearing,
// malformed, or unsafe to retain in public provenance.
var ErrUnsafeMetadata = errors.New("unsafe ingestion metadata")

// FactOrigin records the authority that supplied a canonical fact.
type FactOrigin string

const (
	OriginCaller    FactOrigin = "caller"
	OriginSource    FactOrigin = "source"
	OriginSniff     FactOrigin = "sniff"
	OriginConverter FactOrigin = "converter"
)

// Fact is a canonical safe scalar ready for explicit translation into public
// provenance.
type Fact struct {
	Kind   string
	Value  string
	Origin FactOrigin
}

var safeTokenPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._+-]*$`)

// CanonicalLogicalName validates a single safe display name.
func CanonicalLogicalName(value string) (string, error) {
	if containsControl(value) {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	value = strings.TrimSpace(value)
	if !safePathValue(value, false) {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	return value, nil
}

// CanonicalArchivePath validates a forward-slash relative archive path without
// cleaning an unsafe input into a different accepted location.
func CanonicalArchivePath(value string) (string, error) {
	if containsControl(value) {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	value = strings.TrimSpace(value)
	if !safePathValue(value, true) {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	return value, nil
}

// CanonicalURLDisplay retains only a canonical HTTP(S) origin. Credentials,
// path payloads, queries, and fragments never cross the metadata boundary.
func CanonicalURLDisplay(value string) (string, error) {
	if containsControl(value) {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Opaque != "" {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" || parsed.Host == "" {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	hostname := strings.ToLower(parsed.Hostname())
	if !safeHostname(hostname) {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	port := parsed.Port()
	if port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return "", newClassifiedError(ErrUnsafeMetadata, nil)
		}
		if scheme == "http" && portNumber == 80 || scheme == "https" && portNumber == 443 {
			port = ""
		} else {
			port = strconv.Itoa(portNumber)
		}
	}
	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	}
	return scheme + "://" + host, nil
}

// CanonicalMediaType strips surrounding whitespace and canonicalizes a bare
// media type. Parameters are rejected because they require separate safe facts.
func CanonicalMediaType(value string) (string, error) {
	if containsControl(value) {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil || mediaType == "" || len(params) != 0 {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	mediaType = strings.ToLower(mediaType)
	if strings.ContainsAny(mediaType, "\r\n\x00") {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	return mediaType, nil
}

// CanonicalExtension returns one lowercase dot-prefixed extension fact.
func CanonicalExtension(value string) (string, error) {
	if containsControl(value) {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	if !strings.HasPrefix(value, ".") {
		value = "." + value
	}
	if value == "." || len(value) > 64 || strings.ContainsAny(value, `/\\?#:%`) {
		return "", newClassifiedError(ErrUnsafeMetadata, nil)
	}
	for _, part := range strings.Split(value[1:], ".") {
		if !safeTokenPattern.MatchString(part) {
			return "", newClassifiedError(ErrUnsafeMetadata, nil)
		}
	}
	return value, nil
}

// NewFact validates a registered fact kind, its origin, and its canonical value.
func NewFact(kind, value string, origin FactOrigin) (Fact, error) {
	if !validOrigin(origin) {
		return Fact{}, newClassifiedError(ErrUnsafeMetadata, nil)
	}
	var canonical string
	var err error
	switch kind {
	case "media_type":
		canonical, err = CanonicalMediaType(value)
	case "extension":
		canonical, err = CanonicalExtension(value)
	case "filename":
		canonical, err = CanonicalLogicalName(value)
	case "charset":
		if containsControl(value) {
			return Fact{}, newClassifiedError(ErrUnsafeMetadata, nil)
		}
		canonical = strings.ToLower(strings.TrimSpace(value))
		if !safeTokenPattern.MatchString(canonical) {
			err = newClassifiedError(ErrUnsafeMetadata, nil)
		}
	default:
		err = newClassifiedError(ErrUnsafeMetadata, nil)
	}
	if err != nil {
		return Fact{}, err
	}
	return Fact{Kind: kind, Value: canonical, Origin: origin}, nil
}

func validOrigin(origin FactOrigin) bool {
	for _, accepted := range factOrigins() {
		if origin == accepted {
			return true
		}
	}
	return false
}

// factOrigins is the ordered internal mirror of the v1 schema's closed origin
// vocabulary. Validation and contract-mirror tests both enumerate this value.
func factOrigins() [4]FactOrigin {
	return [...]FactOrigin{OriginCaller, OriginSource, OriginSniff, OriginConverter}
}

func safePathValue(value string, allowSlash bool) bool {
	if value == "" || len(value) > 4096 || !utf8.ValidString(value) ||
		strings.ContainsAny(value, `\?#:`) || filepath.IsAbs(value) || portableAbsolute(value) {
		return false
	}
	if !allowSlash && strings.Contains(value, "/") {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if !safePathSegment(segment) {
			return false
		}
	}
	return inspectDecoded(value)
}

const maxDecodeRounds = 16

func inspectDecoded(value string) bool {
	current := value
	for round := 0; round <= maxDecodeRounds; round++ {
		if !safeDecodedForm(current) {
			return false
		}
		decoded, err := url.PathUnescape(current)
		if err != nil {
			return false
		}
		if decoded == current {
			return true
		}
		if strings.Count(decoded, "/") > strings.Count(current, "/") ||
			strings.Count(decoded, `\`) > strings.Count(current, `\`) {
			return false
		}
		if round == maxDecodeRounds {
			return false
		}
		current = decoded
	}
	return false
}

func safeDecodedForm(value string) bool {
	if !utf8.ValidString(value) || strings.ContainsAny(value, `\?#:`) || filepath.IsAbs(value) || portableAbsolute(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if !safePathSegment(segment) {
			return false
		}
	}
	lower := strings.ToLower(value)
	for _, forbidden := range []string{"data:", "authorization:", "proxy-authorization:", "bearer ", "basic "} {
		if strings.Contains(lower, forbidden) {
			return false
		}
	}
	return true
}

func containsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func safePathSegment(segment string) bool {
	return segment != "" && segment != "." && segment != ".." &&
		!strings.HasSuffix(segment, ".") && !strings.HasSuffix(segment, " ")
}

func portableAbsolute(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) {
		return true
	}
	return len(value) >= 3 && isASCIIAlpha(value[0]) && value[1] == ':' && (value[2] == '/' || value[2] == '\\')
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func safeHostname(value string) bool {
	if value == "" || len(value) > 253 || strings.ContainsAny(value, "\x00\r\n@/\\?#") {
		return false
	}
	if net.ParseIP(value) != nil {
		return true
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}
