package vii

import "net/http"

// Route is a request handler with lifecycle hooks.
// Errors should be handled by the route via OnErr.
type Route interface {
	Handle(r *http.Request, w http.ResponseWriter) error
	OnMount(app *App) error
	OnErr(r *http.Request, w http.ResponseWriter, err error)
}

// Optional interfaces a Route may implement.
// If not implemented, defaults are "no validators/services".

// WithValidators provides validators that run before services and before Handle().
type WithValidators interface {
	Validators() []AnyValidator
}

// WithServices provides services that run after validators.
// Before() runs in-order, After() runs in reverse order.
type WithServices interface {
	Services() []Service
}
