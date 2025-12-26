package vii

import (
	"net/http"
	"reflect"
)

type App struct {
	routes map[string]map[string]Route // method -> path -> route

	OnErr      func(app *App, route Route, r *http.Request, w http.ResponseWriter, err error)
	OnNotFound func(app *App, r *http.Request, w http.ResponseWriter)
}

func New() *App {
	return &App{
		routes: make(map[string]map[string]Route),
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

type serviceNode struct {
	svc        Service
	validators []AnyValidator
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := a.lookup(r.Method, r.URL.Path)
	if route == nil {
		if a.OnNotFound != nil {
			a.OnNotFound(a, r, w)
			return
		}
		http.NotFound(w, r)
		return
	}

	// 1) Route-level validators (for endpoint-only data)
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
				return
			}
		}
	}

	// 2) Resolve service dependency graph + run service validators + Before
	var nodes []serviceNode
	if rs, ok := route.(WithServices); ok {
		nodes = resolveServices(rs.Services())

		for i := range nodes {
			n := nodes[i]

			// 2a) Service validators (service owns its required inputs)
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
					return
				}
			}

			// 2b) Service.Before (may inject service data back into context)
			var err error
			r, err = n.svc.Before(r, w)
			if err != nil {
				route.OnErr(r, w, err)
				if a.OnErr != nil {
					a.OnErr(a, route, r, w, err)
				}
				return
			}
		}
	}

	// 3) Route handler
	if err := route.Handle(r, w); err != nil {
		route.OnErr(r, w, err)
		if a.OnErr != nil {
			a.OnErr(a, route, r, w, err)
		}
		return
	}

	// 4) After hooks (reverse order)
	for i := len(nodes) - 1; i >= 0; i-- {
		if err := nodes[i].svc.After(r, w); err != nil {
			route.OnErr(r, w, err)
			if a.OnErr != nil {
				a.OnErr(a, route, r, w, err)
			}
			return
		}
	}
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

// resolveServices flattens a service dependency graph into execution order:
// dependencies first, then the service itself.
// It also attaches each service's own validators (if it implements WithValidators).
//
// NOTE: this dedupes by concrete type to prevent accidental double-running the same
// service type through multiple dependency paths. If you want multiple instances
// of the same type, wrap them in distinct types.
func resolveServices(roots []Service) []serviceNode {
	var out []serviceNode

	typeKey := func(s Service) reflect.Type {
		if s == nil {
			return nil
		}
		return reflect.TypeOf(s)
	}

	visiting := map[reflect.Type]bool{}
	visited := map[reflect.Type]bool{}

	var visit func(s Service)
	visit = func(s Service) {
		if s == nil {
			return
		}
		t := typeKey(s)
		if visited[t] {
			return
		}
		if visiting[t] {
			// cycle: ignore re-entry to avoid infinite recursion
			// (you can later replace with a hard error if you prefer)
			return
		}

		visiting[t] = true

		// service -> its dependent services first
		if ws, ok := any(s).(WithServices); ok {
			for _, dep := range ws.Services() {
				visit(dep)
			}
		}

		// then the service itself
		var vals []AnyValidator
		if wv, ok := any(s).(WithValidators); ok {
			vals = wv.Validators()
		}
		out = append(out, serviceNode{svc: s, validators: vals})

		visiting[t] = false
		visited[t] = true
	}

	for _, s := range roots {
		visit(s)
	}
	return out
}
