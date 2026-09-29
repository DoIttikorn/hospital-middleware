package memory

import (
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/patient/patienttest"
)

func TestContract(t *testing.T) {
	patienttest.Run(t, func(t *testing.T) patienttest.Env {
		return patienttest.Env{Repo: New(), HospitalA: "hosp-a", HospitalB: "hosp-b"}
	})
}
