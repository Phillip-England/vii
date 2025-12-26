package vii

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
)

// CSRFKey allows retrieving the CSRF token by key:
//
//	tok, ok := vii.Valid(r, vii.CSRFKey)
var CSRFKey = NewKey[CSRFToken]("csrf")

// CSRFToken is the per-request token (copied from cookie and/or request).
type CSRFToken struct {
	Value string
}

// CSRFMetrics is optional instrumentation.
// The default is no-op; plug in Prometheus/OpenTelemetry/etc by implementing these hooks.
type CSRFMetrics interface {
	Generated()
	Validated()
	Failed(reason string)
	Skipped(reason string)
}

// noop metrics (default)
type csrfNoopMetrics struct{}

func (csrfNoopMetrics) Generated()                {}
func (csrfNoopMetrics) Validated()                {}
func (csrfNoopMetrics) Failed(reason string)      {}
func (csrfNoopMetrics) Skipped(reason string)     {}
func (csrfNoopMetrics) String() string            { return "noop" }
func (csrfNoopMetrics) Observe(_ time.Duration)   {} // reserved if you want to extend later
func (csrfNoopMetrics) Observe2(_ string, _ int64) {} // reserved

// CSRFService provides CSRF protection using the double-submit cookie pattern.
// Safe methods ensure a cookie exists; unsafe methods require the client echo
// the cookie value via header or form field.
type CSRFService struct {
	// CookieName defaults to "csrf".
	CookieName string

	// HeaderName defaults to "X-CSRF-Token".
	HeaderName string

	// FormField defaults to "csrf_token".
	FormField string

	// ProtectMethods defaults to POST, PUT, PATCH, DELETE.
	ProtectMethods []string

	// CookiePath defaults to "/".
	CookiePath string

	// SameSite defaults to Lax.
	SameSite http.SameSite

	// Secure, if nil, defaults to (r.TLS != nil).
	Secure *bool

	// HttpOnly defaults to false (so JS can read cookie if you choose header-based clients).
	HttpOnly bool

	// MaxAgeSeconds defaults to 0 (session cookie).
	MaxAgeSeconds int

	// Skip can bypass CSRF enforcement for certain requests (e.g. webhook endpoints).
	// Return true to skip.
	Skip func(r *http.Request) (bool, string)

	// Metrics defaults to no-op.
	Metrics CSRFMetrics
}

var (
	ErrCSRFTokenMissing  = errors.New("vii: csrf token missing")
	ErrCSRFTokenMismatch = errors.New("vii: csrf token mismatch")
)

func (s CSRFService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	if r == nil || w == nil {
		return r, nil
	}

	cfg := s.withDefaults(r)

	if cfg.Skip != nil {
		if ok, reason := cfg.Skip(r); ok {
			cfg.Metrics.Skipped(reason)
			return r, nil
		}
	}

	// Always try to ensure a cookie exists on "safe" methods.
	if isSafeMethod(r.Method) {
		cTok, ok := readCSRFCookie(r, cfg.CookieName)
		if !ok || cTok == "" {
			newTok, err := newCSRFToken()
			if err != nil {
				cfg.Metrics.Failed("token_generate")
				return r, err
			}
			writeCSRFCookie(w, r, cfg, newTok)
			cfg.Metrics.Generated()
			r = ProvideKey(r, CSRFKey, CSRFToken{Value: newTok})
			return r, nil
		}

		// Make token available to handlers.
		r = ProvideKey(r, CSRFKey, CSRFToken{Value: cTok})
		return r, nil
	}

	// Unsafe methods: enforce.
	if !methodIn(r.Method, cfg.ProtectMethods) {
		// Not protected, but still expose token if present.
		if cTok, ok := readCSRFCookie(r, cfg.CookieName); ok && cTok != "" {
			r = ProvideKey(r, CSRFKey, CSRFToken{Value: cTok})
		}
		cfg.Metrics.Skipped("method_not_protected")
		return r, nil
	}

	cTok, ok := readCSRFCookie(r, cfg.CookieName)
	if !ok || cTok == "" {
		cfg.Metrics.Failed("cookie_missing")
		return r, ErrCSRFTokenMissing
	}

	reqTok := readCSRFRequestToken(r, cfg.HeaderName, cfg.FormField)
	if reqTok == "" {
		cfg.Metrics.Failed("request_token_missing")
		return r, ErrCSRFTokenMissing
	}

	if !secureEqual(cTok, reqTok) {
		cfg.Metrics.Failed("mismatch")
		return r, ErrCSRFTokenMismatch
	}

	// Success; expose token to handlers.
	cfg.Metrics.Validated()
	r = ProvideKey(r, CSRFKey, CSRFToken{Value: cTok})
	return r, nil
}

func (s CSRFService) After(r *http.Request, w http.ResponseWriter) error {
	_ = r
	_ = w
	return nil
}

func (s CSRFService) withDefaults(r *http.Request) CSRFService {
	out := s

	if out.CookieName == "" {
		out.CookieName = "csrf"
	}
	if out.HeaderName == "" {
		out.HeaderName = "X-CSRF-Token"
	}
	if out.FormField == "" {
		out.FormField = "csrf_token"
	}
	if len(out.ProtectMethods) == 0 {
		out.ProtectMethods = []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	}
	if out.CookiePath == "" {
		out.CookiePath = "/"
	}
	if out.SameSite == 0 {
		out.SameSite = http.SameSiteLaxMode
	}
	if out.Metrics == nil {
		out.Metrics = csrfNoopMetrics{}
	}
	// HttpOnly default is false (zero value) -> keep as-is.
	// MaxAgeSeconds default is 0 -> keep as-is.

	// Secure default: true only if TLS is present.
	if out.Secure == nil {
		sec := (r.TLS != nil)
		out.Secure = &sec
	}

	return out
}

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func methodIn(method string, list []string) bool {
	for _, m := range list {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

func readCSRFCookie(r *http.Request, name string) (string, bool) {
	c, err := r.Cookie(name)
	if err != nil || c == nil {
		return "", false
	}
	return c.Value, c.Value != ""
}

func writeCSRFCookie(w http.ResponseWriter, r *http.Request, cfg CSRFService, tok string) {
	secure := false
	if cfg.Secure != nil {
		secure = *cfg.Secure
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    tok,
		Path:     cfg.CookiePath,
		MaxAge:   cfg.MaxAgeSeconds,
		Secure:   secure,
		HttpOnly: cfg.HttpOnly,
		SameSite: cfg.SameSite,
	})
}

func readCSRFRequestToken(r *http.Request, headerName, formField string) string {
	// Prefer header.
	if headerName != "" {
		if v := strings.TrimSpace(r.Header.Get(headerName)); v != "" {
			return v
		}
	}
	// Fallback to form field.
	if formField != "" {
		// Ignore parse errors; if body isn't form-encoded this will no-op.
		_ = r.ParseForm()
		if v := strings.TrimSpace(r.FormValue(formField)); v != "" {
			return v
		}
	}
	return ""
}

func newCSRFToken() (string, error) {
	// 32 bytes -> 43 chars base64url (no padding).
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func secureEqual(a, b string) bool {
	// Constant-time compare over bytes; also constant-time length check behavior.
	if len(a) != len(b) {
		// Compare anyway to avoid early exit timing patterns.
		min := len(a)
		if len(b) < min {
			min = len(b)
		}
		_ = subtle.ConstantTimeCompare([]byte(a[:min]), []byte(b[:min]))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
