package postgres

import (
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/database/dbtest"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital/hospitaltest"
)

// TestContract runs the Repository contract against the seeded hospitals
// (migration 000002): make deps-up && make test-integration
func TestContract(t *testing.T) {
	db := dbtest.Open(t)
	hospitaltest.Run(t, New(db), hospital.Hospital{
		ID:         "a0000000-0000-4000-8000-000000000001",
		Code:       "hospital-a",
		Name:       "Hospital A",
		HISBaseURL: "https://hospital-a.api.co.th",
	})
}
