package vii_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	vii "github.com/phillip-england/vii/vii"
)

func TestStatic_EmbeddedFiles_ServesFromPrefix(t *testing.T) {
	app := vii.New()

	efs := fstest.MapFS{
		"hello.txt": &fstest.MapFile{Data: []byte("hello-embedded")},
		"nested/a.txt": &fstest.MapFile{Data: []byte("nested-a")},
	}

	if err := app.ServeEmbeddedFiles("/static", efs); err != nil {
		t.Fatalf("ServeEmbeddedFiles: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	{
		res, err := http.Get(ts.URL + "/static/hello.txt")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer res.Body.Close()
		b, _ := readAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("expected 200, got %d body=%q", res.StatusCode, string(b))
		}
		if string(b) != "hello-embedded" {
			t.Fatalf("unexpected body: %q", string(b))
		}
	}

	{
		res, err := http.Get(ts.URL + "/static/nested/a.txt")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer res.Body.Close()
		b, _ := readAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("expected 200, got %d body=%q", res.StatusCode, string(b))
		}
		if string(b) != "nested-a" {
			t.Fatalf("unexpected body: %q", string(b))
		}
	}

	// requesting the mount root "/static" should not crash; typically 301/200 depending on FileServer behavior.
	{
		res, err := http.Get(ts.URL + "/static")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode == 404 {
			t.Fatalf("expected non-404 for /static, got %d", res.StatusCode)
		}
	}
}

func TestStatic_LocalFiles_ServesFromDisk(t *testing.T) {
	app := vii.New()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hi.txt"), []byte("hello-disk"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "x.txt"), []byte("sub-x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := app.ServeLocalFiles("/static", dir); err != nil {
		t.Fatalf("ServeLocalFiles: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/static/hi.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	b, _ := readAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d body=%q", res.StatusCode, string(b))
	}
	if string(b) != "hello-disk" {
		t.Fatalf("unexpected body: %q", string(b))
	}

	res2, err := http.Get(ts.URL + "/static/sub/x.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res2.Body.Close()
	b2, _ := readAll(res2.Body)
	if res2.StatusCode != 200 {
		t.Fatalf("expected 200, got %d body=%q", res2.StatusCode, string(b2))
	}
	if string(b2) != "sub-x" {
		t.Fatalf("unexpected body: %q", string(b2))
	}
}

func TestStatic_ExtensionlessHtml(t *testing.T) {
	app := vii.New()

	efs := fstest.MapFS{
		"about.html": &fstest.MapFile{Data: []byte("<h1>About</h1>")},
		"contact.html": &fstest.MapFile{Data: []byte("<h1>Contact</h1>")},
		"css/style.css": &fstest.MapFile{Data: []byte("body { color: red; }")},
	}

	if err := app.ServeEmbeddedFiles("/", efs); err != nil {
		t.Fatalf("ServeEmbeddedFiles: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	// 1. /about -> should serve about.html
	{
		res, err := http.Get(ts.URL + "/about")
		if err != nil {
			t.Fatalf("get /about: %v", err)
		}
		defer res.Body.Close()
		b, _ := readAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("expected 200 for /about, got %d", res.StatusCode)
		}
		if string(b) != "<h1>About</h1>" {
			t.Fatalf("expected about.html content, got %q", string(b))
		}
	}

	// 2. /contact -> should serve contact.html
	{
		res, err := http.Get(ts.URL + "/contact")
		if err != nil {
			t.Fatalf("get /contact: %v", err)
		}
		defer res.Body.Close()
		b, _ := readAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("expected 200 for /contact, got %d", res.StatusCode)
		}
		if string(b) != "<h1>Contact</h1>" {
			t.Fatalf("expected contact.html content, got %q", string(b))
		}
	}

	// 3. /css/style.css -> should still work
	{
		res, err := http.Get(ts.URL + "/css/style.css")
		if err != nil {
			t.Fatalf("get /css/style.css: %v", err)
		}
		defer res.Body.Close()
		b, _ := readAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("expected 200 for /css/style.css, got %d", res.StatusCode)
		}
		if string(b) != "body { color: red; }" {
			t.Fatalf("expected style.css content, got %q", string(b))
		}
	}
}

