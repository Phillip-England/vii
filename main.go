package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	vii "github.com/phillip-england/vii/vii"
)

// Embed everything in ./docs (including index.html).
//
//go:embed docs/*
var docs embed.FS

func main() {
	app := vii.New()
	app.Use(vii.LoggerService{})
	// Optional: nicer 404s for non-existent static paths
	app.OnNotFound = func(app *vii.App, r *http.Request, w http.ResponseWriter) {
		_ = app
		http.NotFound(w, r)
	}

	// Serve the embedded ./docs directory at "/".
	// That means:
	//   GET /            -> docs/index.html
	//   GET /whatever.js -> docs/whatever.js
	docsSub, err := fs.Sub(docs, "docs")
	if err != nil {
		log.Fatalf("fs.Sub(docs, \"docs\"): %v", err)
	}
	if err := app.ServeEmbeddedFiles("/", docsSub); err != nil {
		log.Fatalf("ServeEmbeddedFiles: %v", err)
	}

	log.Println("Serving embedded docs on http://localhost:8080 (from ./docs)")
	log.Fatal(http.ListenAndServe(":8080", app))
}
