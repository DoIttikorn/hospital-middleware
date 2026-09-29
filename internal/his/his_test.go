package his

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

const okBody = `{
	"first_name_th": "สมชาย", "middle_name_th": "", "last_name_th": "ใจดี",
	"first_name_en": "Somchai", "middle_name_en": null, "last_name_en": " Jaidee ",
	"date_of_birth": "1990-05-17", "patient_hn": "HN0001234",
	"national_id": "1234567890123", "passport_id": "",
	"phone_number": "0812345678", "email": "somchai@example.com", "gender": "m"
}`

func serve(t *testing.T, h http.HandlerFunc) *HospitalA {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewHospitalA(srv.URL+"/", time.Second) // trailing slash is tolerated
}

func TestHospitalAFindByID(t *testing.T) {
	ctx := context.Background()

	t.Run("200 maps the patient", func(t *testing.T) {
		var gotPath string
		c := serve(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.EscapedPath()
			w.Write([]byte(okBody))
		})
		p, err := c.FindByID(ctx, "1234567890123")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/patient/search/1234567890123" {
			t.Errorf("path = %q", gotPath)
		}
		if p.PatientHN != "HN0001234" || p.FirstNameTH != "สมชาย" || p.LastNameEN != "Jaidee" ||
			p.MiddleNameEN != "" || p.DateOfBirth != "1990-05-17" || p.Gender != "M" ||
			p.PhoneNumber != "0812345678" || p.Email != "somchai@example.com" {
			t.Errorf("patient = %+v", p)
		}
		if p.NationalID == nil || *p.NationalID != "1234567890123" {
			t.Errorf("national_id = %v", p.NationalID)
		}
		if p.PassportID != nil {
			t.Errorf("blank passport_id = %q, want nil", *p.PassportID)
		}
	})

	t.Run("escapes the id in the path", func(t *testing.T) {
		var gotPath string
		c := serve(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.EscapedPath()
			w.Write([]byte(okBody))
		})
		if _, err := c.FindByID(ctx, "a/b c"); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/patient/search/a%2Fb%20c" {
			t.Errorf("path = %q", gotPath)
		}
	})

	t.Run("normalizes dates and genders", func(t *testing.T) {
		tests := []struct{ in, want string }{
			{"1990-05-17", "1990-05-17"},
			{"1990-05-17T00:00:00Z", "1990-05-17"},
			{"17/05/1990", ""},
			{"", ""},
		}
		for _, tt := range tests {
			if got := normalizeDate(tt.in); got != tt.want {
				t.Errorf("normalizeDate(%q) = %q, want %q", tt.in, got, tt.want)
			}
		}
		for in, want := range map[string]string{"M": "M", "f": "F", "X": "", "": ""} {
			if got := normalizeGender(in); got != want {
				t.Errorf("normalizeGender(%q) = %q, want %q", in, got, want)
			}
		}
	})

	t.Run("404 is ErrNotFound", func(t *testing.T) {
		c := serve(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
		if _, err := c.FindByID(ctx, "x"); !errors.Is(err, patient.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	unavailable := map[string]http.HandlerFunc{
		"500":      func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) },
		"401":      func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) },
		"bad json": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{nope`)) },
		"empty":    func(w http.ResponseWriter, r *http.Request) {},
	}
	for name, h := range unavailable {
		t.Run(name+" is ErrHISUnavailable", func(t *testing.T) {
			if _, err := serve(t, h).FindByID(ctx, "x"); !errors.Is(err, patient.ErrHISUnavailable) {
				t.Errorf("err = %v, want ErrHISUnavailable", err)
			}
		})
	}

	t.Run("timeout is ErrHISUnavailable", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
		}))
		t.Cleanup(srv.Close)
		c := NewHospitalA(srv.URL, 50*time.Millisecond)
		if _, err := c.FindByID(ctx, "x"); !errors.Is(err, patient.ErrHISUnavailable) {
			t.Errorf("err = %v, want ErrHISUnavailable", err)
		}
	})

	t.Run("unreachable host is ErrHISUnavailable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close() // nothing listens there any more
		if _, err := NewHospitalA(url, time.Second).FindByID(ctx, "x"); !errors.Is(err, patient.ErrHISUnavailable) {
			t.Errorf("err = %v, want ErrHISUnavailable", err)
		}
	})

	t.Run("cancelled context is ErrHISUnavailable", func(t *testing.T) {
		c := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(okBody)) })
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := c.FindByID(cctx, "x"); !errors.Is(err, patient.ErrHISUnavailable) {
			t.Errorf("err = %v, want ErrHISUnavailable", err)
		}
	})
}

func TestRegistry(t *testing.T) {
	hospA := hospital.Hospital{Code: "hospital-a", HISBaseURL: "https://hospital-a.api.co.th"}

	t.Run("hospital with an adapter and URL", func(t *testing.T) {
		c, ok := NewRegistry(time.Second, nil).For(hospA)
		if !ok {
			t.Fatal("no client for hospital-a")
		}
		if got := c.(*HospitalA).baseURL; got != "https://hospital-a.api.co.th" {
			t.Errorf("baseURL = %q", got)
		}
	})

	t.Run("override wins over the stored URL", func(t *testing.T) {
		c, ok := NewRegistry(time.Second, map[string]string{"hospital-a": "http://localhost:9999"}).For(hospA)
		if !ok || c.(*HospitalA).baseURL != "http://localhost:9999" {
			t.Errorf("client = %+v, %v", c, ok)
		}
	})

	t.Run("no adapter for the hospital", func(t *testing.T) {
		if _, ok := NewRegistry(time.Second, nil).For(hospital.Hospital{Code: "hospital-b", HISBaseURL: "https://b"}); ok {
			t.Error("client returned for a hospital without an adapter")
		}
	})

	t.Run("no HIS address", func(t *testing.T) {
		if _, ok := NewRegistry(time.Second, nil).For(hospital.Hospital{Code: "hospital-a"}); ok {
			t.Error("client returned without a base URL")
		}
	})
}
