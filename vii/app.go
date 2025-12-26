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
	mux      map[string]*http.ServeMux
	static   []staticMount
	embedded map[string]fs.FS

	services []Service // NEW: global services

	OnErr      func(app *App, route Route, r *http.Request, w http.ResponseWriter, err error)
	OnNotFound func(app *App, r *http.Request, w http.ResponseWriter)
}

func New() *App {
	return &App{
		mux:      make(map[string]*http.ServeMux),
		static:   nil,
		embedded: make(map[string]fs.FS),
		services: nil,
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

func (a *App) MountPattern(pattern string, route Route) error {
	method, path, err := splitPattern(pattern)
	if err != nil {
		return err
	}
	return a.Mount(method, path, route)
}

func (a *App) Mount(method, path string, route Route) error {
	if a.mux == nil {
		a.mux = make(map[string]*http.ServeMux)
	}
	if a.embedded == nil {
		a.embedded = make(map[string]fs.FS)
	}
	if path == "" {
		return fmt.Errorf("vii: mount path is empty")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	m := a.getMux(method)
	mh := &mountedHandler{
		app:   a,
		route: route,
	}
	m.Handle(path, mh)
	if err := route.OnMount(a); err != nil {
		return err
	}
	mh.pipe = compilePipeline(a, route) // includes global services now
	return nil
}

func (a *App) getMux(method string) *http.ServeMux {
	if a.mux == nil {
		a.mux = make(map[string]*http.ServeMux)
	}
	m := a.mux[method]
	if m == nil {
		m = http.NewServeMux()
		a.mux[method] = m
	}
	return m
}

type mountedHandler struct {
	app   *App
	route Route
	pipe  *compiledPipeline
}

type serviceNode struct {
	svc        Service
	validators []AnyValidator
}

func (h *mountedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a := h.app
	if a == nil || h.route == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet && isWebSocketUpgrade(r) && a.hasAnyWSMatch(r) {
		a.serveWebSocket(w, r)
		return
	}
	if h.pipe != nil {
		_ = h.pipe.serve(w, r)
		return
	}
	_ = a.serveFor(h.route, w, r)
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a != nil && r != nil && r.Method == http.MethodGet && isWebSocketUpgrade(r) && a.hasAnyWSMatch(r) {
		a.serveWebSocket(w, r)
		return
	}
	if a != nil && a.mux != nil {
		if m := a.mux[r.Method]; m != nil {
			h, pat := m.Handler(r)
			if pat != "" {
				h.ServeHTTP(w, r)
				return
			}
		}
	}
	if a.tryStatic(w, r) {
		return
	}
	if a.OnNotFound != nil {
		a.OnNotFound(a, r, w)
		return
	}
	http.NotFound(w, r)
}

func (a *App) ServeEmbeddedFiles(prefix string, f fs.FS) error {
	if prefix == "" {
		return fmt.Errorf("vii: static prefix is empty")
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
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

func (a *App) embeddedDir(key string) (fs.FS, bool) {
	if a == nil || a.embedded == nil {
		return nil, false
	}
	f, ok := a.embedded[key]
	return f, ok
}

func (a *App) serveFor(route Route, w http.ResponseWriter, r *http.Request) error {
	r = withApp(r, a)

	// Route validators
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

	// Global + route services (NEW)
	var roots []Service
	if a != nil && len(a.services) > 0 {
		roots = append(roots, a.services...)
	}
	if rs, ok := route.(WithServices); ok {
		roots = append(roots, rs.Services()...)
	}

	var nodes []serviceNode
	if len(roots) > 0 {
		nodes = resolveServices(roots)
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

func (a *App) dispatchWS(phase string, w http.ResponseWriter, r *http.Request) {
	if a.mux != nil {
		if m := a.mux[phase]; m != nil {
			_, pat := m.Handler(r)
			if pat != "" {
				m.ServeHTTP(w, r)
				return
			}
		}
	}
	if phase == Method.OPEN || phase == Method.MESSAGE {
		if m := a.mux[http.MethodGet]; m != nil {
			_, pat := m.Handler(r)
			if pat != "" {
				m.ServeHTTP(w, r)
				return
			}
		}
	}
}

func (a *App) hasAnyWSMatch(r *http.Request) bool {
	if a == nil || a.mux == nil || r == nil {
		return false
	}
	for _, phase := range []string{Method.OPEN, Method.MESSAGE, Method.DRAIN, Method.CLOSE} {
		if m := a.mux[phase]; m != nil {
			_, pat := m.Handler(r)
			if pat != "" {
				return true
			}
		}
	}
	if m := a.mux[http.MethodGet]; m != nil {
		_, pat := m.Handler(r)
		return pat != ""
	}
	return false
}

func (a *App) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	server := websocket.Server{
		Handler: func(conn *websocket.Conn) {
			base := r.Clone(r.Context())
			base = withApp(base, a)
			base = WithValidated(base, WSConn{Conn: conn})
			writer := newWSWriter(a, conn, base)

			{
				req := base.Clone(base.Context())
				req.Method = Method.OPEN
				a.dispatchWS(Method.OPEN, writer, req)
			}

			var closeErr error
			for {
				var msg []byte
				if err := websocket.Message.Receive(conn, &msg); err != nil {
					closeErr = err
					break
				}
				req := base.Clone(base.Context())
				req.Method = Method.MESSAGE
				req = WithValidated(req, WSMessage{Data: msg})
				a.dispatchWS(Method.MESSAGE, writer, req)
			}

			{
				req := base.Clone(base.Context())
				req.Method = Method.CLOSE
				req = WithValidated(req, WSClose{Err: closeErr})
				a.dispatchWS(Method.CLOSE, writer, req)
			}
		},
	}
	server.ServeHTTP(w, r)
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

func splitPattern(pat string) (method string, path string, err error) {
	pat = strings.TrimSpace(pat)
	i := strings.IndexByte(pat, ' ')
	if i <= 0 || i == len(pat)-1 {
		return "", "", fmt.Errorf("vii: invalid pattern %q (want: \"METHOD /path\")", pat)
	}
	method = strings.TrimSpace(pat[:i])
	path = strings.TrimSpace(pat[i+1:])
	if method == "" || path == "" {
		return "", "", fmt.Errorf("vii: invalid pattern %q (want: \"METHOD /path\")", pat)
	}
	return method, path, nil
}
