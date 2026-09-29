// Package his has the adapters that read patients from each hospital's HIS
// (Hospital Information System) and the Registry that picks the right one
// for a hospital. They implement the ports declared in the patient package.
package his

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

// maxResponseBytes bounds how much of a HIS response is read.
const maxResponseBytes = 1 << 20

// HospitalA is the client for Hospital A's HIS:
//
//	GET {base}/patient/search/{id}   id is a national ID or a passport ID
//
// 200 returns the patient as JSON, 404 means there is no such patient, and
// anything else counts as the HIS being unavailable.
type HospitalA struct {
	baseURL string
	client  *http.Client
}

var _ patient.HISClient = (*HospitalA)(nil)

// NewHospitalA returns a client for the HIS at baseURL. timeout bounds each
// request.
func NewHospitalA(baseURL string, timeout time.Duration) *HospitalA {
	return &HospitalA{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

// hospitalAPatient is the JSON body Hospital A returns. Pointers tell a
// missing field from an empty one.
type hospitalAPatient struct {
	FirstNameTH  *string `json:"first_name_th"`
	MiddleNameTH *string `json:"middle_name_th"`
	LastNameTH   *string `json:"last_name_th"`
	FirstNameEN  *string `json:"first_name_en"`
	MiddleNameEN *string `json:"middle_name_en"`
	LastNameEN   *string `json:"last_name_en"`
	DateOfBirth  *string `json:"date_of_birth"`
	PatientHN    *string `json:"patient_hn"`
	NationalID   *string `json:"national_id"`
	PassportID   *string `json:"passport_id"`
	PhoneNumber  *string `json:"phone_number"`
	Email        *string `json:"email"`
	Gender       *string `json:"gender"`
}

func (c *HospitalA) FindByID(ctx context.Context, id string) (patient.Patient, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/patient/search/"+url.PathEscape(id), nil)
	if err != nil {
		return patient.Patient{}, fmt.Errorf("%w: build request: %v", patient.ErrHISUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return patient.Patient{}, fmt.Errorf("%w: %v", patient.ErrHISUnavailable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return patient.Patient{}, fmt.Errorf("%w in hospital A", patient.ErrNotFound)
	case resp.StatusCode != http.StatusOK:
		return patient.Patient{}, fmt.Errorf("%w: status %d", patient.ErrHISUnavailable, resp.StatusCode)
	}

	var wire hospitalAPatient
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&wire); err != nil {
		return patient.Patient{}, fmt.Errorf("%w: bad response: %v", patient.ErrHISUnavailable, err)
	}
	return wire.toPatient(), nil
}

func (w hospitalAPatient) toPatient() patient.Patient {
	str := func(s *string) string {
		if s == nil {
			return ""
		}
		return strings.TrimSpace(*s)
	}
	// Identifiers stay nil when blank: the store treats "" as a real value.
	id := func(s *string) *string {
		if v := str(s); v != "" {
			return &v
		}
		return nil
	}
	return patient.Patient{
		FirstNameTH:  str(w.FirstNameTH),
		MiddleNameTH: str(w.MiddleNameTH),
		LastNameTH:   str(w.LastNameTH),
		FirstNameEN:  str(w.FirstNameEN),
		MiddleNameEN: str(w.MiddleNameEN),
		LastNameEN:   str(w.LastNameEN),
		DateOfBirth:  normalizeDate(str(w.DateOfBirth)),
		PatientHN:    str(w.PatientHN),
		NationalID:   id(w.NationalID),
		PassportID:   id(w.PassportID),
		PhoneNumber:  str(w.PhoneNumber),
		Email:        str(w.Email),
		Gender:       normalizeGender(str(w.Gender)),
	}
}

// normalizeDate accepts "YYYY-MM-DD" or an RFC 3339 timestamp and returns
// "YYYY-MM-DD", or "" for anything else, so a bad date never blocks the
// import.
func normalizeDate(s string) string {
	if t, err := time.Parse(patient.DateLayout, s); err == nil {
		return t.Format(patient.DateLayout)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format(patient.DateLayout)
	}
	return ""
}

// normalizeGender returns "M" or "F", or "" for anything else.
func normalizeGender(s string) string {
	switch g := strings.ToUpper(s); g {
	case "M", "F":
		return g
	}
	return ""
}
