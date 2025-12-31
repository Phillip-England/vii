package vii

import (
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//=====================================
// templating
//=====================================

// LoadTemplates loads templates from disk (Legacy)
func (app *App) LoadTemplates(path string, funcMap template.FuncMap) error {
	strEquals := func(input string, value string) bool {
		return input == value
	}
	vbfFuncMap := template.FuncMap{
		"strEquals": strEquals,
	}
	for k, v := range funcMap {
		vbfFuncMap[k] = v
	}
	templates := template.New("").Funcs(vbfFuncMap)
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".html" {
			_, err := templates.ParseFiles(path)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	app.SetContext(VII_CONTEXT, templates)
	return nil
}

// LoadTemplatesFS loads templates from an embedded filesystem.
// The fileSystem parameter is the embedded FS (e.g., templateFS from a //go:embed directive).
// It will parse all *.html files in the filesystem.
func (app *App) LoadTemplatesFS(fileSystem fs.FS, funcMap template.FuncMap) error {
	strEquals := func(input string, value string) bool {
		return input == value
	}
	vbfFuncMap := template.FuncMap{
		"strEquals": strEquals,
	}
	for k, v := range funcMap {
		vbfFuncMap[k] = v
	}

	templates := template.New("").Funcs(vbfFuncMap)

	var paths []string
	err := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".html") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(paths) > 0 {
		_, err = templates.ParseFS(fileSystem, paths...)
		if err != nil {
			return err
		}
	}

	app.SetContext(VII_CONTEXT, templates)
	return nil
}

func getTemplates(r *http.Request) *template.Template {
	templates, _ := GetContext(VII_CONTEXT, r).(*template.Template)
	return templates
}

func Render(w http.ResponseWriter, r *http.Request, filepath string, data any) error {
	w.Header().Add("Content-Type", "text/html")
	templates := getTemplates(r)
	err := templates.ExecuteTemplate(w, filepath, data)
	if err != nil {
		return err
	}
	return nil
}
