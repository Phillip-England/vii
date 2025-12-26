package vii

import (
	"net/http"
	"strings"
)

type staticMount struct {
	prefix  string
	handler http.Handler
}

func (a *App) tryStatic(w http.ResponseWriter, r *http.Request) bool {
	if a == nil || len(a.static) == 0 || r == nil {
		return false
	}

	path := r.URL.Path

	// Prefer the longest matching prefix (more specific mount wins).
	bestIdx := -1
	bestLen := -1

	for i := range a.static {
		m := a.static[i]
		p := m.prefix
		if p == "" || m.handler == nil {
			continue
		}

		// Match "/static" OR "/static/..."
		if path == p || strings.HasPrefix(path, p+"/") {
			if len(p) > bestLen {
				bestLen = len(p)
				bestIdx = i
			}
		}
	}

	if bestIdx < 0 {
		return false
	}

	a.static[bestIdx].handler.ServeHTTP(w, r)
	return true
}
