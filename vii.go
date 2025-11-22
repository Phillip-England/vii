package vii

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/templ"
)

type Group struct {
	parent     *App
	prefix     string
	middleware []func(http.Handler) http.Handler
}

func (app *App) Group(prefix string) *Group {
	return &Group{
		parent:     app,
		prefix:     strings.TrimRight(prefix, "/"),
		middleware: []func(http.Handler) http.Handler{},
	}
}

func (g *Group) Use(middleware ...func(http.Handler) http.Handler) {
	g.middleware = append(g.middleware, middleware...)
}

// UPDATED: Removed GlobalMiddleware injection here (moved to Serve)
func (g *Group) At(path string, handler http.HandlerFunc, middleware ...func(http.Handler) http.Handler) {
	resolvedPath := g.prefix + strings.TrimRight(strings.Split(path, " ")[1], "/")
	method := strings.Split(path, " ")[0] 
	// Only apply Group + Local middleware here
	allMiddleware := append(g.middleware, middleware...) 
	g.parent.Mux.HandleFunc(method+" "+resolvedPath, func(w http.ResponseWriter, r *http.Request) {
		r = SetContext("GLOBAL", g.parent.GlobalContext, r)
		chain(handler, allMiddleware...).ServeHTTP(w, r)
	})
}

//=====================================
// app
//=====================================

const VII_CONTEXT = "VII_CONTEXT"

type App struct {
	Mux              *http.ServeMux
	GlobalContext    map[string]any
	GlobalMiddleware []func(http.Handler) http.Handler
}

func NewApp() App {
	mux := http.NewServeMux()
	app := App{
		Mux:              mux,
		GlobalContext:    make(map[string]any),
		GlobalMiddleware: []func(http.Handler) http.Handler{},
	}
	return app
}

func (app *App) Use(middleware ...func(http.Handler) http.Handler) {
	app.GlobalMiddleware = append(app.GlobalMiddleware, middleware...)
}

func (app *App) SetContext(key string, value any) {
	app.GlobalContext[key] = value
}

// UPDATED: Removed GlobalMiddleware injection here (moved to Serve)
func (app *App) At(path string, handler http.HandlerFunc, middleware ...func(http.Handler) http.Handler) {
	app.Mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		r = SetContext("GLOBAL", app.GlobalContext, r)
		// Only apply Local middleware here
		chain(handler, middleware...).ServeHTTP(w, r)
	})
}

func (app *App) Templates(path string, funcMap template.FuncMap) error {
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

func (app *App) Favicon(middleware ...func(http.Handler) http.Handler) {
	app.Mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		chain(func(w http.ResponseWriter, r *http.Request) {
			filePath := "favicon.ico"
			fullPath := filepath.Join(".", ".", filePath)
			http.ServeFile(w, r, fullPath)
		}, middleware...).ServeHTTP(w, r)
	})
}

func (app *App) Static(path string, middleware ...func(http.Handler) http.Handler) {
	staticPath := strings.TrimRight(path, "/")
	fileServer := http.FileServer(http.Dir(staticPath))
	stripHandler := http.StripPrefix("/"+filepath.Base(staticPath)+"/", fileServer)
	var handler http.Handler = stripHandler
	if len(middleware) > 0 {
		handler = chain(stripHandler.ServeHTTP, middleware...)
	}
	app.Mux.Handle("GET /"+filepath.Base(staticPath)+"/", handler)
}

func (app *App) JSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = w.Write(jsonData)
	return err
}

func (app *App) HTML(w http.ResponseWriter, status int, html string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write([]byte(html))
	return err
}

func (app *App) Text(w http.ResponseWriter, status int, text string) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write([]byte(text))
	return err
}

// UPDATED: This is the Critical Fix
// We wrap the entire Mux in the Global Middleware here.
// This ensures Middleware runs BEFORE the router checks if the path exists.
func (app *App) Serve(port string) error {
	fmt.Println("starting server on port " + port + " 🚀")
	
	// Wrap the Mux (Router) with the global middleware
	// Note: chain applies middleware in order, but because they wrap each other,
	// the LAST added middleware becomes the outermost shell and runs FIRST.
	// Since you added MwCORS last in main.go, it will run first, which is perfect.
	finalHandler := chain(app.Mux.ServeHTTP, app.GlobalMiddleware...)
	
	err := http.ListenAndServe(":"+port, finalHandler)
	if err != nil {
		return err
	}
	return nil
}

//=====================================
// middleware
//=====================================

func chain(h http.HandlerFunc, middleware ...func(http.Handler) http.Handler) http.Handler {
	finalHandler := http.Handler(h)
	for _, m := range middleware {
		finalHandler = m(finalHandler)
	}
	return finalHandler
}

func MwTimeout(seconds int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			done := make(chan bool)
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(seconds)*time.Second)
			defer cancel()
			r = r.WithContext(ctx)
			go func() {
				next.ServeHTTP(w, r)
				select {
				case <-ctx.Done():
					return
				case done <- true:
				}
			}()
			select {
			case <-done:
				return
			case <-ctx.Done():
				http.Error(w, "Request timed out", http.StatusGatewayTimeout)
				return
			}
		})
	}
}

func MwLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		next.ServeHTTP(w, r)
		endTime := time.Since(startTime)
		fmt.Printf("[%s][%s][%s]\n", r.Method, r.URL.Path, endTime)
	})
}

func MwCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

//=====================================
// context
//=====================================

type ContextKey string

func SetContext(key string, val any, r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKey(key), val)
	r = r.WithContext(ctx)
	return r
}

func GetContext(key string, r *http.Request) any {
	ctxMap, ok := r.Context().Value(ContextKey("GLOBAL")).(map[string]any)
	if ok {
		mapVal := ctxMap[key]
		if mapVal != nil {
			return mapVal
		}
	}
	val := r.Context().Value(ContextKey(key))
	return val
}

//=====================================
// templating
//=====================================

func getTemplates(r *http.Request) *template.Template {
	templates, _ := GetContext(VII_CONTEXT, r).(*template.Template)
	return templates
}

func ExecuteTemplate(w http.ResponseWriter, r *http.Request, filepath string, data any) error {
	w.Header().Add("Content-Type", "text/html")
	templates := getTemplates(r)
	err := templates.ExecuteTemplate(w, filepath, data)
	if err != nil {
		return err
	}
	return nil
}

//=====================================
// request / response helpers
//=====================================

// gets a url param
func Param(r *http.Request, paramName string) string {
	return r.URL.Query().Get(paramName)
}

// compares a provided value to a url query param
func ParamIs(r *http.Request, paramName string, valueToCheck string) bool {
	return r.URL.Query().Get(paramName) == valueToCheck
}

// responds from a handler with a string as HTML and sets status code
func WriteHTML(w http.ResponseWriter, status int, content string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	w.Write([]byte(content))
}

// responds from a handler as plain text and sets status code
func WriteString(w http.ResponseWriter, status int, content string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	w.Write([]byte(content))
}

// WriteJSON responds from a handler with JSON while setting the appropriate headers
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Encode data to JSON and write it to the response
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	return nil
}

// responds from a handler with a templ component with the appropriate headers
func WriteTempl(w http.ResponseWriter, r *http.Request, component templ.Component) error {
	w.Header().Add("Content-Type", "text/html")
	err := component.Render(r.Context(), w)
	if err != nil {
		return err
	}
	return nil
}