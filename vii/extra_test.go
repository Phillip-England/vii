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

	g.Handle("GET /test", func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	}, localMw)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

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

	g1.Handle("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v1 hello"))
	})

	g2.Handle("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v2 hello"))
	})

	// Test v1
	req1 := httptest.NewRequest("GET", "/v1/hello", nil)
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, req1)
	if w1.Body.String() != "v1 hello" {
		t.Errorf("Expected 'v1 hello', got '%s'", w1.Body.String())
	}

	// Test v2
	req2 := httptest.NewRequest("GET", "/v2/hello", nil)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Body.String() != "v2 hello" {
		t.Errorf("Expected 'v2 hello', got '%s'", w2.Body.String())
	}
}

func TestGlobalContext(t *testing.T) {
	app := NewApp()
	app.SetContext("config", "production")

	app.Handle("GET /ctx", func(w http.ResponseWriter, r *http.Request) {
		// Test retrieval via the helper logic which looks in GLOBAL map
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
	app.ServeHTTP(w, req)
}

func TestCORS(t *testing.T) {
	app := NewApp()
	app.Use(CORS)

	app.Handle("GET /cors", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// Test standard request
	req := httptest.NewRequest("GET", "/cors", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected Allow-Origin '*', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}

	// Test Preflight OPTIONS
	reqOpt := httptest.NewRequest("OPTIONS", "/cors", nil)
	wOpt := httptest.NewRecorder()
	app.ServeHTTP(wOpt, reqOpt)

	if wOpt.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", wOpt.Result().StatusCode)
	}
	if wOpt.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Expected Allow-Methods header")
	}
	
	// Test Origin Header
	reqOrigin := httptest.NewRequest("GET", "/cors", nil)
	reqOrigin.Header.Set("Origin", "http://example.com")
	wOrigin := httptest.NewRecorder()
	app.ServeHTTP(wOrigin, reqOrigin)
	
	if wOrigin.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("Expected Allow-Origin 'http://example.com', got '%s'", wOrigin.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRequestHelpersExtra(t *testing.T) {
	req := httptest.NewRequest("GET", "/?q=hello&id=123", nil)
	req.Header.Set("X-API-Key", "secret")
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc-123"})

	t.Run("Header", func(t *testing.T) {
		if val := Header(req, "X-API-Key"); val != "secret" {
			t.Errorf("Expected header 'secret', got '%s'", val)
		}
	})

	t.Run("Cookie", func(t *testing.T) {
		cookie, err := Cookie(req, "session")
		if err != nil {
			t.Errorf("Cookie error: %v", err)
		}
		if cookie.Value != "abc-123" {
			t.Errorf("Expected cookie value 'abc-123', got '%s'", cookie.Value)
		}
	})

	t.Run("Query", func(t *testing.T) {
		if val := Query(req, "q"); val != "hello" {
			t.Errorf("Expected query 'hello', got '%s'", val)
		}
	})

	t.Run("QueryIs", func(t *testing.T) {
		if !QueryIs(req, "q", "hello") {
			t.Error("QueryIs expected true for q=hello")
		}
		if QueryIs(req, "id", "456") {
			t.Error("QueryIs expected false for id=456")
		}
	})
}

func TestResponseHelpers(t *testing.T) {
	t.Run("WriteError", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := WriteError(w, http.StatusBadRequest, "bad input")
		if err != nil {
			t.Fatalf("WriteError failed: %v", err)
		}
		if w.Result().StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Result().StatusCode)
		}
		expected := `{"error":"bad input"}`
		if strings.TrimSpace(w.Body.String()) != expected {
			t.Errorf("Expected body '%s', got '%s'", expected, w.Body.String())
		}
	})

	t.Run("WriteHTML", func(t *testing.T) {
		w := httptest.NewRecorder()
		WriteHTML(w, http.StatusOK, "<h1>Hello</h1>")
		if w.Header().Get("Content-Type") != "text/html" {
			t.Errorf("Expected Content-Type text/html, got %s", w.Header().Get("Content-Type"))
		}
		if w.Body.String() != "<h1>Hello</h1>" {
			t.Errorf("Expected body '<h1>Hello</h1>', got '%s'", w.Body.String())
		}
	})

	t.Run("WriteText", func(t *testing.T) {
		w := httptest.NewRecorder()
		WriteText(w, http.StatusOK, "Hello")
		if w.Header().Get("Content-Type") != "text/plain" {
			t.Errorf("Expected Content-Type text/plain, got %s", w.Header().Get("Content-Type"))
		}
		if w.Body.String() != "Hello" {
			t.Errorf("Expected body 'Hello', got '%s'", w.Body.String())
		}
	})

	t.Run("SetHeader", func(t *testing.T) {
		w := httptest.NewRecorder()
		SetHeader(w, "X-Custom", "Value")
		if w.Header().Get("X-Custom") != "Value" {
			t.Errorf("Expected header 'Value', got '%s'", w.Header().Get("X-Custom"))
		}
	})

	t.Run("Redirect", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		Redirect(w, req, "/new", http.StatusFound)
		if w.Result().StatusCode != http.StatusFound {
			t.Errorf("Expected status 302, got %d", w.Result().StatusCode)
		}
		if loc, _ := w.Result().Location(); loc.String() != "/new" {
			t.Errorf("Expected location '/new', got '%s'", loc)
		}
	})
}

func TestFSOperations(t *testing.T) {
	// Mock FS
	mockFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>Index</html>")},
		"style.css":  {Data: []byte("body { color: red; }")},
	}

	t.Run("ServeFS", func(t *testing.T) {
		app := NewApp()
		app.ServeFS("/static", mockFS)

		req := httptest.NewRequest("GET", "/static/style.css", nil)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Result().StatusCode)
		}
		if w.Body.String() != "body { color: red; }" {
			t.Errorf("Expected body 'body { color: red; }', got '%s'", w.Body.String())
		}
	})

	t.Run("LoadTemplatesFS_And_Render", func(t *testing.T) {
		app := NewApp()
		err := app.LoadTemplatesFS(mockFS, nil)
		if err != nil {
			t.Fatalf("LoadTemplatesFS failed: %v", err)
		}

		app.Handle("GET /", func(w http.ResponseWriter, r *http.Request) {
			err := Render(w, r, "index.html", nil)
			if err != nil {
				t.Errorf("Render failed: %v", err)
			}
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Result().StatusCode)
		}
		if w.Body.String() != "<html>Index</html>" {
			t.Errorf("Expected body '<html>Index</html>', got '%s'", w.Body.String())
		}
	})
}
