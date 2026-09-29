// Package hospitaltest is the contract every hospital.Repository must pass.
package hospitaltest

import (
	"context"
	"errors"
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
)

// Run checks the Repository contract against repo, which must contain known
// and must not contain a hospital with code "no-such-hospital" or the ID
// "00000000-0000-4000-8000-00000000dead".
func Run(t *testing.T, repo hospital.Repository, known hospital.Hospital) {
	ctx := context.Background()
	same := func(t *testing.T, got hospital.Hospital) {
		t.Helper()
		if got.ID != known.ID || got.Code != known.Code || got.Name != known.Name || got.HISBaseURL != known.HISBaseURL {
			t.Errorf("got %+v, want %+v", got, known)
		}
	}

	t.Run("by code", func(t *testing.T) {
		got, err := repo.ByCode(ctx, known.Code)
		if err != nil {
			t.Fatal(err)
		}
		same(t, got)
	})
	t.Run("by id", func(t *testing.T) {
		got, err := repo.ByID(ctx, known.ID)
		if err != nil {
			t.Fatal(err)
		}
		same(t, got)
	})
	t.Run("missing code is ErrNotFound", func(t *testing.T) {
		if _, err := repo.ByCode(ctx, "no-such-hospital"); !errors.Is(err, hospital.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("missing id is ErrNotFound", func(t *testing.T) {
		if _, err := repo.ByID(ctx, "00000000-0000-4000-8000-00000000dead"); !errors.Is(err, hospital.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
