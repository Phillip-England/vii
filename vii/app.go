package vii

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"reflect"
	"strings"

	"golang.org/x/net/websocket"
)

type App struct {
	routes map[string]map[string]Route // method -> path -> route

	// static mounts (prefix-based) for local/embedded file serving
	static []staticMount

	// embedded dirs registry for non-static assets (templates, docs, etc.)
	embedded map[string]fs.FS

	OnErr      func(app *App, route Route, r *http.Request, w http.ResponseWriter, err error)
	OnNotFound func(app *App, r *http.Request, w http.ResponseWriter)
}

func New() *App {
	return &App{
		routes:   make(map[string]map[string]Route),
		static:   nil,
		embedded: make(map[string]fs.FS),
		OnErr: func(app *App, route Route, r *http.Request, w http.ResponseWriter, err error) {
			_ = app
			_ = route
			_ = r
			_ = w
			_ = err
		},
		OnNotFound: func(app *App, r *http.Request, w http.ResponseWriter) {
			_ = app
			http.NotFound(w, r)
		},
	}
}

func (a *App) Mount(method, path string, route Route) error {
	if a.routes == nil {
		a.routes = make(map[string]map[string]Route)
	}
	if _, ok := a.routes[method]; !ok {
		a.routes[method] = make(map[string]Route)
	}
	a.routes[method][path] = route
	return route.OnMount(a)
}

// ServeEmbeddedFiles mounts an fs.FS at a URL prefix (e.g. "/static").
// This is used for "static handling" (served directly to the client).
func (a *App) ServeEmbeddedFiles(prefix string, f fs.FS) error {
	if prefix == "" {
		return fmt.Errorf("vii: static prefix is empty")
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	// normalize: "/static/" -> "/static"
	if len(prefix) > 1 && strings.HasSuffix(prefix, "/") {
		prefix = strings.TrimSuffix(prefix, "/")
	}
	if f == nil {
		return fmt.Errorf("vii: embedded fs is nil")
	}

	h := http.StripPrefix(prefix, http.FileServer(http.FS(f)))
	a.static = append(a.static, staticMount{
		prefix:  prefix,
		handler: h,
	})
	return nil
}

// ServeLocalFiles mounts a directory on disk at a URL prefix (e.g. "/static").
// This is dynamic (reads from disk at request time).
func (a *App) ServeLocalFiles(prefix string, dir string) error {
	if dir == "" {
		return fmt.Errorf("vii: local static dir is empty")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("vii: stat local dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vii: local static path is not a directory: %s", dir)
	}
	return a.ServeEmbeddedFiles(prefix, os.DirFS(dir))
}

// EmbedDir registers an fs.FS under a key, intended for request-time access
// (templates, docs, private assets) rather than direct static serving.
func (a *App) EmbedDir(key string, f fs.FS) error {
	if key == "" {
		return fmt.Errorf("vii: embed key is empty")
	}
	if f == nil {
		return fmt.Errorf("vii: embed fs is nil")
	}
	if a.embedded == nil {
		a.embedded = make(map[string]fs.FS)
	}
	a.embedded[key] = f
	return nil
}

// embeddedDir returns a registered embedded fs by key.
func (a *App) embeddedDir(key string) (fs.FS, bool) {
	if a == nil || a.embedded == nil {
		return nil, false
	}
	f, ok := a.embedded[key]
	return f, ok
}

type serviceNode struct {
	svc        Service
	validators []AnyValidator
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isWebSocketUpgrade(r) {
		a.serveWebSocket(w, r)
		return
	}

	route := a.lookup(r.Method, r.URL.Path)
	if route == nil {
		// If no route matches, try static mounts (prefix match).
		if a.tryStatic(w, r) {
			return
		}
		if a.OnNotFound != nil {
			a.OnNotFound(a, r, w)
			return
		}
		http.NotFound(w, r)
		return
	}

	_ = a.serveFor(r.Method, route, w, r)
}

func (a *App) serveFor(method string, route Route, w http.ResponseWriter, r *http.Request) error {
	_ = method

	// Make the App available during the request lifecycle.
	r = withApp(r, a)

	if rv, ok := route.(WithValidators); ok {
		for _, v := range rv.Validators() {
			if v == nil {
				continue
			}
			var err error
			r, err = v.ValidateAny(r)
			if err != nil {
				route.OnErr(r, w, err)
				if a.OnErr != nil {
					a.OnErr(a, route, r, w, err)
				}
				return err
			}
		}
	}

	var nodes []serviceNode
	if rs, ok := route.(WithServices); ok {
		nodes = resolveServices(rs.Services())
		for i := range nodes {
			n := nodes[i]

			for _, v := range n.validators {
				if v == nil {
					continue
				}
				var err error
				r, err = v.ValidateAny(r)
				if err != nil {
					route.OnErr(r, w, err)
					if a.OnErr != nil {
						a.OnErr(a, route, r, w, err)
					}
					return err
				}
			}

			var err error
			r, err = n.svc.Before(r, w)
			if err != nil {
				route.OnErr(r, w, err)
				if a.OnErr != nil {
					a.OnErr(a, route, r, w, err)
				}
				return err
			}
		}
	}

	if err := route.Handle(r, w); err != nil {
		route.OnErr(r, w, err)
		if a.OnErr != nil {
			a.OnErr(a, route, r, w, err)
		}
		return err
	}

	for i := len(nodes) - 1; i >= 0; i-- {
		if err := nodes[i].svc.After(r, w); err != nil {
			route.OnErr(r, w, err)
			if a.OnErr != nil {
				a.OnErr(a, route, r, w, err)
			}
			return err
		}
	}

	return nil
}

func (a *App) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if a.lookup(Method.OPEN, path) == nil &&
		a.lookup(Method.MESSAGE, path) == nil &&
		a.lookup(Method.DRAIN, path) == nil &&
		a.lookup(Method.CLOSE, path) == nil {
		if a.OnNotFound != nil {
			a.OnNotFound(a, r, w)
			return
		}
		http.NotFound(w, r)
		return
	}

	server := websocket.Server{
		Handler: func(conn *websocket.Conn) {
			base := r.Clone(r.Context())
			base = withApp(base, a)
			base = WithValidated(base, WSConn{Conn: conn})

			writer := newWSWriter(a, conn, base)

			if openRoute := a.lookup(Method.OPEN, path); openRoute != nil {
				req := base.Clone(base.Context())
				req.Method = Method.OPEN
				_ = a.serveFor(Method.OPEN, openRoute, writer, req)
			}

			var closeErr error
			for {
				var msg []byte
				if err := websocket.Message.Receive(conn, &msg); err != nil {
					closeErr = err
					break
				}
				if msgRoute := a.lookup(Method.MESSAGE, path); msgRoute != nil {
					req := base.Clone(base.Context())
					req.Method = Method.MESSAGE
					req = WithValidated(req, WSMessage{Data: msg})
					_ = a.serveFor(Method.MESSAGE, msgRoute, writer, req)
				}
			}

			if closeRoute := a.lookup(Method.CLOSE, path); closeRoute != nil {
				req := base.Clone(base.Context())
				req.Method = Method.CLOSE
				req = WithValidated(req, WSClose{Err: closeErr})
				_ = a.serveFor(Method.CLOSE, closeRoute, writer, req)
			}
		},
	}
	server.ServeHTTP(w, r)
}

