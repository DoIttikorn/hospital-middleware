package postgres

import (
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/database/dbtest"
	"github.com/DoIttikorn/hospital-middleware/internal/patient/patienttest"
)

// TestContract runs the Repository contract against a real PostgreSQL:
//
//	make deps-up && make test-integration
func TestContract(t *testing.T) {
	db := dbtest.Open(t)
	repo := New(db)
	patienttest.Run(t, func(t *testing.T) patienttest.Env {
		return patienttest.Env{Repo: repo, HospitalA: dbtest.NewHospital(t, db), HospitalB: dbtest.NewHospital(t, db)}
	})
}
