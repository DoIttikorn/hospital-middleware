// Package patient is the patient search domain. Patients belong to one
// hospital; staff only ever see patients of their own hospital. Patient
// records come from the hospital's HIS (Hospital Information System) and are
// cached in our database so they can be searched by any field.
//
// Layout, like every domain package:
//
//   - this package holds the model, the business rules (Service) and the
//     ports the rules need (Repository, HISClient, HISRegistry); it imports
//     no database driver and no web framework;
//   - adapters live in sub-packages: memory (test double), postgres (the
//     real store) and handler (the REST endpoint); the HIS adapters are in
//     internal/his;
//   - patienttest is the contract every Repository must pass.
package patient

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Patient is a patient record as returned by the API. ID and HospitalID are
// internal and never serialized.
type Patient struct {
	ID         string `json:"-"`
	HospitalID string `json:"-"`

	FirstNameTH  string `json:"first_name_th"`
	MiddleNameTH string `json:"middle_name_th"`
	LastNameTH   string `json:"last_name_th"`
	FirstNameEN  string `json:"first_name_en"`
	MiddleNameEN string `json:"middle_name_en"`
	LastNameEN   string `json:"last_name_en"`
	// DateOfBirth is YYYY-MM-DD, or empty when unknown.
	DateOfBirth string  `json:"date_of_birth"`
	PatientHN   string  `json:"patient_hn"`
	NationalID  *string `json:"national_id"`
	PassportID  *string `json:"passport_id"`
	PhoneNumber string  `json:"phone_number"`
	Email       string  `json:"email"`
	Gender      string  `json:"gender"` // "M", "F", or empty when unknown

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// Criteria is what a search filters on. Every field is optional and blank
// fields are ignored; the filled ones are AND-ed.
//
// Matching: national_id, passport_id, phone_number and date_of_birth are
// exact; email is exact ignoring case; the three names are a
// case-insensitive prefix match against the Thai or the English name.
type Criteria struct {
	NationalID  string
	PassportID  string
	FirstName   string
	MiddleName  string
	LastName    string
	DateOfBirth string // YYYY-MM-DD
	PhoneNumber string
	Email       string
}

// Page selects a window of the results.
type Page struct {
	Limit  int
	Offset int
}

// Pagination limits.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// DateLayout is the format of Patient.DateOfBirth and Criteria.DateOfBirth.
const DateLayout = "2006-01-02"

// Result is one page of search results.
type Result struct {
	Total  int       `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
	Data   []Patient `json:"data"`
}

var (
	// ErrInvalid wraps validation errors in the search input.
	ErrInvalid = errors.New("invalid search")
	// ErrNotFound is returned by a HISClient when the HIS has no such
	// patient.
	ErrNotFound = errors.New("patient not found")
	// ErrHISUnavailable is returned when the hospital's HIS could not
	// answer: unreachable, timed out, an error status, or a bad response.
	ErrHISUnavailable = errors.New("hospital information system unavailable")
)

// normalize trims the criteria, and validates and defaults the page.
func normalize(c Criteria, p Page) (Criteria, Page, error) {
	c.NationalID = strings.TrimSpace(c.NationalID)
	c.PassportID = strings.TrimSpace(c.PassportID)
	c.FirstName = strings.TrimSpace(c.FirstName)
	c.MiddleName = strings.TrimSpace(c.MiddleName)
	c.LastName = strings.TrimSpace(c.LastName)
	c.DateOfBirth = strings.TrimSpace(c.DateOfBirth)
	c.PhoneNumber = strings.TrimSpace(c.PhoneNumber)
	c.Email = strings.TrimSpace(c.Email)

	if c.DateOfBirth != "" {
		if _, err := time.Parse(DateLayout, c.DateOfBirth); err != nil {
			return c, p, fmt.Errorf("%w: date_of_birth must be YYYY-MM-DD", ErrInvalid)
		}
	}
	switch {
	case p.Limit < 0:
		return c, p, fmt.Errorf("%w: limit must not be negative", ErrInvalid)
	case p.Limit == 0:
		p.Limit = DefaultLimit
	case p.Limit > MaxLimit:
		p.Limit = MaxLimit
	}
	if p.Offset < 0 {
		return c, p, fmt.Errorf("%w: offset must not be negative", ErrInvalid)
	}
	return c, p, nil
}
