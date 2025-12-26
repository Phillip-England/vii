package vii_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	vii "github.com/phillip-england/vii/vii"
)

type testValidated struct{ V string }

type okValidator struct{}

func (okValidator) Validate(r *http.Request) (testValidated, error) {
	_ = r
	return testValidated{V: "ok"}, nil
}

type failValidator struct{}

func (failValidator) Validate(r *http.Request) (testValidated, error) {
	_ = r
	return testValidated{}, errors.New("validator failed")
}

type logService struct {
	name string
	log  *[]string
}

// ServiceKey enables multiple instances of the same service type to run.
// Without this, services are de-duped by type and only one instance would execute.
func (s logService) ServiceKey() string { return s.name }

func (s logService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	_ = w
	*s.log = append(*s.log, "svc.before."+s.name)
	if _, ok := vii.Validated[testValidated](r); !ok {
		return r, errors.New("service missing validated data")
	}
	r = vii.WithValidated(r, s)
	return r, nil
}

func (s logService) After(r *http.Request, w http.ResponseWriter) error {
	_ = r
	_ = w
	*s.log = append(*s.log, "svc.after."+s.name)
	return nil
}

type failingBeforeService struct{ log *[]string }

func (s failingBeforeService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	_ = w
	*s.log = append(*s.log, "svc.before.fail")
	return r, errors.New("before failed")
}

func (s failingBeforeService) After(r *http.Request, w http.ResponseWriter) error {
	_ = r
	_ = w
	*s.log = append(*s.log, "svc.after.fail")
	return nil
}

type failAfterService struct{ log *[]string }

func (s failAfterService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	_ = w
	*s.log = append(*s.log, "svc.before.failafter")
	return r, nil
}

func (s failAfterService) After(r *http.Request, w http.ResponseWriter) error {
	_ = r
	_ = w
	*s.log = append(*s.log, "svc.after.failafter")
	return errors.New("after failed")
}

type testRoute struct {
	log            *[]string
	validators     []vii.AnyValidator
	services       []vii.Service
	handleErr      error
	handleFn       func(r *http.Request, w http.ResponseWriter) error // <-- NEW
	capturedErrMsg string
	onErrCalled    bool
	onMountCalled  bool
}

func (tr *testRoute) OnMount(app *vii.App) error {
	_ = app
	tr.onMountCalled = true
	return nil
}

func (tr *testRoute) Validators() []vii.AnyValidator { return tr.validators }
func (tr *testRoute) Services() []vii.Service        { return tr.services }

