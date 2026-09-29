package patient_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/his/histest"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	hospitalmem "github.com/DoIttikorn/hospital-middleware/internal/hospital/memory"
	"github.com/DoIttikorn/hospital-middleware/internal/patient"
	patientmem "github.com/DoIttikorn/hospital-middleware/internal/patient/memory"
)

var (
	hospA = hospital.Hospital{ID: "id-a", Code: "hospital-a", Name: "Hospital A", HISBaseURL: "https://a"}
	hospB = hospital.Hospital{ID: "id-b", Code: "hospital-b", Name: "Hospital B"} // no HIS
)

func ptr(s string) *string { return &s }

type env struct {
	svc  patient.Service
	repo *patientmem.Repository
	his  *histest.Client
}

func newEnv(t *testing.T, hisPatients ...patient.Patient) env {
	t.Helper()
	client := &histest.Client{Patients: map[string]patient.Patient{}}
	for _, p := range hisPatients {
		if p.NationalID != nil {
			client.Patients[*p.NationalID] = p
		}
		if p.PassportID != nil {
			client.Patients[*p.PassportID] = p
		}
	}
	repo := patientmem.New()
	svc := patient.NewService(repo, hospital.NewService(hospitalmem.New(hospA, hospB)), histest.Registry{"hospital-a": client})
	return env{svc: svc, repo: repo, his: client}
}

func (e env) seed(t *testing.T, ps ...patient.Patient) {
	t.Helper()
	for i, p := range ps {
		if p.ID == "" {
			p.ID = fmt.Sprintf("id-%s-%d", p.PatientHN, i)
		}
		if _, err := e.repo.Upsert(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
}

func hns(r patient.Result) []string {
	out := make([]string, len(r.Data))
	for i, p := range r.Data {
		out[i] = p.PatientHN
	}
	return out
}

var (
	somchai = patient.Patient{HospitalID: hospA.ID, PatientHN: "HN001", NationalID: ptr("111"),
		FirstNameEN: "Somchai", LastNameEN: "Jaidee", DateOfBirth: "1990-05-17"}
	mary = patient.Patient{HospitalID: hospA.ID, PatientHN: "HN002", PassportID: ptr("P222"),
		FirstNameEN: "Mary", LastNameEN: "Smith", DateOfBirth: "1985-01-30"}
	other = patient.Patient{HospitalID: hospB.ID, PatientHN: "HN900", NationalID: ptr("999"),
		FirstNameEN: "Other", LastNameEN: "Person"}
)

func TestSearchFromLocalStore(t *testing.T) {
	ctx := context.Background()

	t.Run("no criteria lists the hospital's patients", func(t *testing.T) {
		e := newEnv(t)
		e.seed(t, mary, somchai, other)
		res, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{}, patient.Page{})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(hns(res), []string{"HN001", "HN002"}) || res.Total != 2 {
			t.Errorf("got %v (total %d), want [HN001 HN002]", hns(res), res.Total)
		}
		if len(e.his.Calls()) != 0 {
			t.Error("HIS called for a search without identifiers")
		}
	})

	t.Run("empty result is an empty list, not nil", func(t *testing.T) {
		res, err := newEnv(t).svc.Search(ctx, hospA.ID, patient.Criteria{FirstName: "Nobody"}, patient.Page{})
		if err != nil || res.Data == nil || len(res.Data) != 0 || res.Total != 0 {
			t.Errorf("got %+v, %v", res, err)
		}
	})

	t.Run("filters by criteria", func(t *testing.T) {
		e := newEnv(t)
		e.seed(t, somchai, mary)
		res, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{FirstName: "mar", DateOfBirth: "1985-01-30"}, patient.Page{})
		if err != nil || !slices.Equal(hns(res), []string{"HN002"}) {
			t.Errorf("got %v, %v", hns(res), err)
		}
	})

	t.Run("trims blank and padded criteria", func(t *testing.T) {
		e := newEnv(t)
		e.seed(t, somchai, mary)
		res, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{FirstName: "  som ", LastName: "   ", Email: ""}, patient.Page{})
		if err != nil || !slices.Equal(hns(res), []string{"HN001"}) {
			t.Errorf("got %v, %v; blank fields must be ignored", hns(res), err)
		}
	})

	t.Run("only sees its own hospital, also without criteria", func(t *testing.T) {
		e := newEnv(t)
		e.seed(t, somchai, other)
		res, _ := e.svc.Search(ctx, hospA.ID, patient.Criteria{}, patient.Page{})
		if slices.Contains(hns(res), "HN900") {
			t.Errorf("hospital A sees hospital B's patient: %v", hns(res))
		}
		res, _ = e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "999"}, patient.Page{})
		if len(res.Data) != 0 {
			t.Errorf("hospital A found hospital B's patient by national_id: %v", hns(res))
		}
	})

	t.Run("pagination defaults, caps and windows", func(t *testing.T) {
		e := newEnv(t)
		for i := range 25 {
			p := somchai
			p.PatientHN = fmt.Sprintf("HN%03d", i)
			p.NationalID = ptr(fmt.Sprintf("nid-%d", i))
			e.seed(t, p)
		}
		tests := []struct {
			name           string
			page           patient.Page
			wantLen        int
			wantLim, wantO int
		}{
			{"defaults", patient.Page{}, 20, 20, 0},
			{"custom", patient.Page{Limit: 5, Offset: 10}, 5, 5, 10},
			{"limit is capped", patient.Page{Limit: 1000}, 25, 100, 0},
			{"offset past the end", patient.Page{Offset: 30}, 0, 20, 30},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{}, tt.page)
				if err != nil {
					t.Fatal(err)
				}
				if len(res.Data) != tt.wantLen || res.Limit != tt.wantLim || res.Offset != tt.wantO || res.Total != 25 {
					t.Errorf("len=%d limit=%d offset=%d total=%d; want len=%d limit=%d offset=%d total=25",
						len(res.Data), res.Limit, res.Offset, res.Total, tt.wantLen, tt.wantLim, tt.wantO)
				}
			})
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		e := newEnv(t)
		tests := map[string]struct {
			c patient.Criteria
			p patient.Page
		}{
			"bad date":        {patient.Criteria{DateOfBirth: "17/05/1990"}, patient.Page{}},
			"impossible date": {patient.Criteria{DateOfBirth: "1990-13-45"}, patient.Page{}},
			"negative limit":  {patient.Criteria{}, patient.Page{Limit: -1}},
			"negative offset": {patient.Criteria{}, patient.Page{Offset: -1}},
		}
		for name, tt := range tests {
			t.Run(name, func(t *testing.T) {
				if _, err := e.svc.Search(ctx, hospA.ID, tt.c, tt.p); !errors.Is(err, patient.ErrInvalid) {
					t.Errorf("err = %v, want ErrInvalid", err)
				}
			})
		}
	})
}

