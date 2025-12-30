package vii

import (
	"io/fs"
	"net/http"
)

type appCtx struct {
	App *App
}

func withApp(r *http.Request, a *App) *http.Request {
	if r == nil {
		return r
	}
	return WithValidated(r, appCtx{App: a})
}

func AppFrom(r *http.Request) (*App, bool) {
	c, ok := Validated[appCtx](r)
	if !ok || c.App == nil {
		return nil, false
	}
	return c.App, true
}

// EmbeddedDir returns an fs.FS registered by app.EmbedDir(key, fs).
func EmbeddedDir(r *http.Request, key string) (fs.FS, bool) {
	app, ok := AppFrom(r)
	if !ok {
		return nil, false
	}
	return app.embeddedDir(key)
}

// EmbeddedReadFile reads a file from an embedded dir registered via app.EmbedDir.
// Path is interpreted within the embedded fs (no leading slash).
func EmbeddedReadFile(r *http.Request, key string, path string) ([]byte, bool) {
	fsys, ok := EmbeddedDir(r, key)
	if !ok {
		return nil, false
	}
	b, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, false
	}
	return b, true
}

func Injection[T any](r *http.Request, key InjectionKey) (T, bool) {
	var zero T
	app, ok := AppFrom(r)
	if !ok {
		return zero, false
	}
	val := app.getInjection(key)
	if val == nil {
		return zero, false
	}
	tVal, ok := val.(T)
	if !ok {
		return zero, false
	}
	return tVal, true
}
