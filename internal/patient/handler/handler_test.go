package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/DoIttikorn/hospital-middleware/internal/auth"
	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// fake is a Service stub that records what it was called with.
type fake struct {
	res     patient.Result
	err     error
	called  bool
	gotHosp string
	gotCrit patient.Criteria
	gotPage patient.Page
}

func (f *fake) Search(_ context.Context, hospitalID string, c patient.Criteria, p patient.Page) (patient.Result, error) {
	f.called, f.gotHosp, f.gotCrit, f.gotPage = true, hospitalID, c, p
	return f.res, f.err
}

type harness struct {
	router *gin.Engine
	fake   *fake
	token  string
	mgr    *auth.Manager
}

func newHarness(t *testing.T) harness {
	t.Helper()
	mgr, err := auth.NewManager("test-secret-at-least-16-bytes", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := mgr.Issue("staff-1", "hospital-from-token")
	f := &fake{res: patient.Result{Limit: 20, Data: []patient.Patient{}}}
	r := gin.New()
	New(f, slog.New(slog.DiscardHandler)).Register(r.Group("", mgr.Middleware()))
	return harness{router: r, fake: f, token: token, mgr: mgr}
}

func (h harness) get(path, token string) (*httptest.ResponseRecorder, map[string]any) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func ptr(s string) *string { return &s }

func TestSearchSuccess(t *testing.T) {
	t.Run("returns the page with the documented fields", func(t *testing.T) {
		h := newHarness(t)
		h.fake.res = patient.Result{Total: 1, Limit: 20, Offset: 0, Data: []patient.Patient{{
			ID: "internal-id", HospitalID: "internal-hospital",
			FirstNameTH: "สมชาย", LastNameTH: "ใจดี", FirstNameEN: "Somchai", LastNameEN: "Jaidee",
			DateOfBirth: "1990-05-17", PatientHN: "HN0001234", NationalID: ptr("1234567890123"),
			PhoneNumber: "0812345678", Email: "somchai@example.com", Gender: "M",
		}}}
		rec, body := h.get("/patient/search", h.token)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		if body["total"] != float64(1) || body["limit"] != float64(20) || body["offset"] != float64(0) {
			t.Errorf("paging = %v", body)
		}
		data := body["data"].([]any)
		if len(data) != 1 {
			t.Fatalf("data = %v", data)
		}
		p := data[0].(map[string]any)
		for _, field := range []string{
			"first_name_th", "middle_name_th", "last_name_th", "first_name_en", "middle_name_en", "last_name_en",
			"date_of_birth", "patient_hn", "national_id", "passport_id", "phone_number", "email", "gender",
		} {
			if _, ok := p[field]; !ok {
				t.Errorf("response is missing %q", field)
			}
		}
		if p["patient_hn"] != "HN0001234" || p["national_id"] != "1234567890123" || p["passport_id"] != nil || p["gender"] != "M" {
			t.Errorf("patient = %v", p)
		}
		for _, hidden := range []string{"id", "hospital_id", "ID", "HospitalID", "created_at"} {
			if _, ok := p[hidden]; ok {
				t.Errorf("internal field %q is exposed", hidden)
			}
		}
	})

	t.Run("empty data serializes as [] not null", func(t *testing.T) {
		h := newHarness(t)
		rec, _ := h.get("/patient/search", h.token)
		if !strings.Contains(rec.Body.String(), `"data":[]`) {
			t.Errorf("body = %s", rec.Body)
		}
	})

	t.Run("passes every query parameter to the service", func(t *testing.T) {
		h := newHarness(t)
		q := "national_id=1&passport_id=2&first_name=3&middle_name=4&last_name=5&date_of_birth=17/05/1990&phone_number=6&email=a%40b.c&limit=7&offset=8"
		if rec, _ := h.get("/patient/search?"+q, h.token); rec.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		wantCrit := patient.Criteria{NationalID: "1", PassportID: "2", FirstName: "3", MiddleName: "4", LastName: "5",
			DateOfBirth: "1990-05-17", PhoneNumber: "6", Email: "a@b.c"}
		if h.fake.gotCrit != wantCrit {
			t.Errorf("criteria = %+v, want %+v", h.fake.gotCrit, wantCrit)
		}
		if h.fake.gotPage != (patient.Page{Limit: 7, Offset: 8}) {
			t.Errorf("page = %+v", h.fake.gotPage)
		}
	})

	t.Run("no query parameters is a valid search", func(t *testing.T) {
		h := newHarness(t)
		rec, _ := h.get("/patient/search", h.token)
		if rec.Code != http.StatusOK || !h.fake.called {
			t.Errorf("status = %d, service called = %v; want 200 and a call", rec.Code, h.fake.called)
		}
		if h.fake.gotCrit != (patient.Criteria{}) || h.fake.gotPage != (patient.Page{}) {
			t.Errorf("criteria = %+v page = %+v, want zero values", h.fake.gotCrit, h.fake.gotPage)
		}
	})

	t.Run("blank limit and offset use the defaults", func(t *testing.T) {
		h := newHarness(t)
		if rec, _ := h.get("/patient/search?limit=&offset=", h.token); rec.Code != http.StatusOK {
			t.Errorf("status = %d", rec.Code)
		}
		if h.fake.gotPage != (patient.Page{}) {
			t.Errorf("page = %+v", h.fake.gotPage)
		}
	})

	t.Run("hospital comes from the token, not the request", func(t *testing.T) {
		h := newHarness(t)
		h.get("/patient/search?hospital_id=someone-elses&hospital=hospital-b", h.token)
		if h.fake.gotHosp != "hospital-from-token" {
			t.Errorf("service saw hospital %q, want the token's", h.fake.gotHosp)
		}
	})
}

func TestSearchAuth(t *testing.T) {
	h := newHarness(t)
	other, _ := auth.NewManager("a-different-secret-16-bytes", time.Hour)
	forged, _ := other.Issue("staff-1", "hospital-b")
	for name, token := range map[string]string{"no token": "", "garbage": "garbage", "wrong signature": forged} {
		t.Run(name, func(t *testing.T) {
			rec, body := h.get("/patient/search", token)
			if rec.Code != http.StatusUnauthorized || body["code"] != "unauthorized" {
				t.Errorf("status = %d body = %s, want 401 unauthorized", rec.Code, rec.Body)
			}
			if h.fake.called {
				t.Error("service called without authentication")
			}
		})
	}

	t.Run("expired token", func(t *testing.T) {
		mgr, _ := auth.NewManager("test-secret-at-least-16-bytes", time.Nanosecond)
		expired, _ := mgr.Issue("staff-1", "hospital-a")
		time.Sleep(1100 * time.Millisecond) // JWT expiry has one-second precision
		if rec, _ := h.get("/patient/search", expired); rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestSearchErrors(t *testing.T) {
	t.Run("bad limit or offset is 400 and never reaches the service", func(t *testing.T) {
		for _, q := range []string{"limit=abc", "offset=xyz", "limit=1.5", "limit=99999999999999999999"} {
			h := newHarness(t)
			rec, body := h.get("/patient/search?"+q, h.token)
			if rec.Code != http.StatusBadRequest || body["code"] != "validation_error" || h.fake.called {
				t.Errorf("%s: status = %d body = %s called = %v", q, rec.Code, rec.Body, h.fake.called)
			}
		}
	})

	t.Run("date_of_birth not DD/MM/YYYY is 400 and never reaches the service", func(t *testing.T) {
		// The formats themselves are covered in package dates.
		for _, dob := range []string{"1990-05-17", "31/02/1990"} {
			h := newHarness(t)
			rec, body := h.get("/patient/search?date_of_birth="+dob, h.token)
			if rec.Code != http.StatusBadRequest || body["code"] != "validation_error" || h.fake.called {
				t.Errorf("%s: status = %d body = %s called = %v", dob, rec.Code, rec.Body, h.fake.called)
			}
			if detail, _ := body["detail"].(string); !strings.Contains(detail, "date_of_birth must be DD/MM/YYYY") {
				t.Errorf("%s: detail = %q", dob, detail)
			}
		}
	})

	tests := []struct {
		name string
		err  error
		want int
		code any
	}{
		{"validation", fmt.Errorf("%w: date_of_birth must be DD/MM/YYYY", patient.ErrInvalid), http.StatusBadRequest, "validation_error"},
		{"HIS unavailable", fmt.Errorf("%w: status 503", patient.ErrHISUnavailable), http.StatusBadGateway, "his_unavailable"},
		{"unexpected", errors.New("db exploded: secret detail"), http.StatusInternalServerError, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.fake.err = tt.err
			rec, body := h.get("/patient/search", h.token)
			if rec.Code != tt.want || body["code"] != tt.code {
				t.Errorf("status = %d code = %v, want %d %v", rec.Code, body["code"], tt.want, tt.code)
			}
			if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/problem+json") {
				t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
			}
			if strings.Contains(rec.Body.String(), "secret detail") || strings.Contains(rec.Body.String(), "503") {
				t.Errorf("internal detail leaked: %s", rec.Body)
			}
		})
	}
}
