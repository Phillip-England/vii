# vii

Minimal, ergonomic, struct-based servers in Go.

### Install

```bash
go get github.com/phillip-england/vii@latest
```

### Philosophy

**vii** is not just a router; it's a lightweight framework. It enforces a strict separation of concerns using **Structs** instead of closure functions.

1. **Routes** define endpoints.
2. **Validators** parse and sanitise input *before* the handler runs.
3. **Services** handle cross-cutting concerns (Middleware).

### Basic Server

1. Create a `main.go`:

```go
package main

import (
	"fmt"
	"net/http"
	"github.com/phillip-england/vii/vii"
)

// 1. Define your Route
type HomeRoute struct{}

// 2. Implement the Route Interface
func (HomeRoute) OnMount(app *vii.App) error { return nil }

func (HomeRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	w.Write([]byte("Hello from vii!"))
	return nil
}

func (HomeRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {
    http.Error(w, err.Error(), 500)
}

func main() {
	app := vii.New()
	
    // 3. Mount the Route
	app.Add("GET /", HomeRoute{})

	fmt.Println("Server on :8080")
	http.ListenAndServe(":8080", app)
}
```

### Directory Structure

Since `vii` generates code, it encourages a modular layout:

```text
.
├── main.go
├── go.mod
├── /routes
│   └── home_route.go
├── /services
│   └── auth_service.go
└── /validators
    └── user_validator.go
```