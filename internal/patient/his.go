package patient

import (
	"context"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
)

// HISClient reads patients from one hospital's HIS.
type HISClient interface {
	// FindByID looks a patient up by national ID or passport ID. It returns
	// an error wrapping ErrNotFound when the HIS has no such patient, and
	// one wrapping ErrHISUnavailable when it cannot answer.
	FindByID(ctx context.Context, id string) (Patient, error)
}

// HISRegistry picks the HIS client for a hospital. It reports false when the
// hospital has no HIS integration.
type HISRegistry interface {
	For(h hospital.Hospital) (HISClient, bool)
}
