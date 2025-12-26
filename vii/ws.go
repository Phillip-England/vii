package vii

import (
	"net/http"
	"strings"

	"golang.org/x/net/websocket"
)

// WSMessage is placed into request context (validated store) for MESSAGE events.
type WSMessage struct {
	Data []byte
}

// isWebSocketUpgrade checks if an incoming HTTP request is attempting to upgrade.
func isWebSocketUpgrade(r *http.Request) bool {
	if r == nil {
		return false
	}
	conn := strings.ToLower(r.Header.Get("Connection"))
	upg := strings.ToLower(r.Header.Get("Upgrade"))
	return strings.Contains(conn, "upgrade") && upg == "websocket"
}

// wsWriter lets websocket routes use the same signature as HTTP routes.
// Writing sends a WS message to the client.
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

func (w *wsWriter) Header() http.Header { return w.hdr }

// WriteHeader is mostly a no-op for websockets, but we track it for completeness.
func (w *wsWriter) WriteHeader(statusCode int) { w.status = statusCode }

// Write sends a websocket message (binary frame) to the client.
// After a successful send, if a DRAIN route exists for this path, it is invoked.
func (w *wsWriter) Write(p []byte) (int, error) {
	if err := websocket.Message.Send(w.conn, p); err != nil {
		return 0, err
	}

	// Fire DRAIN as a "post-send" event if the route exists.
	if w.app != nil {
		if dr := w.app.lookup(Method.DRAIN, w.path); dr != nil {
			req := w.baseR.Clone(w.baseR.Context())
			req.Method = Method.DRAIN
			// DRAIN can optionally access the last-sent payload too
			req = WithValidated(req, WSMessage{Data: append([]byte(nil), p...)})
			_ = w.app.serveFor(Method.DRAIN, dr, w, req)
		}
	}

	return len(p), nil
}
