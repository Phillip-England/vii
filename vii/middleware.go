package vii

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

//=====================================
// middleware
//=====================================

func Chain(h http.HandlerFunc, middleware ...func(http.Handler) http.Handler) http.Handler {
	finalHandler := http.Handler(h)
	for _, m := range middleware {
		finalHandler = m(finalHandler)
	}
	return finalHandler
}

// RateLimiterConfig holds the configuration for the rate limiter.
type RateLimiterConfig struct {
	Limit      int           // Number of requests allowed per window.
	Window     time.Duration // The time window.
	MaxClients int           // Max number of unique clients to track.
}

// RateLimiter is a middleware that provides rate limiting based on IP address.
func RateLimiter(config RateLimiterConfig) func(http.Handler) http.Handler {
	// Set sensible defaults if not provided
	if config.Limit <= 0 {
		config.Limit = 20
	}
	if config.Window <= 0 {
		config.Window = 1 * time.Minute
	}
	if config.MaxClients <= 0 {
		config.MaxClients = 1000
	}

	var (
		mu       sync.Mutex
		requests = make(map[string][]time.Time)
		queue    = make([]string, 0, config.MaxClients)
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()

			// Get client IP. Note: r.RemoteAddr may not be the true client IP if behind a proxy.
			// In a production environment, you might want to check X-Forwarded-For or other headers.
			ip := strings.Split(r.RemoteAddr, ":")[0]

			// Eviction logic for new clients when the map is full
			if _, exists := requests[ip]; !exists && len(queue) >= config.MaxClients {
				evictIP := queue[0]
				queue = queue[1:]
				delete(requests, evictIP)
			}

			// If the client is new, add them to the queue
			if _, exists := requests[ip]; !exists {
				queue = append(queue, ip)
			}

			// Clean up old requests for the current IP
			now := time.Now()
			var recentRequests []time.Time
			for _, t := range requests[ip] {
				if now.Sub(t) < config.Window {
					recentRequests = append(recentRequests, t)
				}
			}
			requests[ip] = recentRequests

			// Check if the limit is exceeded
			if len(requests[ip]) >= config.Limit {
				mu.Unlock()
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			// Add the current request timestamp
			requests[ip] = append(requests[ip], now)

			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

func Timeout(seconds int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, time.Duration(seconds)*time.Second, "Request timed out")
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
