# vii

A minimal, expressive, and fast web framework for Go.

vii provides a simple yet powerful API to build web applications and services, getting out of your way so you can focus on your logic.

## Features

-   **Simple Routing:** Expressive and straightforward route definition.
-   **Route Groups:** Organize your routes and apply middleware to entire groups.
-   **Middleware Support:** Comes with built-in middleware for logging, timeouts, and CORS. Easily add your own.
-   **Flexible Template Rendering:** Load HTML templates from the filesystem at runtime or embed them into your binary for single-file deployment.
-   **Flexible Static File Serving:** Serve static assets from a directory or from a compile-time embedded filesystem.
-   **JSON Helpers:** Simple helpers for writing JSON responses.

## Installation

The library code is in the `pkg` directory. To use it, import it in your Go files:

```go
import "github.com/Phillip-England/vii/pkg"
```

## Full Usage Example

Here is a complete example demonstrating the main features of vii.

**Project Structure:**

```
/my-app
├── go.mod
├── main.go
├── static/
│   └── style.css
└── templates/
    ├── index.html
    └── layout.html
```

**`main.go`:**

```go
package main

import (
	"embed"
	"net/http"

	"github.com/Phillip-England/vii/pkg"
)

// (Optional) Embed static files and templates for single-binary deployment.
//go:embed static/*
var embeddedStaticFS embed.FS

//go:embed templates/*.html
var embeddedTemplateFS embed.FS

func main() {
	// --- 1. Initialization ---
	app := vii.NewApp()

	// --- 2. Global Middleware ---
	// Apply middleware to all routes.
	app.Use(vii.Logger, vii.CORS)

	// --- 3. Templates ---
	// Choose one method for loading templates:

	// Method A: Load from the filesystem (good for development)
	// err := app.LoadTemplates("templates", nil)
	// if err != nil {
	// 	panic(err)
	// }

	// Method B: Load from embedded filesystem (good for production)
	err := app.LoadTemplatesFS(embeddedTemplateFS, nil)
	if err != nil {
		panic(err)
	}

	// --- 4. Static Files ---
	// Choose one method for serving static files:

	// Method A: Serve from the filesystem (good for development)
	// app.ServeDir("/static", "static")

	// Method B: Serve from embedded filesystem (good for production)
	app.ServeFS("/static", embeddedStaticFS)
	
	// You can also register a handler for the favicon.
	app.Favicon()

	// --- 5. Routing ---
	// Register a handler for the root path.
	app.Handle("GET /", func(w http.ResponseWriter, r *http.Request) {
		// The Render function finds and executes a template by its filename.
		data := map[string]string{"Title": "Home Page"}
		err := vii.Render(w, r, "index.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// --- 6. Route Grouping ---
	// Group routes under a common prefix and apply group-specific middleware.
	apiGroup := app.Group("/api")
	apiGroup.Use(vii.Timeout(10)) // Apply a 10-second timeout to all /api routes.

	apiGroup.Handle("GET /greet", func(w http.ResponseWriter, r *http.Request) {
		name := vii.Param(r, "name")
		if name == "" {
			name = "World"
		}
		vii.WriteJSON(w, http.StatusOK, map[string]string{"message": "Hello, " + name})
	})
	
	// --- 7. Start Server ---
	// Start the application server on port 8080.
	err = app.Serve("8080")
	if err != nil {
		panic(err)
	}
}
```

## API Overview

### Core App

-   `vii.NewApp() App`: Creates a new vii application instance.
-   `app.Use(middleware ...)`: Applies one or more global middleware to all routes.
-   `app.Handle(pattern string, handler http.HandlerFunc, ...)`: Registers a handler for a specific method and path pattern (e.g., `"GET /"`).
-   `app.Serve(port string)`: Starts the HTTP server.

### Routing and Groups

-   `app.Group(prefix string) *Group`: Creates a new route group with a URL prefix.
-   `group.Use(middleware ...)`: Applies middleware to all routes within the group.
-   `group.Handle(...)`: Registers a handler within the group.

### Templates

-   `app.LoadTemplates(path string, ...)`: Loads and parses HTML templates from a directory on the filesystem.
-   `app.LoadTemplatesFS(fs fs.FS, ...)`: Loads and parses HTML templates from an embedded filesystem (`embed.FS`).
-   `vii.Render(w, r, templateName string, data any)`: Renders a previously loaded template by its filename.

### Static Files

-   `app.ServeDir(urlPrefix string, dirPath string, ...)`: Serves static files from a directory on the filesystem.
-   `app.ServeFS(urlPrefix string, fs fs.FS, ...)`: Serves static files from an embedded filesystem.
-   `app.Favicon(...)`: Registers a handler to serve a `favicon.ico` file from the project root.

### Response Writers

-   `vii.WriteJSON(w, statusCode int, data any)`: Serializes the `data` interface to JSON and writes it as the response.
-   `vii.WriteHTML(w, statusCode int, htmlContent string)`: Writes a raw HTML string as the response.
-   `vii.WriteText(w, statusCode int, textContent string)`: Writes a plain text string as the response.

### Middleware

-   `vii.Logger`: A request logger that prints the method, path, and request duration to the console.
-   `vii.Timeout(seconds int)`: A middleware that applies a timeout to the request context.
-   `vii.CORS`: A permissive Cross-Origin Resource Sharing (CORS) middleware.