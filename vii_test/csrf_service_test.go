package vii_test

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	vii "github.com/phillip-england/vii/vii"
)

type csrfTestRoute struct {
	handle func(r *http.Request, w http.ResponseWriter) error
	onErr  func(r *http.Request, w http.ResponseWriter, err error)
}

func (csrfTestRoute) OnMount(app *vii.App) error { _ = app; return nil }

func (rt csrfTestRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {
	if rt.onErr != nil {
		rt.onErr(r, w, err)
		return
	}
	http.Error(w, err.Error(), http.StatusForbidden)
}

func (rt csrfTestRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	if rt.handle == nil {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	return rt.handle(r, w)
}

type csrfMetrics struct {
	gen    int
	val    int
	failed map[string]int
	skip   map[string]int
}

func newCSRFMetrics() *csrfMetrics {
	return &csrfMetrics{
		failed: map[string]int{},
		skip:   map[string]int{},
	}
}

func (m *csrfMetrics) Generated()            { m.gen++ }
func (m *csrfMetrics) Validated()            { m.val++ }
func (m *csrfMetrics) Failed(reason string)  { m.failed[reason]++ }
func (m *csrfMetrics) Skipped(reason string) { m.skip[reason]++ }

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func TestCSRF_GeneratesCookieOnSafeRequest_AndExposesToken(t *testing.T) {
	app := vii.New()
	m := newCSRFMetrics()
	app.Use(vii.CSRFService{Metrics: m})

	if err := app.Mount(http.MethodGet, "/token", csrfTestRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			tok, ok := vii.Valid(r, vii.CSRFKey)
			if !ok || tok.Value == "" {
				http.Error(w, "missing token", 500)
				return nil
			}
			_, _ = w.Write([]byte(tok.Value))
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	c := newClient()
	res, err := c.Get(ts.URL + "/token")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	// Ensure cookie is set.
	found := false
	for _, ck := range res.Cookies() {
		if ck.Name == "csrf" && ck.Value != "" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected csrf cookie to be set")
	}

	if m.gen != 1 {
		t.Fatalf("expected metrics.Generated() once, got %d", m.gen)
	}
	if m.val != 0 {
		t.Fatalf("expected metrics.Validated() 0, got %d", m.val)
	}
}

func TestCSRF_UnsafeRequest_ValidatesHeaderMatchesCookie(t *testing.T) {
	app := vii.New()
	m := newCSRFMetrics()
	app.Use(vii.CSRFService{Metrics: m})

	// Safe endpoint to mint cookie.
	if err := app.Mount(http.MethodGet, "/token", csrfTestRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			tok, _ := vii.Valid(r, vii.CSRFKey)
			_, _ = w.Write([]byte(tok.Value))
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	// Protected endpoint.
	if err := app.Mount(http.MethodPost, "/submit", csrfTestRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			// token is also exposed on success
			if tok, ok := vii.Valid(r, vii.CSRFKey); !ok || tok.Value == "" {
				http.Error(w, "missing token in handler", 500)
				return nil
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte("ok"))
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	c := newClient()

	// 1) GET /token to mint cookie + capture token string.
	res, err := c.Get(ts.URL + "/token")
	if err != nil {
		t.Fatalf("get token: %v", err)
	}
	defer res.Body.Close()

	buf := make([]byte, 4096)
	n, _ := res.Body.Read(buf)
	tok := strings.TrimSpace(string(buf[:n]))
	if tok == "" {
		t.Fatalf("expected non-empty token body")
	}

	// 2) POST /submit with header token; cookiejar provides cookie automatically.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/submit", strings.NewReader("x=1"))
	req.Header.Set("X-CSRF-Token", tok)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res2, err := c.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res2.Body.Close()

	if res2.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res2.StatusCode)
	}

	if m.gen != 1 {
		t.Fatalf("expected metrics.Generated() once (from GET), got %d", m.gen)
	}
	if m.val != 1 {
		t.Fatalf("expected metrics.Validated() once (from POST), got %d", m.val)
	}
	if got := m.failed["mismatch"]; got != 0 {
		t.Fatalf("expected mismatch failures 0, got %d", got)
	}
}

func TestCSRF_Mismatch_ReturnsErrorAndMetrics(t *testing.T) {
	app := vii.New()
	m := newCSRFMetrics()
	app.Use(vii.CSRFService{Metrics: m})

	if err := app.Mount(http.MethodGet, "/token", csrfTestRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			tok, _ := vii.Valid(r, vii.CSRFKey)
			_, _ = w.Write([]byte(tok.Value))
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	var gotErr error
	if err := app.Mount(http.MethodPost, "/submit", csrfTestRoute{
		onErr: func(r *http.Request, w http.ResponseWriter, err error) {
			_ = r
			gotErr = err
			http.Error(w, "csrf", http.StatusForbidden)
		},
		handle: func(r *http.Request, w http.ResponseWriter) error {
			w.WriteHeader(200)
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	c := newClient()

	// mint cookie
	_, err := c.Get(ts.URL + "/token")
	if err != nil {
		t.Fatalf("get token: %v", err)
	}

	// post with wrong token
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/submit", strings.NewReader("x=1"))
	req.Header.Set("X-CSRF-Token", "not-the-cookie")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.StatusCode)
	}
	if gotErr == nil {
		t.Fatalf("expected route.OnErr to receive error")
	}
	if !errors.Is(gotErr, vii.ErrCSRFTokenMismatch) {
		t.Fatalf("expected ErrCSRFTokenMismatch, got %v", gotErr)
	}
	if m.failed["mismatch"] != 1 {
		t.Fatalf("expected metrics.Failed(mismatch)=1, got %d", m.failed["mismatch"])
	}
}

func TestCSRF_MissingCookieOrToken_Fails(t *testing.T) {
	app := vii.New()
	m := newCSRFMetrics()
	app.Use(vii.CSRFService{Metrics: m})

	var gotErr error
	if err := app.Mount(http.MethodPost, "/submit", csrfTestRoute{
		onErr: func(r *http.Request, w http.ResponseWriter, err error) {
			_ = r
			gotErr = err
			http.Error(w, "csrf", http.StatusForbidden)
		},
		handle: func(r *http.Request, w http.ResponseWriter) error {
			w.WriteHeader(200)
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	// No cookie jar, no prior GET -> should fail cookie_missing.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/submit", strings.NewReader("x=1"))
	req.Header.Set("X-CSRF-Token", "anything")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.StatusCode)
	}
	if gotErr == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(gotErr, vii.ErrCSRFTokenMissing) {
		t.Fatalf("expected ErrCSRFTokenMissing, got %v", gotErr)
	}
	if m.failed["cookie_missing"] != 1 {
		t.Fatalf("expected metrics.Failed(cookie_missing)=1, got %d", m.failed["cookie_missing"])
	}
}

func TestCSRF_Skip_Bypass(t *testing.T) {
	app := vii.New()
	m := newCSRFMetrics()
	app.Use(vii.CSRFService{
		Metrics: m,
		Skip: func(r *http.Request) (bool, string) {
			if r.URL != nil && r.URL.Path == "/webhook" {
				return true, "webhook"
			}
			return false, ""
		},
	})

	if err := app.Mount(http.MethodPost, "/webhook", csrfTestRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("ok"))
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", strings.NewReader("x=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if m.skip["webhook"] != 1 {
		t.Fatalf("expected metrics.Skipped(webhook)=1, got %d", m.skip["webhook"])
	}
}
