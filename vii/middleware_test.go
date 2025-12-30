package vii

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutMiddleware(t *testing.T) {
	// Handler that sleeps longer than the timeout
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Finished"))
	})

	// Timeout set to 1 (second), which is longer than 200ms. Should pass.
	// Wait, standard timeout is in seconds? Yes, Timeout(seconds int).
	// So 1 second is plenty for 200ms.
	t.Run("Success", func(t *testing.T) {
		timeoutMw := Timeout(1)
		handler := timeoutMw(slowHandler)
		server := httptest.NewServer(handler)
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK, got %v", resp.Status)
		}
	})

	// Handler that sleeps 2 seconds, timeout 1 second. Should fail.
	t.Run("TimedOut", func(t *testing.T) {
		verySlowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		})

		timeoutMw := Timeout(1)
		handler := timeoutMw(verySlowHandler)
		server := httptest.NewServer(handler)
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		// http.TimeoutHandler returns 503 Service Unavailable by default
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("Expected status Service Unavailable (503), got %v", resp.Status)
		}
	})
}
