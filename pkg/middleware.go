package vii

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

//=====================================
// middleware
//=====================================

func chain(h http.HandlerFunc, middleware ...func(http.Handler) http.Handler) http.Handler {
	finalHandler := http.Handler(h)
	for _, m := range middleware {
		finalHandler = m(finalHandler)
	}
	return finalHandler
}

func Timeout(seconds int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			done := make(chan bool)
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(seconds)*time.Second)
			defer cancel()
			r = r.WithContext(ctx)
			go func() {
				next.ServeHTTP(w, r)
				select {
				case <-ctx.Done():
					return
				case done <- true:
				}
		}()
		select {
		case <-done:
			return
		case <-ctx.Done():
			http.Error(w, "Request timed out", http.StatusGatewayTimeout)
			return
		}
		})
	}
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		next.ServeHTTP(w, r)
		endTime := time.Since(startTime)
		fmt.Printf("[%s][%s][%s]\n", r.Method, r.URL.Path, endTime)
	})
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
