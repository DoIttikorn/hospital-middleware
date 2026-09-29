// Package stafftest is the contract every staff.Repository must pass.
//
// The service is tested against the memory adapter, which is only safe if
// every other adapter behaves the same way. Each adapter's tests call Run:
// memory in ordinary unit tests, postgres against a real database
// (INTEGRATION=1).
package stafftest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/ids"
	"github.com/DoIttikorn/hospital-middleware/internal/staff"
)

// Env is a fresh repository and two hospitals it may hold staff for.
type Env struct {
	Repo                 staff.Repository
	HospitalA, HospitalB string
}

// Run checks the Repository contract. newEnv must return an empty
// repository, and hospitals that exist in the store, on every call.
func Run(t *testing.T, newEnv func(t *testing.T) Env) {
	ctx := context.Background()
	at := time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC)
	member := func(hospitalID, username string) staff.Staff {
		return staff.Staff{ID: ids.New(), HospitalID: hospitalID, Username: username,
			PasswordHash: "hash-of-" + username, CreatedAt: at, UpdatedAt: at}
	}

	t.Run("insert then get", func(t *testing.T) {
		e := newEnv(t)
		want := member(e.HospitalA, "nurse01")
		if err := e.Repo.Insert(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := e.Repo.ByHospitalAndUsername(ctx, e.HospitalA, "nurse01")
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != want.ID || got.HospitalID != want.HospitalID || got.Username != want.Username ||
			got.PasswordHash != want.PasswordHash || !got.CreatedAt.Equal(at) || !got.UpdatedAt.Equal(at) {
			t.Errorf("got %+v\nwant %+v", got, want)
		}
	})

	t.Run("get missing is ErrNotFound", func(t *testing.T) {
		e := newEnv(t)
		if _, err := e.Repo.ByHospitalAndUsername(ctx, e.HospitalA, "ghost"); !errors.Is(err, staff.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("duplicate username in one hospital is ErrDuplicate", func(t *testing.T) {
		e := newEnv(t)
		if err := e.Repo.Insert(ctx, member(e.HospitalA, "nurse01")); err != nil {
			t.Fatal(err)
		}
		if err := e.Repo.Insert(ctx, member(e.HospitalA, "nurse01")); !errors.Is(err, staff.ErrDuplicate) {
			t.Errorf("err = %v, want ErrDuplicate", err)
		}
	})

	t.Run("same username in another hospital is allowed", func(t *testing.T) {
		e := newEnv(t)
		a, b := member(e.HospitalA, "nurse01"), member(e.HospitalB, "nurse01")
		if err := e.Repo.Insert(ctx, a); err != nil {
			t.Fatal(err)
		}
		if err := e.Repo.Insert(ctx, b); err != nil {
			t.Fatalf("second hospital: %v", err)
		}
		got, err := e.Repo.ByHospitalAndUsername(ctx, e.HospitalB, "nurse01")
		if err != nil || got.ID != b.ID {
			t.Errorf("got %+v, %v; want staff %s", got, err, b.ID)
		}
	})

	t.Run("lookup is scoped to the hospital", func(t *testing.T) {
		e := newEnv(t)
		if err := e.Repo.Insert(ctx, member(e.HospitalA, "nurse01")); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Repo.ByHospitalAndUsername(ctx, e.HospitalB, "nurse01"); !errors.Is(err, staff.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
