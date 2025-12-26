package vii

import "net/http"

// Provide stores a value in the request context by its type.
// (Just a nicer name for services/routes than WithValidated.)
func Provide[T any](r *http.Request, value T) *http.Request {
	return WithValidated(r, value)
}

// ProvideKey stores a value by its type AND by a named Key[T].
// Use this when you want multiple instances of the same type,
// or you want a stable "named" lookup for clarity.
func ProvideKey[T any](r *http.Request, k Key[T], value T) *http.Request {
	r = WithValidated(r, value) // by type
	r = WithValid(r, k, value)  // by key
	return r
}
