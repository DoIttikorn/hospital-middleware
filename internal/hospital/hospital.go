// Package hospital is the master data of the hospitals the middleware
// serves. Staff and patients belong to exactly one hospital.
//
// Like every domain package it holds the model, the Service and the
// Repository port; the adapters are in the memory and postgres sub-packages,
// and hospitaltest is the contract they both pass. Other domains use the
// Service, each through a narrow interface of its own.
package hospital

import (
	"context"
	"errors"
	"time"
)

// Hospital is one hospital.
type Hospital struct {
	ID   string
	Code string // stable public identifier, e.g. "hospital-a"
	Name string
	// HISBaseURL is where the hospital's HIS is reached; empty when there is
	// no HIS integration.
	HISBaseURL string
	CreatedAt  time.Time
}

// ErrNotFound is returned when no hospital matches.
var ErrNotFound = errors.New("hospital not found")

// Repository is the persistence port for hospitals (read-only: hospitals
// are master data managed by migrations).
type Repository interface {
	// ByCode returns the hospital with the code, or an error wrapping
	// ErrNotFound.
	ByCode(ctx context.Context, code string) (Hospital, error)
	// ByID returns the hospital with the ID, or an error wrapping
	// ErrNotFound.
	ByID(ctx context.Context, id string) (Hospital, error)
}