func (tr *testRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	// If a custom handler is installed, use it.
	if tr.handleFn != nil {
		return tr.handleFn(r, w)
	}

	// Default handler behavior used by most tests.
	*tr.log = append(*tr.log, "route.handle")

	if v, ok := vii.Validated[testValidated](r); ok {
		*tr.log = append(*tr.log, "handler.validated."+v.V)
	} else {
		*tr.log = append(*tr.log, "handler.validated.missing")
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
	return tr.handleErr
}

func (tr *testRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {
	_ = r
	_ = w
	tr.onErrCalled = true
	tr.capturedErrMsg = err.Error()
	*tr.log = append(*tr.log, "route.onerror."+err.Error())
}

func TestMountCallsOnMount(t *testing.T) {
	app := vii.New()
	var log []string
	rt := &testRoute{log: &log}
	if err := app.Mount(http.MethodGet, "/x", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}
	if !rt.onMountCalled {
		t.Fatalf("expected OnMount to be called")
	}
}

func TestPipeline_Success_OrderingAndValidated(t *testing.T) {
	app := vii.New()
	var log []string
	rt := &testRoute{
		log:        &log,
		validators: []vii.AnyValidator{vii.WrapValidator[testValidated](okValidator{})},
		services: []vii.Service{
			logService{name: "a", log: &log},
			logService{name: "b", log: &log},
		},
	}

	if err := app.Mount(http.MethodGet, "/ok", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	want := []string{
		"svc.before.a",
		"svc.before.b",
		"route.handle",
		"handler.validated.ok",
		"svc.after.b",
		"svc.after.a",
	}
	if !equalSlices(log, want) {
		t.Fatalf("log mismatch\n got: %#v\nwant: %#v", log, want)
	}
	if rt.onErrCalled {
		t.Fatalf("did not expect OnErr on success")
	}
}

func TestPipeline_ValidatorError_StopsBeforeServicesAndHandle(t *testing.T) {
	app := vii.New()
	var log []string
	rt := &testRoute{
		log:        &log,
		validators: []vii.AnyValidator{vii.WrapValidator[testValidated](failValidator{})},
		services: []vii.Service{
			logService{name: "a", log: &log},
		},
	}

	if err := app.Mount(http.MethodGet, "/bad", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/bad", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if !rt.onErrCalled {
		t.Fatalf("expected route.OnErr to be called")
	}
	if rt.capturedErrMsg != "validator failed" {
		t.Fatalf("unexpected error msg: %q", rt.capturedErrMsg)
	}
	if len(log) != 1 || log[0] != "route.onerror.validator failed" {
		t.Fatalf("unexpected log: %#v", log)
	}
}

func TestPipeline_ServiceBeforeError_StopsBeforeHandle(t *testing.T) {
	app := vii.New()
	var log []string
	rt := &testRoute{
		log:        &log,
		validators: []vii.AnyValidator{vii.WrapValidator[testValidated](okValidator{})},
		services: []vii.Service{
			logService{name: "a", log: &log},
			failingBeforeService{log: &log},
			logService{name: "b", log: &log},
		},
	}

	if err := app.Mount(http.MethodGet, "/svc", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/svc", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if !rt.onErrCalled {
		t.Fatalf("expected route.OnErr")
	}
	if rt.capturedErrMsg != "before failed" {
		t.Fatalf("unexpected error msg: %q", rt.capturedErrMsg)
	}

	want := []string{
		"svc.before.a",
		"svc.before.fail",
		"route.onerror.before failed",
	}
	if !equalSlices(log, want) {
		t.Fatalf("log mismatch\n got: %#v\nwant: %#v", log, want)
	}
}

func TestPipeline_HandleError_CallsOnErr_NoAfter(t *testing.T) {
	app := vii.New()
	var log []string
	rt := &testRoute{
		log:        &log,
		validators: []vii.AnyValidator{vii.WrapValidator[testValidated](okValidator{})},
		services: []vii.Service{
			logService{name: "a", log: &log},
			logService{name: "b", log: &log},
		},
		handleErr: errors.New("handle failed"),
	}

	if err := app.Mount(http.MethodGet, "/herr", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/herr", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if !rt.onErrCalled {
		t.Fatalf("expected route.OnErr")
	}
	if rt.capturedErrMsg != "handle failed" {
		t.Fatalf("unexpected error msg: %q", rt.capturedErrMsg)
	}

	want := []string{
		"svc.before.a",
		"svc.before.b",
		"route.handle",
		"handler.validated.ok",
		"route.onerror.handle failed",
	}
	if !equalSlices(log, want) {
		t.Fatalf("log mismatch\n got: %#v\nwant: %#v", log, want)
	}
}

func TestPipeline_AfterError_CallsOnErr(t *testing.T) {
	app := vii.New()
	var log []string
	rt := &testRoute{
		log:        &log,
		validators: []vii.AnyValidator{vii.WrapValidator[testValidated](okValidator{})},
		services: []vii.Service{
			logService{name: "a", log: &log},
			failAfterService{log: &log},
			logService{name: "b", log: &log},
		},
	}

	if err := app.Mount(http.MethodGet, "/aerr", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/aerr", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if !rt.onErrCalled {
		t.Fatalf("expected route.OnErr")
	}
	if rt.capturedErrMsg != "after failed" {
		t.Fatalf("unexpected error msg: %q", rt.capturedErrMsg)
	}

	want := []string{
		"svc.before.a",
		"svc.before.failafter",
		"svc.before.b",
		"route.handle",
		"handler.validated.ok",
		"svc.after.b",
		"svc.after.failafter",
		"route.onerror.after failed",
	}
	if !equalSlices(log, want) {
		t.Fatalf("log mismatch\n got: %#v\nwant: %#v", log, want)
	}
}

func TestNotFound_UsesOnNotFound(t *testing.T) {
	app := vii.New()
	called := false
	app.OnNotFound = func(app *vii.App, r *http.Request, w http.ResponseWriter) {
		_ = app
		_ = r
		called = true
		w.WriteHeader(418)
		_, _ = w.Write([]byte("nope"))
	}

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if !called {
		t.Fatalf("expected OnNotFound to be called")
	}
	if rec.Code != 418 {
		t.Fatalf("expected 418, got %d", rec.Code)
	}
}

func TestGlobalOnErr_IsCalledAfterRouteOnErr(t *testing.T) {
	app := vii.New()
	var log []string
	rt := &testRoute{
		log:        &log,
		validators: []vii.AnyValidator{vii.WrapValidator[testValidated](failValidator{})},
	}

	app.OnErr = func(app *vii.App, route vii.Route, r *http.Request, w http.ResponseWriter, err error) {
		_ = app
		_ = route
		_ = r
		_ = w
		log = append(log, "app.onerror."+err.Error())
	}

	if err := app.Mount(http.MethodGet, "/gerr", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/gerr", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	want := []string{
		"route.onerror.validator failed",
		"app.onerror.validator failed",
	}
	if !equalSlices(log, want) {
		t.Fatalf("log mismatch\n got: %#v\nwant: %#v", log, want)
	}
}

// ---- NEW: service-owned validation ----

type OwnedData struct{ Got string } // <-- IMPORTANT: shared named type

type ownedValidatorService struct {
	log *[]string
}

func (s ownedValidatorService) Validators() []vii.AnyValidator {
	return []vii.AnyValidator{
		vii.WrapValidator[testValidated](okValidator{}), // service owns this requirement
	}
}

func (s ownedValidatorService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	_ = w
	*s.log = append(*s.log, "svc.before.owned")

	v, ok := vii.Validated[testValidated](r)
	if !ok {
		return r, errors.New("owned service missing validated data")
	}

	// expose derived/service data to route
	r = vii.WithValidated(r, OwnedData{Got: v.V})
	return r, nil
}

func (s ownedValidatorService) After(r *http.Request, w http.ResponseWriter) error {
	_ = r
	_ = w
	*s.log = append(*s.log, "svc.after.owned")
	return nil
}

func TestService_OwnsItsValidation_AndExposesDataToRoute(t *testing.T) {
	app := vii.New()
	var log []string

	rt := &testRoute{
		log:      &log,
		services: []vii.Service{ownedValidatorService{log: &log}},
	}

	// Route does NOT define validators at all.
	rt.validators = nil

	// Route handle checks service-provided data
	rt.handleFn = func(r *http.Request, w http.ResponseWriter) error {
		*rt.log = append(*rt.log, "route.handle")
		if d, ok := vii.Validated[OwnedData](r); ok {
			*rt.log = append(*rt.log, "handler.owned."+d.Got)
		} else {
			*rt.log = append(*rt.log, "handler.owned.missing")
		}
		w.WriteHeader(200)
		return nil
	}

	if err := app.Mount(http.MethodGet, "/owned", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/owned", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	// NOTE: this will only pass once the framework runs service validators before service.Before.
	want := []string{
		"svc.before.owned",
		"route.handle",
		"handler.owned.ok",
		"svc.after.owned",
	}
	if !equalSlices(log, want) {
		t.Fatalf("log mismatch\n got: %#v\nwant: %#v", log, want)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
