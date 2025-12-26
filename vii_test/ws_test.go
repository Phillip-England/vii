package vii_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	vii "github.com/phillip-england/vii/vii"
	"golang.org/x/net/websocket"
)

type wsUser struct{ Name string }

type wsUserVal struct{}

func (wsUserVal) Validate(r *http.Request) (wsUser, error) {
	u := r.Header.Get("X-User")
	if u == "" {
		return wsUser{}, errors.New("missing X-User")
	}
	return wsUser{Name: u}, nil
}

type wsLogService struct {
	mu  *sync.Mutex
	log *[]string
}

func (wsLogService) Validators() []vii.AnyValidator {
	return []vii.AnyValidator{vii.SV(wsUserVal{})}
}

func (s wsLogService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	_ = w
	u, _ := vii.Validated[wsUser](r)

	s.mu.Lock()
	*s.log = append(*s.log, "svc.before."+r.Method+"."+u.Name)
	s.mu.Unlock()

	return r, nil
}

func (s wsLogService) After(r *http.Request, w http.ResponseWriter) error {
	_ = w
	u, _ := vii.Validated[wsUser](r)

	s.mu.Lock()
	*s.log = append(*s.log, "svc.after."+r.Method+"."+u.Name)
	s.mu.Unlock()

	return nil
}

type wsRoute struct {
	mu       *sync.Mutex
	log      *[]string
	name     string
	services []vii.Service
}

func (rt wsRoute) Services() []vii.Service                              { return rt.services }
func (wsRoute) OnMount(app *vii.App) error                              { return nil }
func (wsRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {}

func (rt wsRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	rt.mu.Lock()
	*rt.log = append(*rt.log, "route."+rt.name+"."+r.Method)
	rt.mu.Unlock()

	switch r.Method {
	case vii.Method.MESSAGE:
		msg, _ := vii.Validated[vii.WSMessage](r)
		_, _ = w.Write([]byte("echo:" + string(msg.Data)))
	case vii.Method.OPEN:
		_, _ = w.Write([]byte("opened"))
	case vii.Method.DRAIN:
		msg, _ := vii.Validated[vii.WSMessage](r)
		rt.mu.Lock()
		*rt.log = append(*rt.log, "drain.payload."+string(msg.Data))
		rt.mu.Unlock()
	case vii.Method.CLOSE:
		// nothing required
	}
	return nil
}

func waitForLogEntry(t *testing.T, mu *sync.Mutex, log *[]string, want string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		found := false
		for _, s := range *log {
			if s == want {
				found = true
				break
			}
		}
		mu.Unlock()

		if found {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	t.Fatalf("timed out waiting for log entry %q\nfull log: %#v", want, *log)
}

func TestWebSocketMethods_RunServicesValidatorsAndHandlers(t *testing.T) {
	app := vii.New()

	var (
		mu  sync.Mutex
		log []string
	)

	svc := wsLogService{mu: &mu, log: &log}

	if err := app.Mount(vii.Method.OPEN, "/ws", wsRoute{mu: &mu, log: &log, name: "open", services: []vii.Service{svc}}); err != nil {
		t.Fatalf("mount open: %v", err)
	}
	if err := app.Mount(vii.Method.MESSAGE, "/ws", wsRoute{mu: &mu, log: &log, name: "message", services: []vii.Service{svc}}); err != nil {
		t.Fatalf("mount message: %v", err)
	}
	if err := app.Mount(vii.Method.DRAIN, "/ws", wsRoute{mu: &mu, log: &log, name: "drain", services: []vii.Service{svc}}); err != nil {
		t.Fatalf("mount drain: %v", err)
	}
	if err := app.Mount(vii.Method.CLOSE, "/ws", wsRoute{mu: &mu, log: &log, name: "close", services: []vii.Service{svc}}); err != nil {
		t.Fatalf("mount close: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	wsURL := "ws" + ts.URL[len("http"):] + "/ws"
	cfg, err := websocket.NewConfig(wsURL, ts.URL)
	if err != nil {
		t.Fatalf("new config: %v", err)
	}

	cfg.Header = http.Header{}
	cfg.Header.Set("X-User", "Jace")

	conn, err := websocket.DialConfig(cfg)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// OPEN should send "opened"
	var opened string
	if err := websocket.Message.Receive(conn, &opened); err != nil {
		t.Fatalf("recv opened: %v", err)
	}

	// MESSAGE should echo
	if err := websocket.Message.Send(conn, []byte("hi")); err != nil {
		t.Fatalf("send: %v", err)
	}

	var got string
	if err := websocket.Message.Receive(conn, &got); err != nil {
		t.Fatalf("recv: %v", err)
	}
	if got != "echo:hi" {
		t.Fatalf("expected echo:hi, got %q", got)
	}

	// Trigger server loop to break and run CLOSE handler.
	_ = conn.Close()

	// IMPORTANT: wait for server goroutine to observe the close + run CLOSE route.
	waitForLogEntry(t, &mu, &log, "svc.before.CLOSE.Jace", 300*time.Millisecond)

	wantContains := []string{
		"svc.before.OPEN.Jace",
		"route.open.OPEN",
		"svc.before.DRAIN.Jace",
		"route.drain.DRAIN",
		"drain.payload.opened",
		"svc.after.DRAIN.Jace",
		"svc.after.OPEN.Jace",

		"svc.before.MESSAGE.Jace",
		"route.message.MESSAGE",
		"svc.before.DRAIN.Jace",
		"route.drain.DRAIN",
		"drain.payload.echo:hi",
		"svc.after.DRAIN.Jace",
		"svc.after.MESSAGE.Jace",

		"svc.before.CLOSE.Jace",
		"route.close.CLOSE",
		"svc.after.CLOSE.Jace",
	}

	// Verify order (subsequence match)
	mu.Lock()
	defer mu.Unlock()

	idx := 0
	for _, w := range wantContains {
		found := false
		for idx < len(log) {
			if log[idx] == w {
				found = true
				idx++
				break
			}
			idx++
		}
		if !found {
			t.Fatalf("missing or out of order log entry %q\nfull log: %#v", w, log)
		}
	}
}
