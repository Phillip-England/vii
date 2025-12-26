package vii_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	vii "github.com/phillip-england/vii/vii"
)

type rlRoute struct{}

func (rlRoute) OnMount(app *vii.App) error { _ = app; return nil }
func (rlRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {
	_ = r
	if err == vii.ErrRateLimited {
		http.Error(w, "limited", http.StatusTooManyRequests)
		return
	}
	http.Error(w, err.Error(), 500)
}
func (rlRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	_ = r
	w.WriteHeader(200)
	_, _ = w.Write([]byte("ok"))
	return nil
}

func TestRateLimit_AllowsBurst_ThenLimits(t *testing.T) {
	app := vii.New()
	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }
	app.Use(&vii.RateLimitService{
		MaxEntries:          100,
		Burst:              2,
		RefillEvery:         time.Second,
		Now:                clock,
		SetRetryAfterHeader: true,
	})
	if err := app.Mount(http.MethodGet, "/x", rlRoute{}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	client := http.DefaultClient
	req := func() *http.Request {
		r, _ := http.NewRequest(http.MethodGet, ts.URL+"/x", nil)
		// Note: RemoteAddr set here does NOT reach server, but this test doesn't rely on it.
		r.RemoteAddr = "1.2.3.4:1234"
		return r
	}

	{
		res, err := client.Do(req())
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
	}
	{
		res, err := client.Do(req())
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
	}
	{
		res, err := client.Do(req())
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", res.StatusCode)
		}
		if got := res.Header.Get("Retry-After"); got == "" {
			t.Fatalf("expected Retry-After header to be set")
		}
	}

	now = now.Add(time.Second)
	{
		res, err := client.Do(req())
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("expected 200 after refill, got %d", res.StatusCode)
		}
	}
}

func TestRateLimit_RespectsXForwardedFor(t *testing.T) {
	app := vii.New()
	now := time.Unix(2000, 0)
	clock := func() time.Time { return now }
	app.Use(&vii.RateLimitService{
		MaxEntries:   100,
		Burst:       1,
		RefillEvery: time.Hour,
		Now:         clock,
	})
	if err := app.Mount(http.MethodGet, "/x", rlRoute{}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	makeReq := func(xff string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, ts.URL+"/x", nil)
		r.RemoteAddr = "9.9.9.9:9999"
		r.Header.Set("X-Forwarded-For", xff)
		return r
	}

	{
		res, err := http.DefaultClient.Do(makeReq("10.0.0.1, 127.0.0.1"))
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
	}
	{
		res, err := http.DefaultClient.Do(makeReq("10.0.0.1, 127.0.0.1"))
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", res.StatusCode)
		}
	}
	{
		res, err := http.DefaultClient.Do(makeReq("10.0.0.2"))
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
	}
}

func TestRateLimit_EvictsOldestWhenMaxEntriesReached(t *testing.T) {
	app := vii.New()
	now := time.Unix(3000, 0)
	clock := func() time.Time { return now }
	app.Use(&vii.RateLimitService{
		MaxEntries:   1,
		Burst:       1,
		RefillEvery: time.Hour,
		Now:         clock,
	})
	if err := app.Mount(http.MethodGet, "/x", rlRoute{}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	// IMPORTANT: RemoteAddr is set by the server, not transmitted by the client.
	// Use X-Real-IP (supported by defaultRateLimitKey) to simulate different callers.
	doIP := func(ip string) int {
		r, _ := http.NewRequest(http.MethodGet, ts.URL+"/x", nil)
		r.Header.Set("X-Real-IP", ip)
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}

	if got := doIP("1.1.1.1"); got != 200 {
		t.Fatalf("expected 200, got %d", got)
	}
	now = now.Add(1 * time.Second)
	if got := doIP("2.2.2.2"); got != 200 {
		t.Fatalf("expected 200, got %d", got)
	}
	now = now.Add(1 * time.Second)
	if got := doIP("1.1.1.1"); got != 200 {
		t.Fatalf("expected 200 (ip1 re-added), got %d", got)
	}
}
