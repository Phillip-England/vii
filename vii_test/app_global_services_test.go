package vii_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	vii "github.com/phillip-england/vii/vii"
)

type globalLogService struct {
	log *[]string
}

func (s globalLogService) ServiceKey() string { return "global" }

func (s globalLogService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	_ = w
	*s.log = append(*s.log, "svc.before.global")
	return r, nil
}

func (s globalLogService) After(r *http.Request, w http.ResponseWriter) error {
	_ = r
	_ = w
	*s.log = append(*s.log, "svc.after.global")
	return nil
}

func TestPipeline_GlobalServices_WrapRouteServices(t *testing.T) {
	app := vii.New()

	var log []string
	app.Use(globalLogService{log: &log})

	rt := &testRoute{
		log:        &log,
		validators: []vii.AnyValidator{vii.WrapValidator[testValidated](okValidator{})},
		services: []vii.Service{
			logService{name: "a", log: &log},
			logService{name: "b", log: &log},
		},
	}

	if err := app.Mount(http.MethodGet, "/g", rt); err != nil {
		t.Fatalf("mount err: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/g", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	want := []string{
		"svc.before.global",
		"svc.before.a",
		"svc.before.b",
		"route.handle",
		"handler.validated.ok",
		"svc.after.b",
		"svc.after.a",
		"svc.after.global",
	}
	if !equalSlices(log, want) {
		t.Fatalf("log mismatch\n got: %#v\nwant: %#v", log, want)
	}
}
