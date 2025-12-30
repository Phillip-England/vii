# vii

A minimal, expressive, and fast web framework for Go.

vii provides a simple yet powerful API to build web applications and services, getting out of your way so you can focus on your logic.

## Features

-   **Simple Routing:** Expressive and straightforward route definition.
-   **Route Groups:** Organize your routes and apply middleware to entire groups.
-   **Middleware Support:** Comes with built-in middleware for logging, timeouts, CORS, and rate limiting. Easily add your own.
-   **Flexible Template Rendering:** Load HTML templates from the filesystem at runtime or embed them into your binary for single-file deployment.
-   **Flexible Static File Serving:** Serve static assets from a directory or from a compile-time embedded filesystem.
-   **Reusable URL Primitives:** Define URL patterns once and reuse them for type-safe URL generation and parameter parsing.
-   **Helper Functions:** A rich set of helpers for handling requests (JSON, headers, cookies) and writing responses (JSON, errors, redirects).

## Installation

The library code is in the `pkg` directory. To use it, import it in your Go files:

```go
import "github.com/Phillip-England/vii/vii"
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
	"fmt"
	"net/http"
	"time"

	"github.com/Phillip-England/vii/vii"
)

// (Optional) Embed static files and templates for single-binary deployment.
//go:embed static/*
var embeddedStaticFS embed.FS

//go:embed templates/*.html
var embeddedTemplateFS embed.FS

// Define a reusable URL primitive
var greetURL = vii.NewURL("/greet").WithQuery("name")

func main() {
	// --- 1. Initialization ---
	app := vii.NewApp()

	// --- 2. Global Middleware ---
	// Apply middleware to all routes.
	app.Use(vii.Logger, vii.CORS, vii.RateLimiter(vii.RateLimiterConfig{Limit: 100, Window: time.Minute}))

	// --- 3. Templates ---
	// Choose one method for loading templates:

	// Method A: Load from the filesystem (good for development)
	// if err := app.LoadTemplates("templates", nil); err != nil {
	// 	panic(err)
	// }

	// Method B: Load from embedded filesystem (good for production)
	if err := app.LoadTemplatesFS(embeddedTemplateFS, nil); err != nil {
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
		if err := vii.Render(w, r, "index.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// --- 6. Route Grouping ---
	// Group routes under a common prefix and apply group-specific middleware.
	apiGroup := app.Group("/api")
	apiGroup.Use(vii.Timeout(10)) // Apply a 10-second timeout to all /api routes.

	apiGroup.Handle("GET /greet", func(w http.ResponseWriter, r *http.Request) {
		params := greetURL.Parse(r)
		name := params["name"]
		if name == "" {
			name = "World"
		}
		
		// Example of building a URL with the primitive
		exampleURL := greetURL.Build(vii.Values{"name": "Alice"})
		fmt.Println("Example generated URL:", exampleURL)

		vii.WriteJSON(w, http.StatusOK, map[string]string{"message": "Hello, " + name})
	})
	
	// --- 7. Start Server ---
	// Start the application server on port 8080.
	if err := app.Serve("8080"); err != nil {
		panic(err)
	}
}
```

## API Overview

### Core App

-   `vii.NewApp() *App`: Creates a new vii application instance.
-   `app.Use(middleware ...)`: Applies one or more global middleware to all routes.
-   `app.Handle(pattern string, handler http.HandlerFunc, ...)`: Registers a handler for a specific method and path pattern (e.g., `"GET /"`).
-   `app.Serve(port string) error`: Starts the HTTP server.

### Routing and Groups

-   `app.Group(prefix string) *Group`: Creates a new route group with a URL prefix.
-   `group.Use(middleware ...)`: Applies middleware to all routes within the group.
-   `group.Handle(...)`: Registers a handler within the group.

### Templates

-   `app.LoadTemplates(path string, ...) error`: Loads and parses HTML templates from a directory on the filesystem.
-   `app.LoadTemplatesFS(fs fs.FS, ...) error`: Loads and parses HTML templates from an embedded filesystem (`embed.FS`).
-   `vii.Render(w, r, templateName string, data any) error`: Renders a previously loaded template by its filename.

### Static Files

-   `app.ServeDir(urlPrefix string, dirPath string, ...)`: Serves static files from a directory on the filesystem.
-   `app.ServeFS(urlPrefix string, fs fs.FS, ...)`: Serves static files from an embedded filesystem.
-   `app.Favicon(...)`: Registers a handler to serve a `favicon.ico` file from the project root.

### Request Helpers

-   `vii.ReadJSON(r *http.Request, v interface{}) error`: Decodes a JSON request body into a struct or map.
-   `vii.Header(r *http.Request, key string) string`: Gets a request header value.
-   `vii.Cookie(r *http.Request, name string) (*http.Cookie, error)`: Retrieves a specific cookie.
-   `vii.Query(r *http.Request, name string) string`: Gets a URL query parameter's value.

### Response Writers

-   `vii.WriteJSON(w, statusCode int, data any) error`: Writes a JSON response.
-   `vii.WriteHTML(w, statusCode int, htmlContent string)`: Writes a raw HTML string response.
-   `vii.WriteText(w, statusCode int, textContent string)`: Writes a plain text response.
-   `vii.WriteError(w, statusCode int, message string) error`: Writes a consistent JSON error response.
-   `vii.Redirect(w, r, url string, code int)`: Performs an HTTP redirect.
-   `vii.SetHeader(w, key, value string)`: Sets a response header.
-   `vii.SetCookie(w, cookie *http.Cookie)`: Sets a response cookie.

### Middleware

-   `vii.Logger`: A request logger that prints the method, path, and request duration.
-   `vii.Timeout(seconds int)`: A middleware that applies a timeout to requests.
-   `vii.CORS`: A permissive Cross-Origin Resource Sharing (CORS) middleware.
-   `vii.RateLimiter(config RateLimiterConfig)`: An in-memory, IP-based rate-limiting middleware.

### URL Primitive

-   `vii.NewURL(path string) *URL`: Creates a new URL definition.
-   `url.WithQuery(params ...string) *URL`: Adds expected query parameters to the definition.
-   `url.Build(values Values) string`: Builds a URL string with query parameters.
-   `url.Parse(r *http.Request) Values`: Extracts defined query parameters from a request.
