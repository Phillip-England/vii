package vii

import (
	"net/http"
	"regexp"
	"strings"
)

type OriginType any

// CORSService is a global/route service that sets CORS headers.
// It does NOT automatically terminate OPTIONS requests (your route can do that).
type CORSService struct {
	// Origin controls which origins are allowed.
	// Supported types:
	//   - string: "*" or exact origin ("https://example.com")
	//   - []string: allowlist of exact origins
	//   - *regexp.Regexp: origin must match
	//   - func(origin string) bool: custom predicate
	//   - bool(true): reflect request Origin (allow any origin, but reflected)
	Origin OriginType

	Methods        []string // default: common methods
	AllowedHeaders []string // default: Content-Type, Authorization
	ExposedHeaders []string // default: none

	Credentials bool // default: false
	MaxAgeSeconds int // default: 600

	// If true, add Vary headers for safer caching behavior.
	// default: true
	Vary bool
}

func (s CORSService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	if r == nil || w == nil {
		return r, nil
	}
	cfg := s.withDefaults()

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return r, nil // not a CORS request
	}

	allowedOrigin, ok := cfg.allowedOrigin(origin)
	if !ok || allowedOrigin == "" {
		// Not allowed: intentionally don't set any CORS headers.
		return r, nil
	}

	h := w.Header()

	if cfg.Vary {
		appendVary(h, "Origin")
		appendVary(h, "Access-Control-Request-Method")
		appendVary(h, "Access-Control-Request-Headers")
	}

	h.Set("Access-Control-Allow-Origin", allowedOrigin)
	if cfg.Credentials {
		h.Set("Access-Control-Allow-Credentials", "true")
	}

	if len(cfg.ExposedHeaders) > 0 {
		h.Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
	}

	// Preflight hints (route can still decide how to respond to OPTIONS)
	if r.Method == http.MethodOptions && strings.TrimSpace(r.Header.Get("Access-Control-Request-Method")) != "" {
		if len(cfg.Methods) > 0 {
			h.Set("Access-Control-Allow-Methods", strings.Join(cfg.Methods, ", "))
		}
		if len(cfg.AllowedHeaders) > 0 {
			h.Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
		} else if reqHdr := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers")); reqHdr != "" {
			// If user didn't configure AllowedHeaders, reflect requested headers.
			h.Set("Access-Control-Allow-Headers", reqHdr)
		}
		if cfg.MaxAgeSeconds > 0 {
			h.Set("Access-Control-Max-Age", itoa(cfg.MaxAgeSeconds))
		}
	}

	return r, nil
}

func (s CORSService) After(r *http.Request, w http.ResponseWriter) error {
	_ = r
	_ = w
	return nil
}

func (s CORSService) withDefaults() CORSService {
	out := s
	if out.Origin == nil {
		// sensible default: reflect Origin when present (works with credentials too)
		out.Origin = true
	}
	if len(out.Methods) == 0 {
		out.Methods = []string{
			http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
			http.MethodDelete, http.MethodHead, http.MethodOptions,
		}
	}
	if out.AllowedHeaders == nil {
		out.AllowedHeaders = []string{
			"Content-Type",
			"Authorization",
			"X-CSRF-Token",
		}
	}
	if out.MaxAgeSeconds == 0 {
		out.MaxAgeSeconds = 600
	}
	if !out.Vary {
		// default true (only keep false if explicitly set false by user)
		out.Vary = true
	}
	return out
}

func (s CORSService) allowedOrigin(reqOrigin string) (string, bool) {
	switch v := s.Origin.(type) {
	case nil:
		return "", false
	case bool:
		if v {
			return reqOrigin, true // reflect
		}
		return "", false
	case string:
		if v == "*" {
			// If credentials are used, "*" is invalid; reflect instead.
			if s.Credentials {
				return reqOrigin, true
			}
			return "*", true
		}
		if strings.EqualFold(v, reqOrigin) {
			return reqOrigin, true
		}
		return "", false
	case []string:
		for _, o := range v {
			if strings.EqualFold(strings.TrimSpace(o), reqOrigin) {
				return reqOrigin, true
			}
		}
		return "", false
	case *regexp.Regexp:
		if v != nil && v.MatchString(reqOrigin) {
			return reqOrigin, true
		}
		return "", false
	case func(string) bool:
		if v(reqOrigin) {
			return reqOrigin, true
		}
		return "", false
	default:
		return "", false
	}
}

// appendVary adds a value to the Vary header without duplicating values.
func appendVary(h http.Header, value string) {
	if value == "" {
		return
	}
	cur := h.Values("Vary")
	// Header may appear multiple times; normalize to a set.
	set := map[string]bool{}
	for _, line := range cur {
		for _, part := range strings.Split(line, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				set[strings.ToLower(p)] = true
			}
		}
	}
	if !set[strings.ToLower(value)] {
		h.Add("Vary", value)
	}
}

// tiny itoa without fmt/strconv to keep this file minimal
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		n = -n
	}
	var b [32]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + (n % 10))
		n /= 10
	}
	return string(b[i:])
}
