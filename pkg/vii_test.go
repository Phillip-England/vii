package vii

import (
	"net/http"
	"testing"
)

func TestVii(t *testing.T) {

	app := NewApp()

	err := app.LoadTemplates("../templates", nil)
	if err != nil {
		panic(err)
	}

	app.Use(Logger)

	app.ServeDir("/static", "../static")
	app.Favicon()

	app.Handle("GET /", func(w http.ResponseWriter, r *http.Request) {
		Render(w, r, "index.html", nil)
	}, Logger, Timeout(10))

	apiGroup := app.Group("/api")
	apiGroup.Use(Logger)

	apiGroup.Handle("GET /", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, 200, map[string]interface{}{
			"message": "Hello, World!",
		})
	}, Logger, Timeout(10))

	// app.Serve("8080")

}
