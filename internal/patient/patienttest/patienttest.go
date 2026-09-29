// Package patienttest is the contract every patient.Repository must pass.
//
// The service is tested against the memory adapter, which is only safe if
// every other adapter behaves the same way. Each adapter's tests call Run:
// memory in ordinary unit tests, postgres against a real database
// (INTEGRATION=1).
package patienttest

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/ids"
	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

// Env is a fresh repository and two hospitals it may hold patients for.
type Env struct {
	Repo                 patient.Repository
	HospitalA, HospitalB string
}

func ptr(s string) *string { return &s }

// Run checks the Repository contract. newEnv must return an empty
// repository, and hospitals that exist in the store, on every call.
func Run(t *testing.T, newEnv func(t *testing.T) Env) {
	ctx := context.Background()
	at := time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC)

	somchai := func(hospitalID string) patient.Patient {
		return patient.Patient{
			ID: ids.New(), HospitalID: hospitalID, PatientHN: "HN001",
			NationalID:  ptr("1111111111111"),
			FirstNameTH: "สมชาย", LastNameTH: "ใจดี",
			FirstNameEN: "Somchai", MiddleNameEN: "Kittipong", LastNameEN: "Jaidee",
			DateOfBirth: "1990-05-17", PhoneNumber: "0811111111", Email: "Somchai@Example.com", Gender: "M",
			CreatedAt: at, UpdatedAt: at,
		}
	}
	mary := func(hospitalID string) patient.Patient {
		return patient.Patient{
			ID: ids.New(), HospitalID: hospitalID, PatientHN: "HN002",
			PassportID:  ptr("AA1234567"),
			FirstNameEN: "Mary", LastNameEN: "Smith",
			DateOfBirth: "1985-01-30", PhoneNumber: "0822222222", Email: "mary@example.com", Gender: "F",
			CreatedAt: at, UpdatedAt: at,
		}
	}
	// seed stores patients and fails the test on error.
	seed := func(t *testing.T, e Env, ps ...patient.Patient) {
		t.Helper()
		for _, p := range ps {
			if _, err := e.Repo.Upsert(ctx, p); err != nil {
				t.Fatalf("Upsert(%s): %v", p.PatientHN, err)
			}
		}
	}
	search := func(t *testing.T, e Env, hospitalID string, c patient.Criteria, p patient.Page) ([]string, int) {
		t.Helper()
		got, total, err := e.Repo.Search(ctx, hospitalID, c, p)
		if err != nil {
			t.Fatalf("Search(%+v): %v", c, err)
		}
		if got == nil {
			t.Fatal("Search returned a nil slice, want a non-nil one")
		}
		hns := make([]string, len(got))
		for i, p := range got {
			hns[i] = p.PatientHN
		}
		return hns, total
	}
	all := patient.Page{}

	t.Run("upsert then search returns every field", func(t *testing.T) {
		e := newEnv(t)
		want := somchai(e.HospitalA)
		stored, err := e.Repo.Upsert(ctx, want)
		if err != nil {
			t.Fatal(err)
		}
		got, _, err := e.Repo.Search(ctx, e.HospitalA, patient.Criteria{NationalID: "1111111111111"}, all)
		if err != nil || len(got) != 1 {
			t.Fatalf("Search = %v, %v; want 1 patient", got, err)
		}
		for name, p := range map[string]patient.Patient{"Upsert result": stored, "Search result": got[0]} {
			if p.ID != want.ID || p.HospitalID != want.HospitalID || p.PatientHN != want.PatientHN ||
				p.FirstNameTH != want.FirstNameTH || p.MiddleNameTH != "" || p.LastNameTH != want.LastNameTH ||
				p.FirstNameEN != want.FirstNameEN || p.MiddleNameEN != want.MiddleNameEN || p.LastNameEN != want.LastNameEN ||
				p.DateOfBirth != want.DateOfBirth || p.PhoneNumber != want.PhoneNumber ||
				p.Email != want.Email || p.Gender != want.Gender {
				t.Errorf("%s = %+v\nwant %+v", name, p, want)
			}
			if p.NationalID == nil || *p.NationalID != "1111111111111" || p.PassportID != nil {
				t.Errorf("%s ids = %v / %v, want national only", name, p.NationalID, p.PassportID)
			}
			if !p.CreatedAt.Equal(at) || !p.UpdatedAt.Equal(at) {
				t.Errorf("%s timestamps = %v / %v", name, p.CreatedAt, p.UpdatedAt)
			}
		}
	})

	t.Run("upsert with the same HN replaces the record and keeps its identity", func(t *testing.T) {
		e := newEnv(t)
		first := somchai(e.HospitalA)
		seed(t, e, first)
		later := at.Add(time.Hour)
		changed := somchai(e.HospitalA) // new ID, same HN
		changed.PhoneNumber, changed.UpdatedAt, changed.CreatedAt = "0899999999", later, later
		stored, err := e.Repo.Upsert(ctx, changed)
		if err != nil {
			t.Fatal(err)
		}
		if stored.ID != first.ID || !stored.CreatedAt.Equal(at) || !stored.UpdatedAt.Equal(later) || stored.PhoneNumber != "0899999999" {
			t.Errorf("stored = %+v, want ID %s, created %v, updated %v, new phone", stored, first.ID, at, later)
		}
		if hns, total := search(t, e, e.HospitalA, patient.Criteria{}, all); total != 1 || len(hns) != 1 {
			t.Errorf("after upsert: %v (total %d), want a single record", hns, total)
		}
	})

	t.Run("same HN in another hospital is a different patient", func(t *testing.T) {
		e := newEnv(t)
		seed(t, e, somchai(e.HospitalA), somchai(e.HospitalB))
		for _, h := range []string{e.HospitalA, e.HospitalB} {
			if _, total := search(t, e, h, patient.Criteria{}, all); total != 1 {
				t.Errorf("hospital %s total = %d, want 1", h, total)
			}
		}
	})

	t.Run("search never crosses hospitals", func(t *testing.T) {
		e := newEnv(t)
		seed(t, e, somchai(e.HospitalA), mary(e.HospitalB))
		if hns, total := search(t, e, e.HospitalA, patient.Criteria{}, all); !slices.Equal(hns, []string{"HN001"}) || total != 1 {
			t.Errorf("hospital A sees %v (total %d), want [HN001]", hns, total)
		}
		if hns, total := search(t, e, e.HospitalA, patient.Criteria{PassportID: "AA1234567"}, all); len(hns) != 0 || total != 0 {
			t.Errorf("hospital A found hospital B's patient: %v (total %d)", hns, total)
		}
	})

	t.Run("search with no criteria lists the hospital ordered by HN", func(t *testing.T) {
		e := newEnv(t)
		seed(t, e, mary(e.HospitalA), somchai(e.HospitalA))
		if hns, total := search(t, e, e.HospitalA, patient.Criteria{}, all); !slices.Equal(hns, []string{"HN001", "HN002"}) || total != 2 {
			t.Errorf("got %v (total %d), want [HN001 HN002]", hns, total)
		}
	})

	t.Run("search on an empty hospital is empty", func(t *testing.T) {
		e := newEnv(t)
		if hns, total := search(t, e, e.HospitalA, patient.Criteria{}, all); len(hns) != 0 || total != 0 {
			t.Errorf("got %v (total %d), want none", hns, total)
		}
	})

	t.Run("criteria", func(t *testing.T) {
		e := newEnv(t)
		seed(t, e, somchai(e.HospitalA), mary(e.HospitalA))
		tests := []struct {
			name string
			c    patient.Criteria
			want []string
		}{
			{"national_id exact", patient.Criteria{NationalID: "1111111111111"}, []string{"HN001"}},
			{"national_id is not a prefix match", patient.Criteria{NationalID: "1111"}, nil},
			{"passport_id exact", patient.Criteria{PassportID: "AA1234567"}, []string{"HN002"}},
			{"first_name matches the English name", patient.Criteria{FirstName: "Som"}, []string{"HN001"}},
			{"first_name matches the Thai name", patient.Criteria{FirstName: "สม"}, []string{"HN001"}},
			{"first_name ignores case", patient.Criteria{FirstName: "mARY"}, []string{"HN002"}},
			{"first_name is a prefix, not a substring", patient.Criteria{FirstName: "chai"}, nil},
			{"middle_name matches", patient.Criteria{MiddleName: "kitt"}, []string{"HN001"}},
			{"last_name matches the English name", patient.Criteria{LastName: "jai"}, []string{"HN001"}},
			{"last_name matches the Thai name", patient.Criteria{LastName: "ใจ"}, []string{"HN001"}},
			{"date_of_birth exact", patient.Criteria{DateOfBirth: "1985-01-30"}, []string{"HN002"}},
			{"date_of_birth no match", patient.Criteria{DateOfBirth: "2000-01-01"}, nil},
			{"phone_number exact", patient.Criteria{PhoneNumber: "0811111111"}, []string{"HN001"}},
			{"email ignores case", patient.Criteria{Email: "SOMCHAI@example.COM"}, []string{"HN001"}},
			{"criteria are AND-ed", patient.Criteria{FirstName: "Som", LastName: "Jaidee", DateOfBirth: "1990-05-17"}, []string{"HN001"}},
			{"AND with one mismatch finds nothing", patient.Criteria{FirstName: "Som", DateOfBirth: "1985-01-30"}, nil},
			{"% is literal, not a wildcard", patient.Criteria{FirstName: "%"}, nil},
			{"_ is literal, not a wildcard", patient.Criteria{FirstName: "_omchai"}, nil},
			{"backslash is literal", patient.Criteria{FirstName: `\`}, nil},
			{"quotes are just characters", patient.Criteria{FirstName: `'; DROP TABLE patients; --`}, nil},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				hns, total := search(t, e, e.HospitalA, tt.c, all)
				if !slices.Equal(hns, tt.want) || total != len(tt.want) {
					t.Errorf("got %v (total %d), want %v", hns, total, tt.want)
				}
			})
		}
	})

	t.Run("pagination", func(t *testing.T) {
		e := newEnv(t)
		for _, hn := range []string{"HN001", "HN002", "HN003", "HN004", "HN005"} {
			p := somchai(e.HospitalA)
			p.PatientHN = hn
			p.NationalID = ptr("id-" + hn)
			seed(t, e, p)
		}
		tests := []struct {
			name string
			page patient.Page
			want []string
		}{
			{"first page", patient.Page{Limit: 2}, []string{"HN001", "HN002"}},
			{"second page", patient.Page{Limit: 2, Offset: 2}, []string{"HN003", "HN004"}},
			{"last partial page", patient.Page{Limit: 2, Offset: 4}, []string{"HN005"}},
			{"offset past the end", patient.Page{Limit: 2, Offset: 10}, nil},
			{"limit larger than the total", patient.Page{Limit: 50}, []string{"HN001", "HN002", "HN003", "HN004", "HN005"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				hns, total := search(t, e, e.HospitalA, patient.Criteria{}, tt.page)
				if !slices.Equal(hns, tt.want) {
					t.Errorf("got %v, want %v", hns, tt.want)
				}
				if total != 5 {
					t.Errorf("total = %d, want 5 whatever the page", total)
				}
			})
		}
	})
}
