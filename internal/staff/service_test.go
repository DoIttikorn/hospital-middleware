package staff_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/auth"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	hospitalmem "github.com/DoIttikorn/hospital-middleware/internal/hospital/memory"
	"github.com/DoIttikorn/hospital-middleware/internal/staff"
	staffmem "github.com/DoIttikorn/hospital-middleware/internal/staff/memory"
)

var (
	hospA = hospital.Hospital{ID: "id-a", Code: "hospital-a", Name: "Hospital A"}
	hospB = hospital.Hospital{ID: "id-b", Code: "hospital-b", Name: "Hospital B"}
)

type env struct {
	svc    staff.Service
	repo   *staffmem.Repository
	tokens *auth.Manager
}

func newEnv(t *testing.T) env {
	t.Helper()
	tokens, err := auth.NewManager("test-secret-at-least-16-bytes", 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	repo := staffmem.New()
	return env{svc: staff.NewService(repo, hospital.NewService(hospitalmem.New(hospA, hospB)), tokens), repo: repo, tokens: tokens}
}

func TestCreate(t *testing.T) {
	ctx := context.Background()
	valid := staff.Input{Username: "nurse01", Password: "s3cretPass!", Hospital: "hospital-a"}

	t.Run("creates staff in the caller's hospital", func(t *testing.T) {
		e := newEnv(t)
		acc, err := e.svc.Create(ctx, hospA.ID, valid)
		if err != nil {
			t.Fatal(err)
		}
		if acc.ID == "" || acc.Username != "nurse01" || acc.Hospital != "hospital-a" {
			t.Errorf("account = %+v", acc)
		}
	})

	t.Run("password is stored hashed", func(t *testing.T) {
		e := newEnv(t)
		if _, err := e.svc.Create(ctx, hospA.ID, valid); err != nil {
			t.Fatal(err)
		}
		st, err := e.repo.ByHospitalAndUsername(ctx, hospA.ID, "nurse01")
		if err != nil {
			t.Fatal(err)
		}
		if st.PasswordHash == valid.Password || !auth.CheckPassword(st.PasswordHash, valid.Password) {
			t.Errorf("stored hash %q does not hash the password", st.PasswordHash)
		}
	})

	t.Run("created staff can log in", func(t *testing.T) {
		e := newEnv(t)
		if _, err := e.svc.Create(ctx, hospA.ID, valid); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Login(ctx, valid); err != nil {
			t.Errorf("Login: %v", err)
		}
	})

	t.Run("trims username and hospital", func(t *testing.T) {
		e := newEnv(t)
		acc, err := e.svc.Create(ctx, hospA.ID, staff.Input{Username: "  nurse02 ", Password: "s3cretPass!", Hospital: " hospital-a "})
		if err != nil || acc.Username != "nurse02" {
			t.Errorf("Create = %+v, %v", acc, err)
		}
	})

	t.Run("same username in another hospital is allowed", func(t *testing.T) {
		e := newEnv(t)
		if _, err := e.svc.Create(ctx, hospA.ID, valid); err != nil {
			t.Fatal(err)
		}
		other := valid
		other.Hospital = "hospital-b"
		if _, err := e.svc.Create(ctx, hospB.ID, other); err != nil {
			t.Errorf("Create in hospital B: %v", err)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		tests := map[string]staff.Input{
			"missing username":  {Password: "s3cretPass!", Hospital: "hospital-a"},
			"blank username":    {Username: "   ", Password: "s3cretPass!", Hospital: "hospital-a"},
			"missing password":  {Username: "nurse01", Hospital: "hospital-a"},
			"missing hospital":  {Username: "nurse01", Password: "s3cretPass!"},
			"short username":    {Username: "ab", Password: "s3cretPass!", Hospital: "hospital-a"},
			"long username":     {Username: strings.Repeat("u", 101), Password: "s3cretPass!", Hospital: "hospital-a"},
			"short password":    {Username: "nurse01", Password: "short", Hospital: "hospital-a"},
			"password too long": {Username: "nurse01", Password: strings.Repeat("p", 73), Hospital: "hospital-a"},
		}
		for name, in := range tests {
			t.Run(name, func(t *testing.T) {
				if _, err := newEnv(t).svc.Create(ctx, hospA.ID, in); !errors.Is(err, staff.ErrInvalid) {
					t.Errorf("err = %v, want ErrInvalid", err)
				}
			})
		}
	})

	t.Run("unknown hospital", func(t *testing.T) {
		in := valid
		in.Hospital = "hospital-zzz"
		if _, err := newEnv(t).svc.Create(ctx, hospA.ID, in); !errors.Is(err, hospital.ErrNotFound) {
			t.Errorf("err = %v, want hospital.ErrNotFound", err)
		}
	})

	t.Run("other hospital than the caller's is forbidden", func(t *testing.T) {
		e := newEnv(t)
		in := valid
		in.Hospital = "hospital-b"
		if _, err := e.svc.Create(ctx, hospA.ID, in); !errors.Is(err, staff.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
		if _, err := e.repo.ByHospitalAndUsername(ctx, hospB.ID, "nurse01"); !errors.Is(err, staff.ErrNotFound) {
			t.Error("staff was created despite the error")
		}
	})

	t.Run("duplicate username in the same hospital", func(t *testing.T) {
		e := newEnv(t)
		if _, err := e.svc.Create(ctx, hospA.ID, valid); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Create(ctx, hospA.ID, valid); !errors.Is(err, staff.ErrDuplicate) {
			t.Errorf("err = %v, want ErrDuplicate", err)
		}
	})
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	creds := staff.Input{Username: "nurse01", Password: "s3cretPass!", Hospital: "hospital-a"}
	setup := func(t *testing.T) env {
		e := newEnv(t)
		if _, err := e.svc.Create(ctx, hospA.ID, creds); err != nil {
			t.Fatal(err)
		}
		return e
	}

	t.Run("returns a token carrying staff and hospital", func(t *testing.T) {
		e := setup(t)
		tok, err := e.svc.Login(ctx, creds)
		if err != nil {
			t.Fatal(err)
		}
		if tok.TokenType != "Bearer" || tok.ExpiresIn != 7200 {
			t.Errorf("token = %+v", tok)
		}
		id, err := e.tokens.Parse(tok.AccessToken)
		if err != nil {
			t.Fatal(err)
		}
		st, _ := e.repo.ByHospitalAndUsername(ctx, hospA.ID, "nurse01")
		if id.StaffID != st.ID || id.HospitalID != hospA.ID {
			t.Errorf("identity = %+v, want staff %s in %s", id, st.ID, hospA.ID)
		}
	})

	t.Run("invalid credentials all look the same", func(t *testing.T) {
		e := setup(t)
		tests := map[string]staff.Input{
			"wrong password":   {Username: "nurse01", Password: "wrong-password", Hospital: "hospital-a"},
			"unknown user":     {Username: "ghost", Password: "s3cretPass!", Hospital: "hospital-a"},
			"unknown hospital": {Username: "nurse01", Password: "s3cretPass!", Hospital: "hospital-zzz"},
			"other hospital":   {Username: "nurse01", Password: "s3cretPass!", Hospital: "hospital-b"},
		}
		for name, in := range tests {
			t.Run(name, func(t *testing.T) {
				if _, err := e.svc.Login(ctx, in); !errors.Is(err, staff.ErrInvalidCredentials) {
					t.Errorf("err = %v, want ErrInvalidCredentials", err)
				}
			})
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		e := setup(t)
		tests := map[string]staff.Input{
			"no username": {Password: "x", Hospital: "hospital-a"},
			"no password": {Username: "nurse01", Hospital: "hospital-a"},
			"no hospital": {Username: "nurse01", Password: "x"},
		}
		for name, in := range tests {
			t.Run(name, func(t *testing.T) {
				if _, err := e.svc.Login(ctx, in); !errors.Is(err, staff.ErrInvalid) {
					t.Errorf("err = %v, want ErrInvalid", err)
				}
			})
		}
	})
}
