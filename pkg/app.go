package vii

import (
	"fmt"
	"net/http"
)

const VII_CONTEXT = "VII_CONTEXT"

type App struct {
	Mux              *http.ServeMux
	GlobalContext    map[string]any
	GlobalMiddleware []func(http.Handler) http.Handler
}

func NewApp() App {
	mux := http.NewServeMux()
	app := App{
		Mux:              mux,
		GlobalContext:    make(map[string]any),
		GlobalMiddleware: []func(http.Handler) http.Handler{},
	}
	return app
}

func (app *App) Use(middleware ...func(http.Handler) http.Handler) {
	app.GlobalMiddleware = append(app.GlobalMiddleware, middleware...)
}

func (app *App) SetContext(key string, value any) {
	app.GlobalContext[key] = value
}

func (app *App) Handle(path string, handler http.HandlerFunc, middleware ...func(http.Handler) http.Handler) {
	app.Mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		r = SetContext("GLOBAL", app.GlobalContext, r)
		// Only apply Local middleware here
		chain(handler, middleware...).ServeHTTP(w, r)
	})
}

func (app *App) Serve(port string) error {
	fmt.Println("starting server on port " + port + " 🚀")

	finalHandler := chain(app.Mux.ServeHTTP, app.GlobalMiddleware...)

	err := http.ListenAndServe(":"+port, finalHandler)
	if err != nil {
		return err
	}
	return nil
}