func TestStatic_LongestPrefixWins(t *testing.T) {
	app := vii.New()

	efs1 := fstest.MapFS{
		"hello.txt": &fstest.MapFile{Data: []byte("from-static")},
	}
	efs2 := fstest.MapFS{
		"hello.txt": &fstest.MapFile{Data: []byte("from-static-v1")},
	}

	if err := app.ServeEmbeddedFiles("/static", efs1); err != nil {
		t.Fatalf("ServeEmbeddedFiles /static: %v", err)
	}
	if err := app.ServeEmbeddedFiles("/static/v1", efs2); err != nil {
		t.Fatalf("ServeEmbeddedFiles /static/v1: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/static/v1/hello.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	b, _ := readAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d body=%q", res.StatusCode, string(b))
	}
	if string(b) != "from-static-v1" {
		t.Fatalf("expected longest-prefix mount to win, got %q", string(b))
	}
}

func TestStatic_RouteBeatsStaticWhenExactMatch(t *testing.T) {
	app := vii.New()

	efs := fstest.MapFS{
		"hello.txt": &fstest.MapFile{Data: []byte("static-body")},
	}
	if err := app.ServeEmbeddedFiles("/static", efs); err != nil {
		t.Fatalf("ServeEmbeddedFiles: %v", err)
	}

	// Exact route should win before tryStatic runs.
	rt := &simpleRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			_, _ = w.Write([]byte("route-body"))
			return nil
		},
	}
	if err := app.Mount(http.MethodGet, "/static/hello.txt", rt); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/static/hello.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	b, _ := readAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d body=%q", res.StatusCode, string(b))
	}
	if string(b) != "route-body" {
		t.Fatalf("expected route to beat static, got %q", string(b))
	}
}

func TestEmbeddedDir_RequestTimeAccess(t *testing.T) {
	app := vii.New()

	assets := fstest.MapFS{
		"docs/readme.md": &fstest.MapFile{Data: []byte("hello-doc")},
	}

	if err := app.EmbedDir("docs", assets); err != nil {
		t.Fatalf("EmbedDir: %v", err)
	}

	rt := &simpleRoute{
		handle: func(r *http.Request, w http.ResponseWriter) error {
			// Ensure app is injected into the request before Handle executes.
			if _, ok := vii.AppFrom(r); !ok {
				http.Error(w, "missing app ctx", 500)
				return nil
			}
			b, ok := vii.EmbeddedReadFile(r, "docs", "docs/readme.md")
			if !ok {
				http.Error(w, "missing embedded file", 500)
				return nil
			}
			_, _ = w.Write(b)
			return nil
		},
	}
	if err := app.Mount(http.MethodGet, "/read", rt); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	ts := httptest.NewServer(app)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/read")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	b, _ := readAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got %d body=%q", res.StatusCode, string(b))
	}
	if string(b) != "hello-doc" {
		t.Fatalf("unexpected body: %q", string(b))
	}
}

func TestServeEmbeddedFiles_Errors(t *testing.T) {
	app := vii.New()

	if err := app.ServeEmbeddedFiles("", fstest.MapFS{}); err == nil {
		t.Fatalf("expected error for empty prefix")
	}
	if err := app.ServeEmbeddedFiles("/static", (fs.FS)(nil)); err == nil {
		t.Fatalf("expected error for nil fs")
	}
}

func TestServeLocalFiles_Errors(t *testing.T) {
	app := vii.New()

	if err := app.ServeLocalFiles("/static", ""); err == nil {
		t.Fatalf("expected error for empty dir")
	}

	// Non-existent directory
	if err := app.ServeLocalFiles("/static", filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatalf("expected error for missing dir")
	}

	// Path exists but is not a dir
	p := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := app.ServeLocalFiles("/static", p); err == nil {
		t.Fatalf("expected error for non-dir path")
	}
}

type simpleRoute struct {
	handle func(r *http.Request, w http.ResponseWriter) error
}

func (simpleRoute) OnMount(app *vii.App) error { return nil }
func (simpleRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {
	_ = r
	http.Error(w, err.Error(), 500)
}
func (rt *simpleRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	if rt.handle == nil {
		w.WriteHeader(204)
		return nil
	}
	return rt.handle(r, w)
}

func readAll(rc interface{ Read([]byte) (int, error) }) ([]byte, error) {
	// tiny local helper to avoid importing io in every test block
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 512)
	for {
		n, err := rc.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			// io.EOF is an error type; compare by string is lame—keep it simple:
			if err.Error() == "EOF" {
				return buf, nil
			}
			// some readers use different EOF errors; fall back to common patterns
			if err.Error() == "unexpected EOF" {
				return buf, nil
			}
			return buf, err
		}
	}
}
