package vii

import (
	"net/http"

	"golang.org/x/net/websocket"
)

// WSConn is stored in request context for all websocket handlers (OPEN/MESSAGE/DRAIN/CLOSE).
type WSConn struct {
	Conn *websocket.Conn
}

// WSClose is stored in request context for CLOSE handlers.
// x/net/websocket does not reliably expose close codes/reasons; Err is the receive-loop
// error that ended the connection (often io.EOF).
type WSClose struct {
	Err error
}

// WS returns the websocket connection for the current handler, if present.
func WS(r *http.Request) (*websocket.Conn, bool) {
	c, ok := Validated[WSConn](r)
	if !ok || c.Conn == nil {
		return nil, false
	}
	return c.Conn, true
}

// WSMsg returns the current websocket message payload for MESSAGE/DRAIN handlers.
func WSMsg(r *http.Request) ([]byte, bool) {
	m, ok := Validated[WSMessage](r)
	if !ok {
		return nil, false
	}
	return m.Data, true
}

// WSCloseInfo returns the close info for CLOSE handlers.
func WSCloseInfo(r *http.Request) (WSClose, bool) {
	return Validated[WSClose](r)
}
