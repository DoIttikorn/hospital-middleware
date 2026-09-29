package postgres

import (
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/database/dbtest"
	"github.com/DoIttikorn/hospital-middleware/internal/staff/stafftest"
)

// TestContract runs the Repository contract against a real PostgreSQL:
//
//	make deps-up && make test-integration
func TestContract(t *testing.T) {
	db := dbtest.Open(t)
	repo := New(db)
	stafftest.Run(t, func(t *testing.T) stafftest.Env {
		return stafftest.Env{Repo: repo, HospitalA: dbtest.NewHospital(t, db), HospitalB: dbtest.NewHospital(t, db)}
	})
}
