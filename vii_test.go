package vii

import (
	"net/http"
	"testing"
)

func TestVii(t *testing.T) {

	app := NewApp()

	err := app.Templates("./templates", nil)
	if err != nil {
		panic(err)
	}

	app.Use(MwLogger)

	app.Static("./static")
	app.Favicon()

	app.At("GET /", func(w http.ResponseWriter, r *http.Request) {
		ExecuteTemplate(w, r, "index.html", nil)
	}, MwLogger, MwTimeout(10))

	apiGroup := app.Group("/api")
	apiGroup.Use(MwLogger)

	apiGroup.At("GET /", func(w http.ResponseWriter, r *http.Request) {
		app.JSON(w, 200, map[string]interface{}{
			"message": "Hello, World!",
		})
	}, MwLogger, MwTimeout(10))

	// app.Serve("8080")

}
