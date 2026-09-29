package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/DoIttikorn/hospital-middleware/internal/auth"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	"github.com/DoIttikorn/hospital-middleware/internal/staff"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// fake is a Service stub that records what it was called with.
type fake struct {
	createErr, loginErr error
	gotCaller           string
	gotInput            staff.Input
}

func (f *fake) Create(_ context.Context, caller string, in staff.Input) (staff.Account, error) {
	f.gotCaller, f.gotInput = caller, in
	if f.createErr != nil {
		return staff.Account{}, f.createErr
	}
	return staff.Account{ID: "new-id", Username: in.Username, Hospital: in.Hospital}, nil
}

func (f *fake) Login(_ context.Context, in staff.Input) (staff.Token, error) {
	f.gotInput = in
	if f.loginErr != nil {
		return staff.Token{}, f.loginErr
	}
	return staff.Token{AccessToken: "jwt", TokenType: "Bearer", ExpiresIn: 3600}, nil
}

type harness struct {
	router *gin.Engine
	fake   *fake
	token  string
}

func newHarness(t *testing.T) harness {
	t.Helper()
	mgr, err := auth.NewManager("test-secret-at-least-16-bytes", 3600e9)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := mgr.Issue("caller-staff", "caller-hospital")
	f := &fake{}
	r := gin.New()
	New(f, slog.New(slog.DiscardHandler)).Register(r.Group(""), r.Group("", mgr.Middleware()))
	return harness{router: r, fake: f, token: token}
}

func (h harness) do(method, path, body, token string) (*httptest.ResponseRecorder, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

const validBody = `{"username":"nurse01","password":"s3cretPass!","hospital":"hospital-a"}`

func TestCreate(t *testing.T) {
	t.Run("201 with the new account, using the caller's hospital", func(t *testing.T) {
		h := newHarness(t)
		rec, body := h.do(http.MethodPost, "/staff/create", validBody, h.token)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		if body["id"] != "new-id" || body["username"] != "nurse01" || body["hospital"] != "hospital-a" {
			t.Errorf("body = %v", body)
		}
		if _, leaked := body["password"]; leaked {
			t.Error("response contains the password")
		}
		if h.fake.gotCaller != "caller-hospital" || h.fake.gotInput.Username != "nurse01" {
			t.Errorf("service saw caller=%q input=%+v", h.fake.gotCaller, h.fake.gotInput)
		}
	})

	t.Run("requires a token", func(t *testing.T) {
		h := newHarness(t)
		for name, token := range map[string]string{"none": "", "garbage": "garbage"} {
			if rec, _ := h.do(http.MethodPost, "/staff/create", validBody, token); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s token: status = %d, want 401", name, rec.Code)
			}
		}
		if h.fake.gotInput != (staff.Input{}) {
			t.Error("service called without authentication")
		}
	})

	errorCases := []struct {
		name string
		err  error
		want int
		code string
	}{
		{"validation", fmt.Errorf("%w: username is required", staff.ErrInvalid), http.StatusBadRequest, "validation_error"},
		{"other hospital", fmt.Errorf("%w: nope", staff.ErrForbidden), http.StatusForbidden, "hospital_mismatch"},
		{"unknown hospital", fmt.Errorf("%w: code x", hospital.ErrNotFound), http.StatusNotFound, "hospital_not_found"},
		{"duplicate", fmt.Errorf("%w: nurse01", staff.ErrDuplicate), http.StatusConflict, "username_taken"},
		{"unexpected", errors.New("db exploded: secret detail"), http.StatusInternalServerError, ""},
	}
	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.fake.createErr = tt.err
			rec, body := h.do(http.MethodPost, "/staff/create", validBody, h.token)
			if rec.Code != tt.want || body["code"] != codeOrNil(tt.code) {
				t.Errorf("status = %d code = %v, want %d %q: %s", rec.Code, body["code"], tt.want, tt.code, rec.Body)
			}
			if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/problem+json") {
				t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
			}
			if tt.want == http.StatusInternalServerError && strings.Contains(rec.Body.String(), "secret detail") {
				t.Error("internal error detail leaked to the client")
			}
		})
	}

	t.Run("malformed body", func(t *testing.T) {
		for name, body := range map[string]string{
			"not json":      `nope`,
			"unknown field": `{"username":"a","password":"b","hospital":"c","admin":true}`,
			"empty":         ``,
		} {
			h := newHarness(t)
			if rec, out := h.do(http.MethodPost, "/staff/create", body, h.token); rec.Code != http.StatusBadRequest || out["code"] != "bad_request" {
				t.Errorf("%s: status = %d body = %s, want 400 bad_request", name, rec.Code, rec.Body)
			}
		}
	})
}

func TestLogin(t *testing.T) {
	t.Run("200 with a token, without authentication", func(t *testing.T) {
		h := newHarness(t)
		rec, body := h.do(http.MethodPost, "/staff/login", validBody, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body)
		}
		if body["token"] != "jwt" || body["token_type"] != "Bearer" || body["expires_in"] != float64(3600) {
			t.Errorf("body = %v", body)
		}
		if h.fake.gotInput.Hospital != "hospital-a" || h.fake.gotInput.Password != "s3cretPass!" {
			t.Errorf("service saw %+v", h.fake.gotInput)
		}
	})

	errorCases := []struct {
		name string
		err  error
		want int
		code string
	}{
		{"validation", fmt.Errorf("%w: password is required", staff.ErrInvalid), http.StatusBadRequest, "validation_error"},
		{"invalid credentials", staff.ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials"},
		{"unexpected", errors.New("db exploded"), http.StatusInternalServerError, ""},
	}
	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.fake.loginErr = tt.err
			rec, body := h.do(http.MethodPost, "/staff/login", validBody, "")
			if rec.Code != tt.want || body["code"] != codeOrNil(tt.code) {
				t.Errorf("status = %d code = %v, want %d %q", rec.Code, body["code"], tt.want, tt.code)
			}
		})
	}

	t.Run("malformed body", func(t *testing.T) {
		h := newHarness(t)
		if rec, _ := h.do(http.MethodPost, "/staff/login", `{`, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}

// codeOrNil maps "no code" to the nil a missing JSON member decodes to.
func codeOrNil(code string) any {
	if code == "" {
		return nil
	}
	return code
}
