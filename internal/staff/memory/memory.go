// Package memory is an in-process staff.Repository. It is the test double
// for the service and handlers: it passes the same contract as the database
// adapter. Everything is lost on restart, so it is not for production.
package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/DoIttikorn/hospital-middleware/internal/staff"
)

type key struct{ hospitalID, username string }

// Repository is safe for concurrent use.
type Repository struct {
	mu    sync.RWMutex
	staff map[key]staff.Staff
}

var _ staff.Repository = (*Repository)(nil)

func New() *Repository { return &Repository{staff: map[key]staff.Staff{}} }

func (r *Repository) Insert(ctx context.Context, s staff.Staff) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key{s.HospitalID, s.Username}
	if _, ok := r.staff[k]; ok {
		return fmt.Errorf("%w: %q", staff.ErrDuplicate, s.Username)
	}
	r.staff[k] = s
	return nil
}

func (r *Repository) ByHospitalAndUsername(ctx context.Context, hospitalID, username string) (staff.Staff, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.staff[key{hospitalID, username}]
	if !ok {
		return staff.Staff{}, fmt.Errorf("%w: %q", staff.ErrNotFound, username)
	}
	return s, nil
}
