// Package auth issues and verifies the JWTs staff log in with, hashes
// passwords, and provides the Gin middleware that protects routes.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// MinSecretLength is the shortest JWT secret NewManager accepts, in bytes.
const MinSecretLength = 16

// ErrInvalidToken is returned by Parse for any token that must not be
// trusted: malformed, wrongly signed, expired, or missing its claims.
var ErrInvalidToken = errors.New("invalid token")

// Identity is who a valid token belongs to. HospitalID is what scopes every
// patient query, so it always comes from the token, never from the request.
type Identity struct {
	StaffID    string
	HospitalID string
}

type claims struct {
	HospitalID string `json:"hid"`
	jwt.RegisteredClaims
}

// Manager signs and verifies HS256 tokens.
type Manager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewManager returns a Manager for tokens that live for ttl.
func NewManager(secret string, ttl time.Duration) (*Manager, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("JWT secret must be at least %d bytes", MinSecretLength)
	}
	if ttl <= 0 {
		return nil, errors.New("token TTL must be positive")
	}
	return &Manager{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

// TTL is how long an issued token is valid.
func (m *Manager) TTL() time.Duration { return m.ttl }

// Issue returns a signed token for the staff member.
func (m *Manager) Issue(staffID, hospitalID string) (string, error) {
	now := m.now()
	c := claims{
		HospitalID: hospitalID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   staffID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.secret)
}

// Parse verifies token and returns its identity. Errors wrap
// ErrInvalidToken.
func (m *Manager) Parse(token string) (Identity, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if c.Subject == "" || c.HospitalID == "" {
		return Identity{}, fmt.Errorf("%w: missing claims", ErrInvalidToken)
	}
	return Identity{StaffID: c.Subject, HospitalID: c.HospitalID}, nil
}

// MaxPasswordLength is bcrypt's limit; longer passwords are rejected rather
// than silently truncated.
const MaxPasswordLength = 72

// HashPassword returns the bcrypt hash of password.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

// CheckPassword reports whether password matches hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
