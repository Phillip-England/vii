package vii

import "net/http"

func Provide[T any](r *http.Request, value T) *http.Request {
	return WithValidated(r, value)
}

func ProvideKey[T any](r *http.Request, k Key[T], value T) *http.Request {
	r = WithValidated(r, value) // by type
	r = WithValid(r, k, value)  // by key
	return r
}

// ProvideOnlyKey stores value ONLY by key (does NOT write into the "by type" slot).
// This is ideal when you want multiple instances of the same type in a single request.
func ProvideOnlyKey[T any](r *http.Request, k Key[T], value T) *http.Request {
	return WithValid(r, k, value)
}
