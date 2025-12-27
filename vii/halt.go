package vii

import "errors"

// ErrHalt is a sentinel used to stop the pipeline early without treating it as an error.
// Services may return ErrHalt after writing a response (e.g. automatic CORS preflight).
var ErrHalt = errors.New("vii: halt")
