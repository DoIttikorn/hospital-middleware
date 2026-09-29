package memory

import (
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/staff/stafftest"
)

func TestContract(t *testing.T) {
	stafftest.Run(t, func(t *testing.T) stafftest.Env {
		return stafftest.Env{Repo: New(), HospitalA: "hosp-a", HospitalB: "hosp-b"}
	})
}
