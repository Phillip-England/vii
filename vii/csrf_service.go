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

var CSRFKey = NewKey[CSRFToken]("csrf")

type CSRFToken struct {
	Value string
}

type CSRFMetrics interface {
	Generated()
	Validated()
	Failed(reason string)
	Skipped(reason string)
}

type csrfNoopMetrics struct{}

func (csrfNoopMetrics) Generated()                {}
func (csrfNoopMetrics) Validated()                {}
func (csrfNoopMetrics) Failed(reason string)      {}
func (csrfNoopMetrics) Skipped(reason string)     {}
func (csrfNoopMetrics) String() string            { return "noop" }
func (csrfNoopMetrics) Observe(_ time.Duration)   {} // reserved if you want to extend later
func (csrfNoopMetrics) Observe2(_ string, _ int64) {} // reserved

type CSRFService struct {
	CookieName string
	HeaderName string
	FormField  string

	ProtectMethods []string

	CookiePath string
	SameSite   http.SameSite
	Secure     *bool

	// HttpOnly is kept for backward compatibility, but cannot distinguish
	// "unset" vs "explicit false". Prefer CookieHttpOnly if you need false.
	HttpOnly bool

	// CookieHttpOnly overrides HttpOnly behavior when non-nil.
	// Default when nil is true (safer by default).
	CookieHttpOnly *bool

	MaxAgeSeconds int

	Skip    func(r *http.Request) (bool, string)
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
		r = ProvideKey(r, CSRFKey, CSRFToken{Value: cTok})
		return r, nil
	}

	if !methodIn(r.Method, cfg.ProtectMethods) {
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
	if out.Secure == nil {
		sec := (r.TLS != nil)
		out.Secure = &sec
	}

	// Default HttpOnly to true unless explicitly overridden via CookieHttpOnly.
	// (HttpOnly bool can't represent "unset" vs "false", so CookieHttpOnly exists for opt-out.)
	if out.CookieHttpOnly == nil {
		def := true
		out.CookieHttpOnly = &def
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

	httpOnly := true
	if cfg.CookieHttpOnly != nil {
		httpOnly = *cfg.CookieHttpOnly
	} else if cfg.HttpOnly {
		// legacy compatibility: if someone set HttpOnly:true historically
		httpOnly = true
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    tok,
		Path:     cfg.CookiePath,
		MaxAge:   cfg.MaxAgeSeconds,
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: cfg.SameSite,
	})
}

func readCSRFRequestToken(r *http.Request, headerName, formField string) string {
	if headerName != "" {
		if v := strings.TrimSpace(r.Header.Get(headerName)); v != "" {
			return v
		}
	}
	if formField != "" {
		_ = r.ParseForm()
		if v := strings.TrimSpace(r.FormValue(formField)); v != "" {
			return v
		}
	}
	return ""
}

func newCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		min := len(a)
		if len(b) < min {
			min = len(b)
		}
		_ = subtle.ConstantTimeCompare([]byte(a[:min]), []byte(b[:min]))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
