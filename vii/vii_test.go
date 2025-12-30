package vii

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVii(t *testing.T) {

	app := NewApp()

	err := app.LoadTemplates("../templates", nil)
	if err != nil {
		panic(err)
	}

	app.Use(Logger)

	app.ServeDir("/static", "../static")
	app.Favicon()

	app.Handle("GET /", func(w http.ResponseWriter, r *http.Request) {
		Render(w, r, "index.html", nil)
	}, Logger, Timeout(10))

	apiGroup := app.Group("/api")
	apiGroup.Use(Logger)

	apiGroup.Handle("GET /", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, 200, map[string]interface{}{
			"message": "Hello, World!",
		})
	}, Logger, Timeout(10))

	// app.Serve("8080")

}

func TestRateLimiter(t *testing.T) {
	// Configure a rate limiter with a small limit for testing
	config := RateLimiterConfig{
		Limit:  2,
		Window: 1 * time.Minute,
	}

	app := NewApp()
	app.Use(RateLimiter(config))

	app.Handle("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(app)
	defer server.Close()

	client := server.Client()

	// First two requests should succeed
	for i := 0; i < config.Limit; i++ {
		req, err := http.NewRequest("GET", server.URL+"/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		// In a test, RemoteAddr is not set, so we can't directly test IP-based limiting
		// in the same way. The middleware will fall back to an empty IP, effectively
		// rate limiting all requests as if they were from one user. This is sufficient
		// to test the limiting logic itself.

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("Expected status OK on request %d, got %v", i+1, res.Status)
		}
	}

	// The next request should be rate limited
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected status Too Many Requests (429), got %v", res.Status)
	}
}

func TestRequestHelpers(t *testing.T) {
	t.Run("ReadJSON_Valid", func(t *testing.T) {
		type input struct {
			Name string `json:"name"`
		}
		var i input
		jsonStr := `{"name": "vii"}`
		req := httptest.NewRequest("POST", "/", bytes.NewBufferString(jsonStr))

		err := ReadJSON(req, &i)
		if err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
		if i.Name != "vii" {
			t.Errorf("Expected name to be 'vii', got '%s'", i.Name)
		}
	})

	t.Run("ReadJSON_Malformed", func(t *testing.T) {
		var i interface{}
		jsonStr := `{"name": "vii",}` // Extra comma
		req := httptest.NewRequest("POST", "/", bytes.NewBufferString(jsonStr))

		err := ReadJSON(req, &i)
		if err == nil {
			t.Fatal("Expected error for malformed JSON, got nil")
		}
		if !strings.Contains(err.Error(), "badly-formed") {
			t.Errorf("Expected error message to contain 'badly-formed', got '%v'", err)
		}
	})

	t.Run("ReadJSON_EmptyBody", func(t *testing.T) {
		var i interface{}
		req := httptest.NewRequest("POST", "/", nil) // No body

		err := ReadJSON(req, &i)
		if err == nil {
			t.Fatal("Expected error for empty body, got nil")
		}
		if !strings.Contains(err.Error(), "body must not be empty") {
			t.Errorf("Expected error message to contain 'body must not be empty', got '%v'", err)
		}
	})

	t.Run("ReadJSON_WrongType", func(t *testing.T) {
		type input struct {
			Age int `json:"age"`
		}
		var i input
		jsonStr := `{"age": "twenty"}` // String instead of int
		req := httptest.NewRequest("POST", "/", bytes.NewBufferString(jsonStr))

		err := ReadJSON(req, &i)
		if err == nil {
			t.Fatal("Expected error for wrong type, got nil")
		}
		if !strings.Contains(err.Error(), "incorrect JSON type") {
			t.Errorf("Expected error message to contain 'incorrect JSON type', got '%v'", err)
		}
	})
}

func TestURLPrimitive(t *testing.T) {
	searchURL := NewURL("/search").WithQuery("q", "category")

	t.Run("Build_URL", func(t *testing.T) {
		testCases := []struct {
			name     string
			values   Values
			expected string
		}{
			{
				name:     "With all params",
				values:   Values{"q": "golang", "category": "tech"},
				expected: "/search?category=tech&q=golang", // Order can vary, but net/url is deterministic
			},
			{
				name:     "With one param",
				values:   Values{"q": "golang"},
				expected: "/search?q=golang",
			},
			{
				name:     "With encoding needed",
				values:   Values{"q": "Go language"},
				expected: "/search?q=Go+language",
			},
			{
				name:     "With no params",
				values:   Values{},
				expected: "/search",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				builtURL := searchURL.Build(tc.values)
				if builtURL != tc.expected {
					t.Errorf("Expected URL '%s', got '%s'", tc.expected, builtURL)
				}
			})
		}
	})

	t.Run("Parse_URL", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/search?category=books&q=Hitchhiker%27s+Guide", nil)
		params := searchURL.Parse(req)

		if params["q"] != "Hitchhiker's Guide" {
			t.Errorf("Expected q to be 'Hitchhiker's Guide', got '%s'", params["q"])
		}
		if params["category"] != "books" {
			t.Errorf("Expected category to be 'books', got '%s'", params["category"])
		}
	})

	t.Run("Parse_URL_Missing_Param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/search?q=hello", nil)
		params := searchURL.Parse(req)

		if params["q"] != "hello" {
			t.Errorf("Expected q to be 'hello', got '%s'", params["q"])
		}
		if params["category"] != "" {
			t.Errorf("Expected category to be empty, got '%s'", params["category"])
		}
	})
}
