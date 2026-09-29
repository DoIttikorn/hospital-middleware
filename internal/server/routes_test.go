package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/auth"
	"github.com/DoIttikorn/hospital-middleware/internal/his/histest"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	hospitalmem "github.com/DoIttikorn/hospital-middleware/internal/hospital/memory"
	"github.com/DoIttikorn/hospital-middleware/internal/patient"
	patientmem "github.com/DoIttikorn/hospital-middleware/internal/patient/memory"
	"github.com/DoIttikorn/hospital-middleware/internal/staff"
	staffmem "github.com/DoIttikorn/hospital-middleware/internal/staff/memory"
)

var (
	testHospA = hospital.Hospital{ID: "id-a", Code: "hospital-a", Name: "Hospital A", HISBaseURL: "https://a"}
	testHospB = hospital.Hospital{ID: "id-b", Code: "hospital-b", Name: "Hospital B"}
)

const adminPassword = "Admin@12345"

// testEnv is a Server without external dependencies: every health check
// passes, the domains use their memory adapters, and hospital A's HIS is a
// fake holding the patients in his.
type testEnv struct {
	server *Server
	his    *histest.Client
}

// newTestServer returns a Server whose hospitals and admin staff mirror the
// migration seed data (admin/adminPassword in each hospital).
func newTestServer() *Server { return newTestEnv().server }

func newTestEnv() testEnv {
	tokens, err := auth.NewManager("test-secret-at-least-16-bytes", time.Hour)
	if err != nil {
		panic(err)
	}
	hospitals := hospital.NewService(hospitalmem.New(testHospA, testHospB))
	staffRepo := staffmem.New()
	staffSvc := staff.NewService(staffRepo, hospitals, tokens)

	hash, err := auth.HashPassword(adminPassword)
	if err != nil {
		panic(err)
	}
	for _, h := range []hospital.Hospital{testHospA, testHospB} {
		err := staffRepo.Insert(context.Background(), staff.Staff{
			ID: "admin-" + h.ID, HospitalID: h.ID, Username: "admin", PasswordHash: hash,
		})
		if err != nil {
			panic(err)
		}
	}

	client := &histest.Client{Patients: map[string]patient.Patient{}}
	patients := patient.NewService(patientmem.New(), hospitals, histest.Registry{"hospital-a": client})
	return testEnv{
		his: client,
		server: &Server{
			log:      slog.New(slog.DiscardHandler),
			auth:     tokens,
			staff:    staffSvc,
			patients: patients,
		},
	}
}

// do sends a request through the full handler and decodes a JSON response
// into out, if out is not nil. token, if not empty, is sent as a bearer
// token.
func do(t *testing.T, h http.Handler, method, path, body string, out any) *httptest.ResponseRecorder {
	t.Helper()
	return doAs(t, h, "", method, path, body, out)
}

func doAs(t *testing.T, h http.Handler, token, method, path, body string, out any) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec
}

func TestUnknownRouteIs404(t *testing.T) {
	h := newTestServer().Handler()
	for _, path := range []string{"/", "/nope", "/patient", "/staff"} {
		if rec := do(t, h, http.MethodGet, path, "", nil); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}
