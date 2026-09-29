package hospital_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	hospitalmem "github.com/DoIttikorn/hospital-middleware/internal/hospital/memory"
)

var hospA = hospital.Hospital{ID: "id-a", Code: "hospital-a", Name: "Hospital A", HISBaseURL: "https://a"}

func TestService(t *testing.T) {
	ctx := context.Background()
	svc := hospital.NewService(hospitalmem.New(hospA))

	t.Run("by code", func(t *testing.T) {
		got, err := svc.ByCode(ctx, "hospital-a")
		if err != nil || got != hospA {
			t.Errorf("got %+v, %v; want %+v", got, err, hospA)
		}
	})
	t.Run("by id", func(t *testing.T) {
		got, err := svc.ByID(ctx, "id-a")
		if err != nil || got != hospA {
			t.Errorf("got %+v, %v; want %+v", got, err, hospA)
		}
	})
	t.Run("unknown code is ErrNotFound", func(t *testing.T) {
		if _, err := svc.ByCode(ctx, "hospital-x"); !errors.Is(err, hospital.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		if _, err := svc.ByID(ctx, "id-x"); !errors.Is(err, hospital.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
