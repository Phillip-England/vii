package vii

import "net/http"

// Methods is a convenience namespace so users can write: vii.Method.GET, vii.Method.OPEN, etc.
type Methods struct {
	GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS string

	// WebSocket lifecycle "methods"
	OPEN, MESSAGE, DRAIN, CLOSE string
}

// Method provides ergonomic access to all supported method strings.
// These WS methods are treated exactly like HTTP methods inside the router.
var Method = Methods{
	GET:     http.MethodGet,
	POST:    http.MethodPost,
	PUT:     http.MethodPut,
	PATCH:   http.MethodPatch,
	DELETE:  http.MethodDelete,
	HEAD:    http.MethodHead,
	OPTIONS: http.MethodOptions,

	OPEN:    "OPEN",
	MESSAGE: "MESSAGE",
	DRAIN:   "DRAIN",
	CLOSE:   "CLOSE",
}
