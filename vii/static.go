package vii

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

func (app *App) Favicon(middleware ...func(http.Handler) http.Handler) {
	app.Mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		Chain(func(w http.ResponseWriter, r *http.Request) {
			filePath := "favicon.ico"
			fullPath := filepath.Join(".", ".", filePath)
			http.ServeFile(w, r, fullPath)
		}, middleware...).ServeHTTP(w, r)
	})
}

// ServeDir serves files from disk at a specified URL prefix.
func (app *App) ServeDir(urlPrefix string, dirPath string, middleware ...func(http.Handler) http.Handler) {
	// Ensure the URL prefix is clean
	urlPrefix = "/" + strings.Trim(urlPrefix, "/") + "/"

	fileServer := http.FileServer(http.Dir(dirPath))
	stripHandler := http.StripPrefix(urlPrefix, fileServer)
	var handler http.Handler = stripHandler
	if len(middleware) > 0 {
		handler = Chain(stripHandler.ServeHTTP, middleware...)
	}
	app.Mux.Handle("GET "+urlPrefix, handler)
}

// ServeFS serves files from an embedded filesystem (NEW)
// urlPrefix: the URL path to serve from (e.g., "/static")
// fileSystem: the embedded FS (e.g., staticFS)
func (app *App) ServeFS(urlPrefix string, fileSystem fs.FS, middleware ...func(http.Handler) http.Handler) {
	// Ensure the prefix is clean
	urlPrefix = "/" + strings.Trim(urlPrefix, "/") + "/"

	// Convert fs.FS to http.FileSystem
	fileServer := http.FileServer(http.FS(fileSystem))

	// Strip the prefix from the request URL so the file server sees the relative path
	stripHandler := http.StripPrefix(urlPrefix, fileServer)

	var handler http.Handler = stripHandler
	if len(middleware) > 0 {
		handler = Chain(stripHandler.ServeHTTP, middleware...)
	}

	app.Mux.Handle("GET "+urlPrefix, handler)
}
