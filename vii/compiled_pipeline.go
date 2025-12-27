package vii

import "net/http"

type compiledPipeline struct {
	app             *App
	route           Route
	routeValidators []AnyValidator
	nodes           []serviceNode
}

func compilePipeline(app *App, route Route) *compiledPipeline {
	var rv []AnyValidator
	if rvs, ok := route.(WithValidators); ok {
		rv = rvs.Validators()
	}

	var roots []Service
	if app != nil && len(app.services) > 0 {
		roots = append(roots, app.services...)
	}
	if rs, ok := route.(WithServices); ok {
		roots = append(roots, rs.Services()...)
	}

	var nodes []serviceNode
	if len(roots) > 0 {
		nodes = resolveServices(roots)
	}

	return &compiledPipeline{
		app:             app,
		route:           route,
		routeValidators: rv,
		nodes:           nodes,
	}
}

func (p *compiledPipeline) serve(w http.ResponseWriter, r *http.Request) error {
	r = withApp(r, p.app)

	for _, v := range p.routeValidators {
		if v == nil {
			continue
		}
		var err error
		r, err = v.ValidateAny(r)
		if err != nil {
			if err == ErrHalt {
				return nil
			}
			p.route.OnErr(r, w, err)
			if p.app != nil && p.app.OnErr != nil {
				p.app.OnErr(p.app, p.route, r, w, err)
			}
			return err
		}
	}

	for i := range p.nodes {
		n := p.nodes[i]

		for _, v := range n.validators {
			if v == nil {
				continue
			}
			var err error
			r, err = v.ValidateAny(r)
			if err != nil {
				if err == ErrHalt {
					return nil
				}
				p.route.OnErr(r, w, err)
				if p.app != nil && p.app.OnErr != nil {
					p.app.OnErr(p.app, p.route, r, w, err)
				}
				return err
			}
		}

		var err error
		r, err = n.svc.Before(r, w)
		if err != nil {
			if err == ErrHalt {
				return nil
			}
			p.route.OnErr(r, w, err)
			if p.app != nil && p.app.OnErr != nil {
				p.app.OnErr(p.app, p.route, r, w, err)
			}
			return err
		}
	}

	if err := p.route.Handle(r, w); err != nil {
		if err == ErrHalt {
			return nil
		}
		p.route.OnErr(r, w, err)
		if p.app != nil && p.app.OnErr != nil {
			p.app.OnErr(p.app, p.route, r, w, err)
		}
		return err
	}

	for i := len(p.nodes) - 1; i >= 0; i-- {
		if err := p.nodes[i].svc.After(r, w); err != nil {
			if err == ErrHalt {
				return nil
			}
			p.route.OnErr(r, w, err)
			if p.app != nil && p.app.OnErr != nil {
				p.app.OnErr(p.app, p.route, r, w, err)
			}
			return err
		}
	}

	return nil
}
