package vii

import "net/http"

// Service participates in the request lifecycle.
//
// NOTE: Before returns an updated *http.Request so a service can inject
// validated/derived data into the request context (via WithValidated/WithValid).
type Service interface {
	Before(r *http.Request, w http.ResponseWriter) (*http.Request, error)
	After(r *http.Request, w http.ResponseWriter) error
}
