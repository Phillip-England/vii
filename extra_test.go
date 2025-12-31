package vii

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestMiddlewareOrder(t *testing.T) {
	var callOrder []string

	globalMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "global_start")
			next.ServeHTTP(w, r)
			callOrder = append(callOrder, "global_end")
		})
	}

	groupMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "group_start")
			next.ServeHTTP(w, r)
			callOrder = append(callOrder, "group_end")
		})
	}

	localMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "local_start")
			next.ServeHTTP(w, r)
			callOrder = append(callOrder, "local_end")
		})
	}

	app := NewApp()
	app.Use(globalMw)

	g := app.Group("/api")
	g.Use(groupMw)

	g.At("GET /test", func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	}, localMw)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	// app.Mux.ServeHTTP(w, req) // app.ServeHTTP is not defined in vii.go, using Mux.ServeHTTP directly or Serve() wrapper but Serve() blocks.
	// Wait, app.At registers to app.Mux.
	// But Global Middleware is applied in Serve().
	// vii.go: Serve() -> chain(app.Mux.ServeHTTP, app.GlobalMiddleware...).
	// So to test global middleware we need to manually chain it or use a helper.
	
	handler := chain(app.Mux.ServeHTTP, app.GlobalMiddleware...)
	handler.ServeHTTP(w, req)

	expected := []string{
		"global_start",
		"group_start",
		"local_start",
		"handler",
		"local_end",
		"group_end",
		"global_end",
	}

	if len(callOrder) != len(expected) {
		t.Fatalf("Expected %d calls, got %d: %v", len(expected), len(callOrder), callOrder)
	}

	for i, v := range expected {
		if callOrder[i] != v {
			t.Errorf("Order mismatch at index %d: expected %s, got %s", i, v, callOrder[i])
		}
	}
}

func TestGroupRouting(t *testing.T) {
	app := NewApp()
	g1 := app.Group("/v1")
	g2 := app.Group("/v2")

	g1.At("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v1 hello"))
	})

	g2.At("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v2 hello"))
	})

	// Test v1
	req1 := httptest.NewRequest("GET", "/v1/hello", nil)
	w1 := httptest.NewRecorder()
	app.Mux.ServeHTTP(w1, req1)
	if w1.Body.String() != "v1 hello" {
		t.Errorf("Expected 'v1 hello', got '%s'", w1.Body.String())
	}

	// Test v2
	req2 := httptest.NewRequest("GET", "/v2/hello", nil)
	w2 := httptest.NewRecorder()
	app.Mux.ServeHTTP(w2, req2)
	if w2.Body.String() != "v2 hello" {
		t.Errorf("Expected 'v2 hello', got '%s'", w2.Body.String())
	}
}

func TestGlobalContext(t *testing.T) {
	app := NewApp()
	app.SetContext("config", "production")

	app.At("GET /ctx", func(w http.ResponseWriter, r *http.Request) {
		val := GetContext("config", r)
		strVal, ok := val.(string)
		if !ok {
			t.Errorf("Expected string value, got %T: %v", val, val)
			return
		}
		
		if strVal != "production" {
			t.Errorf("Expected config 'production', got '%s'", strVal)
		}
	})

	req := httptest.NewRequest("GET", "/ctx", nil)
	w := httptest.NewRecorder()
	app.Mux.ServeHTTP(w, req)
}

func TestCORS(t *testing.T) {
	app := NewApp()
	app.Use(MwCORS)

	app.At("GET /cors", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	handler := chain(app.Mux.ServeHTTP, app.GlobalMiddleware...)

	// Test standard request
	req := httptest.NewRequest("GET", "/cors", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected Allow-Origin '*', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRequestHelpers(t *testing.T) {
	req := httptest.NewRequest("GET", "/?q=hello&id=123", nil)
	
	t.Run("Param", func(t *testing.T) {
		if val := Param(req, "q"); val != "hello" {
			t.Errorf("Expected param 'hello', got '%s'", val)
		}
	})

	t.Run("ParamIs", func(t *testing.T) {
		if !ParamIs(req, "q", "hello") {
			t.Error("ParamIs expected true for q=hello")
		}
	})
}

func TestResponseHelpers(t *testing.T) {
	t.Run("WriteHTML", func(t *testing.T) {
		w := httptest.NewRecorder()
		WriteHTML(w, http.StatusOK, "<h1>Hello</h1>")
		if w.Header().Get("Content-Type") != "text/html" {
			t.Errorf("Expected Content-Type text/html, got %s", w.Header().Get("Content-Type"))
		}
	})

	t.Run("WriteString", func(t *testing.T) {
		w := httptest.NewRecorder()
		WriteString(w, http.StatusOK, "Hello")
		if w.Header().Get("Content-Type") != "text/plain" {
			t.Errorf("Expected Content-Type text/plain, got %s", w.Header().Get("Content-Type"))
		}
	})

	t.Run("WriteJSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		WriteJSON(w, http.StatusOK, map[string]string{"foo": "bar"})
		if w.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
		}
		expected := `{"foo":"bar"}` // json.Encoder adds newline
		if strings.TrimSpace(w.Body.String()) != expected {
			t.Errorf("Expected body '%s', got '%s'", expected, w.Body.String())
		}
	})
}

func TestFSOperations(t *testing.T) {
	// Mock FS
	mockFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>Index</html>")},
		"static/style.css":  {Data: []byte("body { color: red; }")},
	}

	t.Run("StaticEmbed", func(t *testing.T) {
		app := NewApp()
		app.StaticEmbed("/static", mockFS)

		req := httptest.NewRequest("GET", "/static/static/style.css", nil) // FileServer sees "static/style.css" inside FS
		w := httptest.NewRecorder()
		app.Mux.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Result().StatusCode)
		}
		if w.Body.String() != "body { color: red; }" {
			t.Errorf("Expected body 'body { color: red; }', got '%s'", w.Body.String())
		}
	})

	t.Run("TemplatesFS", func(t *testing.T) {
		app := NewApp()
		// Test wildcard matching if supported, or simple listing
		// Note: fstest.MapFS does not support globbing effectively with standard Glob unless patterns are simple?
		// "templates/*.html"
		// Let's try simple match
		err := app.TemplatesFS(mockFS, "*.html", nil)
		if err != nil {
			t.Fatalf("TemplatesFS failed: %v", err)
		}

		app.At("GET /", func(w http.ResponseWriter, r *http.Request) {
			err := ExecuteTemplate(w, r, "index.html", nil)
			if err != nil {
				t.Errorf("ExecuteTemplate failed: %v", err)
			}
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		app.Mux.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Result().StatusCode)
		}
		if w.Body.String() != "<html>Index</html>" {
			t.Errorf("Expected body '<html>Index</html>', got '%s'", w.Body.String())
		}
	})
}
