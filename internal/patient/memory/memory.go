// Package memory is an in-process patient.Repository. It is the test double
// for the service and handlers: it passes the same contract as the database
// adapter. Everything is lost on restart, so it is not for production.
package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

// Repository is safe for concurrent use.
type Repository struct {
	mu       sync.RWMutex
	patients []patient.Patient
}

var _ patient.Repository = (*Repository)(nil)

func New() *Repository { return &Repository{} }

func (r *Repository) Search(ctx context.Context, hospitalID string, c patient.Criteria, p patient.Page) ([]patient.Patient, int, error) {
	r.mu.RLock()
	var matches []patient.Patient
	for _, pt := range r.patients {
		if pt.HospitalID == hospitalID && matchesCriteria(pt, c) {
			matches = append(matches, pt)
		}
	}
	r.mu.RUnlock()

	slices.SortFunc(matches, func(a, b patient.Patient) int {
		return cmp.Or(strings.Compare(a.PatientHN, b.PatientHN), strings.Compare(a.ID, b.ID))
	})
	total := len(matches)
	start := min(p.Offset, total)
	end := total
	if p.Limit > 0 {
		end = min(start+p.Limit, total)
	}
	page := make([]patient.Patient, 0, end-start)
	page = append(page, matches[start:end]...)
	return page, total, nil
}

func (r *Repository) Upsert(ctx context.Context, p patient.Patient) (patient.Patient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, cur := range r.patients {
		if cur.HospitalID == p.HospitalID && cur.PatientHN == p.PatientHN {
			p.ID, p.CreatedAt = cur.ID, cur.CreatedAt
			r.patients[i] = p
			return p, nil
		}
	}
	r.patients = append(r.patients, p)
	return p, nil
}

// matchesCriteria applies the criteria documented on patient.Criteria.
func matchesCriteria(p patient.Patient, c patient.Criteria) bool {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	prefix := func(want, th, en string) bool {
		if want == "" {
			return true
		}
		want = strings.ToLower(want)
		return strings.HasPrefix(strings.ToLower(th), want) || strings.HasPrefix(strings.ToLower(en), want)
	}
	return (c.NationalID == "" || deref(p.NationalID) == c.NationalID) &&
		(c.PassportID == "" || deref(p.PassportID) == c.PassportID) &&
		prefix(c.FirstName, p.FirstNameTH, p.FirstNameEN) &&
		prefix(c.MiddleName, p.MiddleNameTH, p.MiddleNameEN) &&
		prefix(c.LastName, p.LastNameTH, p.LastNameEN) &&
		(c.DateOfBirth == "" || p.DateOfBirth == c.DateOfBirth) &&
		(c.PhoneNumber == "" || p.PhoneNumber == c.PhoneNumber) &&
		(c.Email == "" || strings.EqualFold(p.Email, c.Email))
}
