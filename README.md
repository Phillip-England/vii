# vii
Minimal servers in Go

## Installation
```bash
go get github.com/Phillip-England/vii
```

## Hello, World
1. Create a new `go` project
2. Create a `./templates` dir
3. Create a `./static` dir

Basic Server:

```go
package main

import (
	"github.com/Phillip-England/vii"
)

func main() {
	app := vii.NewApp()
	app.Use(vii.MwLogger, vii.MwTimeout(10))
	app.Static("./static")
	app.Favicon()
	err := app.Templates("./templates", nil)
	if err != nil {
		panic(err)
	}
  app.At("GET /", func(w http.ResponseWriter, r *http.Request) {
		vii.ExecuteTemplate(w, r, "index.html", nil)
	})
	app.Serve("8080")
}
```
