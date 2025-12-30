package vii_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/phillip-england/vii/vii"
)

func TestTemplateLocalDir(t *testing.T) {
	// Create a temporary directory for templates
	tmpDir, err := os.MkdirTemp("", "vii_templates")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy template file
	tplContent := `Hello {{.Data.Name}}!`
	tplName := "hello.html"
	tplPath := filepath.Join(tmpDir, tplName)
	if err := os.WriteFile(tplPath, []byte(tplContent), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	app := vii.New()

	// Load templates from the local directory
	err = app.TemplateLocalDir("my_local_tmpls", tmpDir, "*.html")
	if err != nil {
		t.Fatalf("TemplateLocalDir failed: %v", err)
	}

	// Verify the template is loaded by trying to render it
	// We can use Render helper or check app internals if exposed, but Render is the public API.
	
	// Mock request/response
	req := httptest.NewRequest("GET", "/", nil)
	// We need to associate app with request for Render to work (Render uses AppFrom(r))
	// However, vii.Render takes (r, w, key, name, data, vars).
	// Internally it calls Templates(r, key). Templates(r, key) calls AppFrom(r).
	// AppFrom(r) retrieves app from context.
	// We need a way to put app into context or use a method on app directly.
	
	// vii.App has a method Templates(key) which returns (TemplateRenderer, bool).
	tr, ok := app.Templates("my_local_tmpls")
	if !ok {
		t.Fatalf("Templates not found for key 'my_local_tmpls'")
	}

	w := httptest.NewRecorder()
	data := map[string]any{"Name": "World"}
	err = tr.Execute(w, req, "hello.html", data, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if w.Body.String() != "Hello World!" {
		t.Errorf("unexpected output: got %q, want %q", w.Body.String(), "Hello World!")
	}
}
