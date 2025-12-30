package vii

import (
	"context"
	"net/http"
)

type ContextKey string

func SetContext(key string, val any, r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKey(key), val)
	r = r.WithContext(ctx)
	return r
}

func GetContext(key string, r *http.Request) any {
	ctxMap, ok := r.Context().Value(ContextKey("GLOBAL")).(map[string]any)
	if ok {
		mapVal := ctxMap[key]
		if mapVal != nil {
			return mapVal
		}
	}
	val := r.Context().Value(ContextKey(key))
	return val
}
