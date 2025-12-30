package vii_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	vii "github.com/phillip-england/vii/vii"
)

type Config struct {
	Port int
	Env  string
}

func TestApp_InjectAndRetrieve(t *testing.T) {
	app := vii.New()
	key := vii.InjectionKey("config")
	cfg := Config{Port: 8080, Env: "dev"}

	app.Inject(key, cfg)

	var retrievedConfig Config
	var ok bool

	// Define a route that attempts to retrieve the injection
	rt := &testRoute{
		log: &[]string{},
		handleFn: func(r *http.Request, w http.ResponseWriter) error {
			retrievedConfig, ok = vii.Injection[Config](r, key)
			return nil
		},
	}

	if err := app.Mount(http.MethodGet, "/inject", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/inject", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if !ok {
		t.Fatalf("expected injection retrieval to succeed")
	}
	if retrievedConfig != cfg {
		t.Fatalf("expected config %v, got %v", cfg, retrievedConfig)
	}
}

func TestApp_Inject_MissingKey(t *testing.T) {
	app := vii.New()
	key := vii.InjectionKey("missing")

	var ok bool
	rt := &testRoute{
		log: &[]string{},
		handleFn: func(r *http.Request, w http.ResponseWriter) error {
			_, ok = vii.Injection[Config](r, key)
			return nil
		},
	}

	app.Mount(http.MethodGet, "/missing", rt)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	app.ServeHTTP(httptest.NewRecorder(), req)

	if ok {
		t.Fatalf("expected injection retrieval to fail for missing key")
	}
}

func TestApp_Inject_WrongType(t *testing.T) {
	app := vii.New()
	key := vii.InjectionKey("config")
	app.Inject(key, "not a struct") // Injecting a string

	var ok bool
	rt := &testRoute{
		log: &[]string{},
		handleFn: func(r *http.Request, w http.ResponseWriter) error {
			_, ok = vii.Injection[Config](r, key) // Trying to retrieve as Config struct
			return nil
		},
	}

	app.Mount(http.MethodGet, "/wrongtype", rt)
	req := httptest.NewRequest(http.MethodGet, "/wrongtype", nil)
	app.ServeHTTP(httptest.NewRecorder(), req)

	if ok {
		t.Fatalf("expected injection retrieval to fail for wrong type")
	}
}
