package hospital

import "context"

// Service is everything the hospital domain offers. Other domains (staff,
// patient) take it directly, never a Repository.
type Service interface {
	// ByCode returns the hospital with the code, or an error wrapping
	// ErrNotFound.
	ByCode(ctx context.Context, code string) (Hospital, error)
	// ByID returns the hospital with the ID, or an error wrapping
	// ErrNotFound.
	ByID(ctx context.Context, id string) (Hospital, error)
}

type service struct {
	repo Repository
}

var _ Service = (*service)(nil)

// NewService wires the domain to its adapter.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ByCode(ctx context.Context, code string) (Hospital, error) {
	return s.repo.ByCode(ctx, code)
}

func (s *service) ByID(ctx context.Context, id string) (Hospital, error) {
	return s.repo.ByID(ctx, id)
}
