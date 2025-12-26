package vii_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	vii "github.com/phillip-england/vii/vii"
)

type corsTestRoute struct {
	handle func(r *http.Request, w http.ResponseWriter) error
}

func (corsTestRoute) OnMount(app *vii.App) error { _ = app; return nil }
func (corsTestRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {
	_ = r
	http.Error(w, err.Error(), 500)
}
func (rt corsTestRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	if rt.handle == nil {
		w.WriteHeader(204)
		return nil
	}
	return rt.handle(r, w)
}

func TestCORS_Defaults_ReflectOrigin_AndSetsVary(t *testing.T) {
	app := vii.New()
	app.Use(vii.CORSService{}) // defaults

	if err := app.Mount(http.MethodGet, "/x", corsTestRoute{
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

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/x", nil)
	req.Header.Set("Origin", "https://example.com")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("expected allow-origin reflected, got %q", got)
	}

	// Vary should include Origin (and preflight vary keys)
	vary := strings.Join(res.Header.Values("Vary"), ",")
	if !strings.Contains(strings.ToLower(vary), "origin") {
		t.Fatalf("expected Vary to include Origin, got %q", vary)
	}
}

func TestCORS_AllowList_AllowsAndDenies(t *testing.T) {
	app := vii.New()
	app.Use(vii.CORSService{
		Origin: []string{"https://good.com"},
	})

	if err := app.Mount(http.MethodGet, "/x", corsTestRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			w.WriteHeader(200)
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	// allowed
	{
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/x", nil)
		req.Header.Set("Origin", "https://good.com")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()

		if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://good.com" {
			t.Fatalf("expected allow-origin, got %q", got)
		}
	}

	// denied
	{
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/x", nil)
		req.Header.Set("Origin", "https://bad.com")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()

		if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("expected no allow-origin for denied origin, got %q", got)
		}
	}
}

func TestCORS_Preflight_SetsAllowMethodsHeadersMaxAge(t *testing.T) {
	app := vii.New()
	app.Use(vii.CORSService{
		Origin:        true, // reflect
		Methods:       []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type", "X-Token"},
		MaxAgeSeconds:  123,
	})

	// Route decides to terminate OPTIONS (service sets headers).
	if err := app.Mount(http.MethodOptions, "/x", corsTestRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			w.WriteHeader(204)
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/x", nil)
	req.Header.Set("Origin", "https://client.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Token, Content-Type")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://client.com" {
		t.Fatalf("expected reflected origin, got %q", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Methods"); got != "GET, POST" {
		t.Fatalf("expected allow-methods %q, got %q", "GET, POST", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Headers"); got != "Content-Type, X-Token" {
		t.Fatalf("expected allow-headers %q, got %q", "Content-Type, X-Token", got)
	}
	if got := res.Header.Get("Access-Control-Max-Age"); got != "123" {
		t.Fatalf("expected max-age 123, got %q", got)
	}
}

func TestCORS_Credentials_NeverUsesStar(t *testing.T) {
	app := vii.New()
	app.Use(vii.CORSService{
		Origin:       "*",
		Credentials:  true,
		AllowedHeaders: []string{"Content-Type"},
	})

	if err := app.Mount(http.MethodGet, "/x", corsTestRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			w.WriteHeader(200)
			return nil
		},
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/x", nil)
	req.Header.Set("Origin", "https://client.com")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://client.com" {
		t.Fatalf("expected reflected origin when credentials=true, got %q", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected allow-credentials true, got %q", got)
	}
}
