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

func ProvideOnlyKey[T any](r *http.Request, k Key[T], value T) *http.Request {
	return WithValid(r, k, value)
}