func TestSearchHISFallback(t *testing.T) {
	ctx := context.Background()
	fromHIS := func(hn, national string) patient.Patient {
		return patient.Patient{PatientHN: hn, NationalID: ptr(national), FirstNameEN: "Imported", LastNameEN: "Person", DateOfBirth: "1970-01-01"}
	}

	t.Run("imports a patient found by national_id and returns it", func(t *testing.T) {
		e := newEnv(t, fromHIS("HN777", "777"))
		res, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "777"}, patient.Page{})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(hns(res), []string{"HN777"}) || res.Total != 1 {
			t.Fatalf("got %v (total %d)", hns(res), res.Total)
		}
		if !slices.Equal(e.his.Calls(), []string{"777"}) {
			t.Errorf("HIS calls = %v", e.his.Calls())
		}
		// Cached: the next search is answered locally.
		if _, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "777"}, patient.Page{}); err != nil {
			t.Fatal(err)
		}
		if len(e.his.Calls()) != 1 {
			t.Errorf("HIS called again for a cached patient: %v", e.his.Calls())
		}
	})

	t.Run("imported patient belongs to the searching staff's hospital", func(t *testing.T) {
		e := newEnv(t, fromHIS("HN777", "777"))
		if _, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "777"}, patient.Page{}); err != nil {
			t.Fatal(err)
		}
		stored, _, _ := e.repo.Search(ctx, hospA.ID, patient.Criteria{NationalID: "777"}, patient.Page{})
		if len(stored) != 1 || stored[0].HospitalID != hospA.ID || stored[0].ID == "" || stored[0].CreatedAt.IsZero() {
			t.Errorf("stored = %+v", stored)
		}
		if stored, _, _ := e.repo.Search(ctx, hospB.ID, patient.Criteria{}, patient.Page{}); len(stored) != 0 {
			t.Errorf("import leaked into hospital B: %+v", stored)
		}
	})

	t.Run("finds by passport_id", func(t *testing.T) {
		e := newEnv(t, patient.Patient{PatientHN: "HN888", PassportID: ptr("PP888"), FirstNameEN: "Passport"})
		res, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{PassportID: "PP888"}, patient.Page{})
		if err != nil || !slices.Equal(hns(res), []string{"HN888"}) {
			t.Errorf("got %v, %v", hns(res), err)
		}
	})

	t.Run("national_id wins when both identifiers are given", func(t *testing.T) {
		e := newEnv(t, fromHIS("HN777", "777"))
		_, _ = e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "777", PassportID: "zzz"}, patient.Page{})
		if !slices.Equal(e.his.Calls(), []string{"777"}) {
			t.Errorf("HIS calls = %v, want [777]", e.his.Calls())
		}
	})

	t.Run("stores the searched id when the HIS omits it", func(t *testing.T) {
		e := newEnv(t)
		e.his.Patients["555"] = patient.Patient{PatientHN: "HN555", FirstNameEN: "NoIDs"}
		res, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "555"}, patient.Page{})
		if err != nil || len(res.Data) != 1 || res.Data[0].NationalID == nil || *res.Data[0].NationalID != "555" {
			t.Errorf("got %+v, %v", res, err)
		}
	})

	t.Run("other criteria still apply to the imported patient", func(t *testing.T) {
		e := newEnv(t, fromHIS("HN777", "777"))
		res, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "777", FirstName: "Somebody"}, patient.Page{})
		if err != nil || len(res.Data) != 0 {
			t.Errorf("got %v, %v; the first name does not match", hns(res), err)
		}
	})

	t.Run("HIS without the patient is an empty result", func(t *testing.T) {
		e := newEnv(t)
		res, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "404"}, patient.Page{})
		if err != nil || len(res.Data) != 0 || res.Total != 0 {
			t.Errorf("got %+v, %v", res, err)
		}
	})

	t.Run("HIS unavailable", func(t *testing.T) {
		e := newEnv(t)
		e.his.Err = fmt.Errorf("%w: status 503", patient.ErrHISUnavailable)
		if _, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "777"}, patient.Page{}); !errors.Is(err, patient.ErrHISUnavailable) {
			t.Errorf("err = %v, want ErrHISUnavailable", err)
		}
	})

	t.Run("HIS record without an HN is rejected", func(t *testing.T) {
		e := newEnv(t)
		e.his.Patients["333"] = patient.Patient{NationalID: ptr("333"), FirstNameEN: "NoHN"}
		if _, err := e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "333"}, patient.Page{}); !errors.Is(err, patient.ErrHISUnavailable) {
			t.Errorf("err = %v, want ErrHISUnavailable", err)
		}
	})

	t.Run("hospital without a HIS gives an empty result", func(t *testing.T) {
		e := newEnv(t, fromHIS("HN777", "777"))
		res, err := e.svc.Search(ctx, hospB.ID, patient.Criteria{NationalID: "777"}, patient.Page{})
		if err != nil || len(res.Data) != 0 {
			t.Errorf("got %+v, %v", res, err)
		}
		if len(e.his.Calls()) != 0 {
			t.Error("hospital A's HIS was asked on behalf of hospital B")
		}
	})

	t.Run("no fallback without an identifier, or on later pages", func(t *testing.T) {
		e := newEnv(t, fromHIS("HN777", "777"))
		_, _ = e.svc.Search(ctx, hospA.ID, patient.Criteria{FirstName: "Imported"}, patient.Page{})
		_, _ = e.svc.Search(ctx, hospA.ID, patient.Criteria{NationalID: "777"}, patient.Page{Offset: 20})
		if len(e.his.Calls()) != 0 {
			t.Errorf("HIS calls = %v, want none", e.his.Calls())
		}
	})

	t.Run("unknown hospital of the staff is an error", func(t *testing.T) {
		e := newEnv(t)
		if _, err := e.svc.Search(ctx, "no-such-hospital", patient.Criteria{NationalID: "777"}, patient.Page{}); !errors.Is(err, hospital.ErrNotFound) {
			t.Errorf("err = %v, want hospital.ErrNotFound", err)
		}
	})
}
