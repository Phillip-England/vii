package vii_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	vii "github.com/phillip-england/vii/vii"
	"golang.org/x/net/websocket"
)

type wsAppCtxRoute struct{}

func (wsAppCtxRoute) Services() []vii.Service                          { return nil }
func (wsAppCtxRoute) OnMount(app *vii.App) error                      { return nil }
func (wsAppCtxRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) { _ = r; _ = w; _ = err }

func (wsAppCtxRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	if _, ok := vii.AppFrom(r); !ok {
		http.Error(w, "missing app ctx", 500)
		return nil
	}
	_, _ = w.Write([]byte("ok"))
	return nil
}

func TestWebSocket_AppFrom_IsAvailableInHandlers(t *testing.T) {
	app := vii.New()

	// Just OPEN is enough to validate ws base context injection.
	if err := app.Mount(vii.Method.OPEN, "/wsctx", wsAppCtxRoute{}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	wsURL := "ws" + ts.URL[len("http"):] + "/wsctx"
	cfg, err := websocket.NewConfig(wsURL, ts.URL)
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	cfg.Header = http.Header{}

	conn, err := websocket.DialConfig(cfg)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	var got string
	if err := websocket.Message.Receive(conn, &got); err != nil {
		t.Fatalf("recv: %v", err)
	}
	if got != "ok" {
		t.Fatalf("expected ok, got %q", got)
	}
}
