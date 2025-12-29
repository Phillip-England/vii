package vii

import "net/http"

func Provide[T any](r *http.Request, value T) *http.Request {
	return Set(r, value)
}

func ProvideKey[T any](r *http.Request, k Key[T], value T) *http.Request {
	r = Set(r, value) // by type
	r = SetKey(r, k, value)  // by key
	return r
}

func ProvideOnlyKey[T any](r *http.Request, k Key[T], value T) *http.Request {
	return SetKey(r, k, value)
}
