package vii

import "net/http"

type Service interface {
	Before(r *http.Request, w http.ResponseWriter) (*http.Request, error)
	After(r *http.Request, w http.ResponseWriter) error
}

// ServiceKeyer optionally allows multiple instances of the same service type
// to participate in the pipeline.
//
// Default behavior (no ServiceKey method): services are de-duped by concrete type.
// If ServiceKey is implemented: services are de-duped by (type + "|" + ServiceKey()).
//
// This enables reusing the same service type with different parameters.
type ServiceKeyer interface {
	ServiceKey() string
}
