package vii

import (
	"fmt"
	"net/http"
	"time"
)

// LoggerService logs each request as: [METHOD][PATH][DURATION]
// Duration auto-selects µs for fast requests, ms otherwise.
type LoggerService struct {
	// Logf is optional. Defaults to fmt.Printf("%s\n", line).
	// Signature is kept simple so users can plug in log.Printf, slog, zap wrappers, etc.
	Logf func(line string)
}

type loggerStart struct {
	t time.Time
}

func (s LoggerService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	_ = w
	start := loggerStart{t: time.Now()}
	return WithValidated(r, start), nil
}

func (s LoggerService) After(r *http.Request, w http.ResponseWriter) error {
	_ = w

	st, ok := Validated[loggerStart](r)
	if !ok || st.t.IsZero() {
		// If missing (shouldn't happen), just don't log.
		return nil
	}

	method := ""
	path := ""
	if r != nil {
		method = r.Method
		if r.URL != nil {
			path = r.URL.Path
		}
	}

	d := time.Since(st.t)
	line := fmt.Sprintf("[%s][%s][%s]", method, path, formatLatency(d))

	if s.Logf != nil {
		s.Logf(line)
	} else {
		fmt.Printf("%s\n", line)
	}

	return nil
}

func formatLatency(d time.Duration) string {
	// Choose microseconds for sub-10ms, otherwise milliseconds.
	// This reads nicely in terminals while still showing quick handlers accurately.
	if d < 10*time.Millisecond {
		us := d.Microseconds()
		if us < 0 {
			us = 0
		}
		return fmt.Sprintf("%dµs", us)
	}
	ms := d.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	return fmt.Sprintf("%dms", ms)
}
