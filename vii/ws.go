package vii

import (
	"net/http"
	"strings"

	"golang.org/x/net/websocket"
)

type WSMessage struct {
	Data []byte
}

func isWebSocketUpgrade(r *http.Request) bool {
	if r == nil {
		return false
	}
	conn := strings.ToLower(r.Header.Get("Connection"))
	upg := strings.ToLower(r.Header.Get("Upgrade"))
	return strings.Contains(conn, "upgrade") && upg == "websocket"
}

type wsWriter struct {
	hdr    http.Header
	conn   *websocket.Conn
	app    *App
	baseR  *http.Request
	path   string
	status int
}

func newWSWriter(app *App, conn *websocket.Conn, baseR *http.Request) *wsWriter {
	return &wsWriter{
		hdr:   make(http.Header),
		conn:  conn,
		app:   app,
		baseR: baseR,
		path:  baseR.URL.Path,
	}
}

func (w *wsWriter) Header() http.Header         { return w.hdr }
func (w *wsWriter) WriteHeader(statusCode int) { w.status = statusCode }

func (w *wsWriter) Write(p []byte) (int, error) {
	if err := websocket.Message.Send(w.conn, p); err != nil {
		return 0, err
	}

	// Optional DRAIN hook after outbound writes.
	if w.app != nil {
		req := w.baseR.Clone(w.baseR.Context())
		req.Method = Method.DRAIN
		req = WithValidated(req, WSMessage{Data: append([]byte(nil), p...)})
		w.app.dispatchWS(Method.DRAIN, w, req)
	}

	return len(p), nil
}
