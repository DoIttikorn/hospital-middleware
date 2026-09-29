package memory

import (
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital/hospitaltest"
)

func TestContract(t *testing.T) {
	known := hospital.Hospital{ID: "h1", Code: "hospital-x", Name: "Hospital X", HISBaseURL: "https://x.example"}
	hospitaltest.Run(t, New(known), known)
}
