package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const secret = "test-secret-at-least-16-bytes"

func newManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(secret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestNewManager(t *testing.T) {
	if _, err := NewManager("short", time.Hour); err == nil {
		t.Error("short secret accepted")
	}
	if _, err := NewManager(secret, 0); err == nil {
		t.Error("zero TTL accepted")
	}
	if m, err := NewManager(secret, 90*time.Minute); err != nil || m.TTL() != 90*time.Minute {
		t.Errorf("NewManager = %v, %v", m, err)
	}
}

func TestIssueAndParse(t *testing.T) {
	m := newManager(t)
	tok, err := m.Issue("staff-1", "hosp-1")
	if err != nil {
		t.Fatal(err)
	}
	id, err := m.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if id != (Identity{StaffID: "staff-1", HospitalID: "hosp-1"}) {
		t.Errorf("identity = %+v", id)
	}
}

func TestParseRejects(t *testing.T) {
	m := newManager(t)
	good, _ := m.Issue("s", "h")

	expired := newManager(t)
	expired.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	expiredTok, _ := expired.Issue("s", "h")

	other, _ := NewManager("another-secret-16-bytes!", time.Hour)
	otherTok, _ := other.Issue("s", "h")

	// alg=none must never be accepted.
	noneTok, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims{
		HospitalID:       "h",
		RegisteredClaims: jwt.RegisteredClaims{Subject: "s", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	// Signed correctly but without an expiry.
	noExpTok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		HospitalID: "h", RegisteredClaims: jwt.RegisteredClaims{Subject: "s"},
	}).SignedString([]byte(secret))

	// Signed correctly but without a hospital.
	noHospTok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "s", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))

	tests := map[string]string{
		"empty":           "",
		"garbage":         "not-a-jwt",
		"expired":         expiredTok,
		"wrong signature": otherTok,
		"tampered":        good[:len(good)-2] + "xx",
		"alg none":        noneTok,
		"no expiry":       noExpTok,
		"no hospital":     noHospTok,
	}
	for name, tok := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := m.Parse(tok); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("Parse err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestPasswordHash(t *testing.T) {
	h, err := HashPassword("s3cretPass!")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(h, "s3cretPass") {
		t.Error("hash contains the password")
	}
	if !CheckPassword(h, "s3cretPass!") {
		t.Error("correct password rejected")
	}
	if CheckPassword(h, "wrong") || CheckPassword("not-a-hash", "s3cretPass!") {
		t.Error("wrong password or hash accepted")
	}
}