func (a *App) lookup(method, path string) Route {
	if a.routes == nil {
		return nil
	}
	pm := a.routes[method]
	if pm == nil {
		return nil
	}
	return pm[path]
}

func resolveServices(roots []Service) []serviceNode {
	var out []serviceNode

	serviceID := func(s Service) string {
		if s == nil {
			return ""
		}
		t := reflect.TypeOf(s)
		id := t.String()
		if sk, ok := any(s).(ServiceKeyer); ok {
			k := sk.ServiceKey()
			if k != "" {
				id = id + "|" + k
			} else {
				id = id + "|"
			}
		}
		return id
	}

	visiting := map[string]bool{}
	visited := map[string]bool{}

	var visit func(s Service)
	visit = func(s Service) {
		if s == nil {
			return
		}
		id := serviceID(s)
		if visited[id] {
			return
		}
		if visiting[id] {
			panic(fmt.Sprintf("vii: cyclic service dependency detected at %s", id))
		}
		visiting[id] = true

		if ws, ok := any(s).(WithServices); ok {
			for _, dep := range ws.Services() {
				visit(dep)
			}
		}

		var vals []AnyValidator
		if wv, ok := any(s).(WithValidators); ok {
			vals = wv.Validators()
		}

		out = append(out, serviceNode{svc: s, validators: vals})
		visiting[id] = false
		visited[id] = true
	}

	for _, s := range roots {
		visit(s)
	}

	return out
}
