// Package memory is an in-process hospital.Repository, used as the test
// double for services and handlers.
package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
)

// Repository is safe for concurrent use.
type Repository struct {
	mu   sync.RWMutex
	byID map[string]hospital.Hospital
}

var _ hospital.Repository = (*Repository)(nil)

// New returns a repository holding hs.
func New(hs ...hospital.Hospital) *Repository {
	r := &Repository{byID: map[string]hospital.Hospital{}}
	for _, h := range hs {
		r.byID[h.ID] = h
	}
	return r
}

func (r *Repository) ByCode(ctx context.Context, code string) (hospital.Hospital, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, h := range r.byID {
		if h.Code == code {
			return h, nil
		}
	}
	return hospital.Hospital{}, fmt.Errorf("%w: code %q", hospital.ErrNotFound, code)
}

func (r *Repository) ByID(ctx context.Context, id string) (hospital.Hospital, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.byID[id]
	if !ok {
		return hospital.Hospital{}, fmt.Errorf("%w: id %q", hospital.ErrNotFound, id)
	}
	return h, nil
}
