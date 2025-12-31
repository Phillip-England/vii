package vii

import (
	"net/http"
	"strings"
)

type Group struct {
	parent     *App
	prefix     string
	middleware []func(http.Handler) http.Handler
}

func (app *App) Group(prefix string) *Group {
	return &Group{
		parent:     app,
		prefix:     strings.TrimRight(prefix, "/"),
		middleware: []func(http.Handler) http.Handler{},
	}
}

func (g *Group) Use(middleware ...func(http.Handler) http.Handler) {
	g.middleware = append(g.middleware, middleware...)
}

func (g *Group) Handle(path string, handler http.HandlerFunc, middleware ...func(http.Handler) http.Handler) {
	resolvedPath := g.prefix + strings.TrimRight(strings.Split(path, " ")[1], "/")
	method := strings.Split(path, " ")[0]
	// Only apply Group + Local middleware here
	allMiddleware := append(g.middleware, middleware...)
	finalHandler := Chain(handler, allMiddleware...)
	g.parent.Mux.HandleFunc(method+" "+resolvedPath, func(w http.ResponseWriter, r *http.Request) {
		r = SetContext("GLOBAL", g.parent.GlobalContext, r)
		finalHandler.ServeHTTP(w, r)
	})
}
