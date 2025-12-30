package vii

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

var ErrTemplateNotFound = errors.New("vii: template not found")

type TemplateView struct {
	Request *http.Request
	Data    any
	Vars    map[string]any
}

func Vars(kv ...any) map[string]any {
	if len(kv) == 0 {
		return nil
	}
	out := make(map[string]any, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok || k == "" {
			continue
		}
		out[k] = kv[i+1]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeMaps(dst, src map[string]any) map[string]any {
	if dst == nil && src == nil {
		return nil
	}
	if dst == nil {
		dst = make(map[string]any, len(src))
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (a *App) TemplateLocalDir(key string, dir string, patterns ...string) error {
	return a.TemplateLocalDirWithFuncs(key, dir, nil, patterns...)
}

func (a *App) TemplateLocalDirWithFuncs(key string, dir string, funcs template.FuncMap, patterns ...string) error {
	if a == nil {
		return fmt.Errorf("vii: app is nil")
	}
	if dir == "" {
		return fmt.Errorf("vii: local template dir is empty")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("vii: stat local template dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vii: local template path is not a directory: %s", dir)
	}

	return a.RegisterTemplates(key, os.DirFS(dir), funcs, patterns...)
}

func (a *App) RegisterTemplates(key string, fsys fs.FS, funcs template.FuncMap, patterns ...string) error {
	if a == nil {
		return fmt.Errorf("vii: app is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("vii: templates key is empty")
	}
	if fsys == nil {
		return fmt.Errorf("vii: templates fs is nil")
	}
	if len(patterns) == 0 {
		return fmt.Errorf("vii: templates patterns empty")
	}

	base := template.New(key)
	if funcs != nil {
		base = base.Funcs(funcs)
	}

	tpl, err := base.ParseFS(fsys, patterns...)
	if err != nil {
		return err
	}

	a.tmplMu.Lock()
	defer a.tmplMu.Unlock()

	if a.templates == nil {
		a.templates = make(map[string]*template.Template)
	}
	a.templates[key] = tpl
	return nil
}

func (a *App) TemplateDir(key string, patterns ...string) error {
	return a.TemplateDirWithFuncs(key, nil, patterns...)
}

func (a *App) TemplateDirWithFuncs(key string, funcs template.FuncMap, patterns ...string) error {
	if a == nil {
		return fmt.Errorf("vii: app is nil")
	}
	fsys, ok := a.embeddedDir(key)
	if !ok || fsys == nil {
		return fmt.Errorf("vii: embedded dir %q not found (call EmbedDir first)", key)
	}
	return a.RegisterTemplates(key, fsys, funcs, patterns...)
}

func Templates(r *http.Request, key string) (TemplateRenderer, bool) {
	app, ok := AppFrom(r)
	if !ok || app == nil {
		return TemplateRenderer{}, false
	}
	return app.Templates(key)
}

func (a *App) Templates(key string) (TemplateRenderer, bool) {
	if a == nil {
		return TemplateRenderer{}, false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return TemplateRenderer{}, false
	}

	a.tmplMu.RLock()
	t := a.templates[key]
	a.tmplMu.RUnlock()

	if t == nil {
		return TemplateRenderer{}, false
	}
	return TemplateRenderer{key: key, tpl: t}, true
}

type TemplateRenderer struct {
	key string
	tpl *template.Template
}

func (tr TemplateRenderer) Execute(w http.ResponseWriter, r *http.Request, name string, data any, vars map[string]any) error {
	if tr.tpl == nil {
		return ErrTemplateNotFound
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("vii: template name empty")
	}
	if vars == nil {
		vars = map[string]any{}
	}

	view := map[string]any{}
	if m, ok := data.(map[string]any); ok && m != nil {
		view = mergeMaps(view, m)
		view["Data"] = m
	} else if data != nil {
		view["Data"] = data
	}
	view["Request"] = r
	view["Vars"] = vars

	if err := tr.tpl.ExecuteTemplate(w, name, view); err != nil {
		return err
	}
	return nil
}

func Render(r *http.Request, w http.ResponseWriter, key string, name string, data any, vars map[string]any) error {
	tr, ok := Templates(r, key)
	if !ok {
		return ErrTemplateNotFound
	}
	return tr.Execute(w, r, name, data, vars)
}