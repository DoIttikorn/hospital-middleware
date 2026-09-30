package patient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	"github.com/DoIttikorn/hospital-middleware/internal/ids"
)

// Service is everything the patient domain can do. Handlers depend on it,
// never on a Repository directly.
type Service interface {
	// Search returns the hospital's patients matching c, one page at a time.
	// With no criteria it lists the hospital's patients.
	Search(ctx context.Context, hospitalID string, c Criteria, p Page) (Result, error)
}

// Replaced by hospital.Service, taken directly to keep things simple. This
// was the narrow interface patient declared for the one method it uses
// (looking up the staff's hospital to find its HIS):
//
// type Hospitals interface {
// 	ByID(ctx context.Context, id string) (hospital.Hospital, error)
// }
//
// var _ Hospitals = hospital.Service(nil)

type service struct {
	repo      Repository
	hospitals hospital.Service
	his       HISRegistry
	now       func() time.Time
}

var _ Service = (*service)(nil)

// NewService wires the domain to its adapters.
func NewService(repo Repository, hospitals hospital.Service, his HISRegistry) Service {
	return &service{repo: repo, hospitals: hospitals, his: his, now: now}
}

// now is truncated to milliseconds, the finest precision every supported
// store keeps.
func now() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

func (s *service) Search(ctx context.Context, hospitalID string, c Criteria, p Page) (Result, error) {
	c, p, err := normalize(c, p)
	if err != nil {
		return Result{}, err
	}
	found, total, err := s.repo.Search(ctx, hospitalID, c, p)
	if err != nil {
		return Result{}, err
	}
	// A patient we have never seen may still exist in the HIS. Only an
	// identifier lets us ask it, and only the first page can be empty
	// because of that.
	if total == 0 && p.Offset == 0 && (c.NationalID != "" || c.PassportID != "") {
		imported, err := s.importFromHIS(ctx, hospitalID, c)
		if err != nil {
			return Result{}, err
		}
		if imported {
			// Search again so the other criteria are applied to the
			// imported record too.
			if found, total, err = s.repo.Search(ctx, hospitalID, c, p); err != nil {
				return Result{}, err
			}
		}
	}
	return Result{Total: total, Limit: p.Limit, Offset: p.Offset, Data: found}, nil
}

// importFromHIS looks the identifier up in the hospital's HIS and stores the
// patient it finds. It reports whether a patient was stored; a hospital
// without a HIS, or a HIS without the patient, is not an error.
func (s *service) importFromHIS(ctx context.Context, hospitalID string, c Criteria) (bool, error) {
	h, err := s.hospitals.ByID(ctx, hospitalID)
	if err != nil {
		return false, fmt.Errorf("hospital of staff: %w", err)
	}
	client, ok := s.his.For(h)
	if !ok {
		return false, nil
	}
	id, isNational := c.NationalID, true
	if id == "" {
		id, isNational = c.PassportID, false
	}
	p, err := client.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if p.PatientHN == "" {
		return false, fmt.Errorf("%w: response has no patient_hn", ErrHISUnavailable)
	}
	// The record must be findable by the identifier it was asked for, and
	// the store requires at least one identifier.
	if isNational && p.NationalID == nil {
		p.NationalID = &id
	}
	if !isNational && p.PassportID == nil {
		p.PassportID = &id
	}
	t := s.now()
	p.ID, p.HospitalID, p.CreatedAt, p.UpdatedAt = ids.New(), hospitalID, t, t
	if _, err := s.repo.Upsert(ctx, p); err != nil {
		return false, err
	}
	return true, nil
}
