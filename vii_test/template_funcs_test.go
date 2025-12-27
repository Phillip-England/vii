package vii_test

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	vii "github.com/phillip-england/vii/vii"
)

func TestTemplateFuncsCommon_StringHelpers(t *testing.T) {
	tpl := template.New("t").Funcs(vii.TemplateFuncsCommon())
	// Use explicit ASCII spaces for robustness
	tpl, err := tpl.Parse(`{{ lower "HeLLo" }}|{{ upper "HeLLo" }}|{{ trim "  x  " }}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, nil); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if got := buf.String(); got != "hello|HELLO|x" {
		t.Fatalf("got %q", got)
	}
}

func TestTemplateFuncsCommon_Printf(t *testing.T) {
	tpl := template.New("t").Funcs(vii.TemplateFuncsCommon())
	tpl, err := tpl.Parse(`{{ printf "%s-%d" "x" 7 }}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, nil); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if got := buf.String(); got != "x-7" {
		t.Fatalf("got %q", got)
	}
}

func TestTemplateFuncsCommon_DictGetDefault(t *testing.T) {
	tpl := template.New("t").Funcs(vii.TemplateFuncsCommon())
	// Broken down:
	// dict creates a map. 123 is invalid key (skipped), "" is invalid key (skipped).
	// default "fallback" "   " -> "   " is empty after trim, so returns fallback.
	tpl, err := tpl.Parse(`
{{ $m := dict "Title" "Home" 123 "nope" "" "skip" "Count" 3 "Odd" }}
{{ get $m "Title" }}|{{ get $m "Count" }}|{{ default "fallback" (get $m "Missing") }}|{{ default "fallback" "" }}|{{ default "fallback" "   " }}|{{ default "fallback" false }}|{{ default "fallback" "ok" }}
`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, nil); err != nil {
		t.Fatalf("exec: %v", err)
	}

	// Normalize output by removing newlines created by the template block
	gotRaw := buf.String()
	got := strings.Join(strings.Fields(gotRaw), "")
	
	want := "Home|3|fallback|fallback|fallback|fallback|ok"
	if got != want {
		t.Errorf("mismatch:\n got:  %q\n want: %q\nraw buffer: %q", got, want, gotRaw)
	}
}

func TestTemplateFuncsCommon_FormatTime(t *testing.T) {
	tm := time.Date(2025, 12, 27, 10, 11, 12, 0, time.UTC)
	tpl := template.New("t").Funcs(vii.TemplateFuncsCommon())
	tpl, _ = tpl.Parse(`{{ formatTime . "" }}`)
	var buf bytes.Buffer
	_ = tpl.Execute(&buf, tm)
	if got := buf.String(); got != tm.Format(time.RFC3339) {
		t.Fatalf("got %q", got)
	}
}

func TestTemplateFuncsCommon_JSON(t *testing.T) {
	type X struct {
		A string `json:"a"`
		B int    `json:"b"`
	}
	in := X{A: "hi", B: 2}
	tpl := template.New("t").Funcs(vii.TemplateFuncsCommon())
	tpl, _ = tpl.Parse(`{{ json . }}`)
	var buf bytes.Buffer
	_ = tpl.Execute(&buf, in)
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("json err: %v", err)
	}
	
	if out["a"] != "hi" {
		t.Fatalf("expected a=hi, got %v", out["a"])
	}

	// json.Unmarshal unmarshals numbers to float64 by default.
	// We handle both int (if optimization happened) or float64.
	switch v := out["b"].(type) {
	case float64:
		if v != 2 {
			t.Fatalf("expected b=2, got %v", v)
		}
	case int:
		if v != 2 {
			t.Fatalf("expected b=2, got %v", v)
		}
	default:
		t.Fatalf("expected b to be number, got %T", v)
	}
}

func TestTemplateFuncsCommon_SafeHTML(t *testing.T) {
	tpl := template.New("t").Funcs(vii.TemplateFuncsCommon())
	tpl, _ = tpl.Parse(`{{ "<b>x</b>" }}|{{ safeHTML "<b>x</b>" }}`)
	var buf bytes.Buffer
	_ = tpl.Execute(&buf, nil)
	// First one is escaped by html/template, second one is not.
	if got := buf.String(); got != "&lt;b&gt;x&lt;/b&gt;|<b>x</b>" {
		t.Fatalf("got %q", got)
	}
}

func TestTemplateRenderer_Execute_ViewShape(t *testing.T) {
	app := vii.New()
	fsys := fstest.MapFS{
		"base.html":  &fstest.MapFile{Data: []byte(`{{define "base"}}R={{if .Request}}1{{end}};V={{.Vars.X}};D={{.Data}}{{end}}`)},
		"merge.html": &fstest.MapFile{Data: []byte(`{{define "merge"}}A={{.A}};V={{.Vars.K}}{{end}}`)},
	}
	if err := app.RegisterTemplates("views", fsys, vii.TemplateFuncsCommon(), "base.html", "merge.html"); err != nil {
		t.Fatalf("register: %v", err)
	}
	tr, _ := app.Templates("views")
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	_ = tr.Execute(rec, req, "base", "hello", map[string]any{"X": "vv"})
	if rec.Body.String() != "R=1;V=vv;D=hello" {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
}

func TestTemplateRenderer_Execute_Errors(t *testing.T) {
	tr := vii.TemplateRenderer{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	if err := tr.Execute(rec, req, "x", nil, nil); err != vii.ErrTemplateNotFound {
		t.Fatalf("expected ErrTemplateNotFound")
	}
}

type capRoute struct {
	got **http.Request
}

func (capRoute) OnMount(app *vii.App) error { return nil }
func (capRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {}
func (rt capRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	if rt.got != nil {
		*rt.got = r
	}
	_, _ = io.WriteString(w, "ok")
	return nil
}

func callWithApp(app *vii.App) *http.Request {
	var captured *http.Request
	_ = app.Mount(http.MethodGet, "/__cap__", capRoute{got: &captured})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/__cap__", nil)
	app.ServeHTTP(rec, req)
	if captured == nil {
		return req
	}
	return captured
}

func TestRender_Helper_UsesRequestAppTemplates(t *testing.T) {
	app := vii.New()
	fsys := fstest.MapFS{
		"t.html": &fstest.MapFile{Data: []byte(`{{define "t"}}ok{{end}}`)},
	}
	if err := app.RegisterTemplates("views", fsys, nil, "t.html"); err != nil {
		t.Fatalf("register: %v", err)
	}
	
	// Helper to get a request that is "valid" within the app's context
	req := callWithApp(app)
	rec := httptest.NewRecorder()
	
	if err := vii.Render(req, rec, "views", "t", nil, nil); err != nil {
		t.Fatalf("render: %v", err)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
}