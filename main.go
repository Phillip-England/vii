package main

import (
	"fmt"
	"net/http"

	vii "github.com/phillip-england/vii/vii"
)

// --------------------
// Validated data
// --------------------

type User struct{ Name string }

type UserVal struct{}

func (UserVal) Validate(r *http.Request) (User, error) {
	name := r.Header.Get("X-User")
	if name == "" {
		return User{}, fmt.Errorf("missing X-User header")
	}
	return User{Name: name}, nil
}

// --------------------
// Dynamic service (parameterized)
// --------------------

type AuditService struct {
	Name   string
	Prefix string
	Key    vii.Key[AuditService] // where this instance will be stored in ctx
}

// Factory: builds a "dynamic" / configured service instance
func NewAuditService(name, prefix string) AuditService {
	return AuditService{
		Name:   name,
		Prefix: prefix,
		Key:    vii.NewKey[AuditService](name), // key per instance
	}
}

// IMPORTANT: allow multiple instances of same service type in one route
func (s AuditService) ServiceKey() string { return s.Name }

// Optional: service-owned validation
func (s AuditService) Validators() []vii.AnyValidator {
	return []vii.AnyValidator{
		vii.SV(UserVal{}),
	}
}

// A method the route will call on the service instance
func (s AuditService) Note(u User, msg string) string {
	return fmt.Sprintf("[%s] %s%s — %s", s.Name, s.Prefix, u.Name, msg)
}

func (s AuditService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	_ = w

	// Inject THIS EXACT instance into request context by key
	r = vii.WithValid(r, s.Key, s)

	u, _ := vii.Validated[User](r)
	fmt.Println(s.Note(u, "before"))
	return r, nil
}

func (s AuditService) After(r *http.Request, w http.ResponseWriter) error {
	_ = w

	u, _ := vii.Validated[User](r)
	fmt.Println(s.Note(u, "after"))
	return nil
}

// --------------------
// Route uses the services
// --------------------

type HomeRoute struct {
	// Keep the keys/instances on the route so Handle knows how to fetch them
	a AuditService
	b AuditService
}

func NewHomeRoute() HomeRoute {
	return HomeRoute{
		a: NewAuditService("audit-A", "A::"),
		b: NewAuditService("audit-B", "B::"),
	}
}

func (rt HomeRoute) Services() []vii.Service {
	// Inject two different parameterized instances of the same type
	return []vii.Service{rt.a, rt.b}
}

func (rt HomeRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	u, _ := vii.Validated[User](r)

	// Fetch each service instance back out of ctx by its key
	a, okA := vii.Valid[AuditService](r, rt.a.Key)
	b, okB := vii.Valid[AuditService](r, rt.b.Key)

	if !okA || !okB {
		return fmt.Errorf("missing audit services in context (a=%v, b=%v)", okA, okB)
	}

	// Actually use them (call their methods)
	lines := []string{
		"Hello, " + u.Name + "!",
		a.Note(u, "route is using audit-A"),
		b.Note(u, "route is using audit-B"),
	}

	w.WriteHeader(http.StatusOK)
	for _, line := range lines {
		_, _ = w.Write([]byte(line + "\n"))
	}
	return nil
}

func (HomeRoute) OnMount(app *vii.App) error { return nil }

func (HomeRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusUnauthorized)
}

// --------------------
// main
// --------------------

func main() {
	app := vii.New()

	// Mount a route INSTANCE that carries the service configs/keys
	_ = app.Mount("GET", "/", NewHomeRoute())

	fmt.Println(`Try: curl -H "X-User: Jace" http://localhost:8080`)
	_ = http.ListenAndServe(":8080", app)
}
