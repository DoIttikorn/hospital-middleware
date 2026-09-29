// Package staff is the hospital staff domain: creating staff accounts and
// logging them in. It is laid out like every domain package:
//
//   - this package holds the model, the business rules (Service) and the
//     Repository port; it imports no database driver and no web framework;
//   - adapters live in sub-packages: memory (in-process, also the test
//     double), postgres (the real store) and handler (the REST endpoints);
//   - stafftest is the contract every Repository must pass.
package staff

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/auth"
)

// Staff is a stored staff account. A staff member belongs to exactly one
// hospital, which scopes everything they can see.
type Staff struct {
	ID           string
	HospitalID   string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Account is the public view of a created staff member.
type Account struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Hospital string `json:"hospital"` // hospital code
}

// Input is what a client sends to create a staff member or to log in.
type Input struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Hospital string `json:"hospital"` // hospital code
}

// Token is the result of a successful login.
type Token struct {
	AccessToken string `json:"token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // seconds
}

// Field limits.
const (
	MinUsernameLength = 3
	MaxUsernameLength = 100
	MinPasswordLength = 8
)

var (
	// ErrInvalid wraps validation errors in the input.
	ErrInvalid = errors.New("invalid staff")
	// ErrNotFound is returned when no staff member matches.
	ErrNotFound = errors.New("staff not found")
	// ErrDuplicate is returned when the username is taken in that hospital.
	ErrDuplicate = errors.New("username already exists in this hospital")
	// ErrInvalidCredentials is returned by Login for an unknown hospital,
	// unknown user or wrong password alike, so callers can't tell which.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrForbidden is returned when a staff member tries to act on a
	// hospital other than their own.
	ErrForbidden = errors.New("staff can only manage their own hospital")
)

// normalize trims the identifying fields; the password is left untouched.
func (in Input) normalize() Input {
	in.Username = strings.TrimSpace(in.Username)
	in.Hospital = strings.TrimSpace(in.Hospital)
	return in
}

// validateLogin checks that every field is present.
func (in Input) validateLogin() error {
	switch {
	case in.Username == "":
		return fmt.Errorf("%w: username is required", ErrInvalid)
	case in.Password == "":
		return fmt.Errorf("%w: password is required", ErrInvalid)
	case in.Hospital == "":
		return fmt.Errorf("%w: hospital is required", ErrInvalid)
	}
	return nil
}

// validateCreate applies the stricter rules for a new account.
func (in Input) validateCreate() error {
	if err := in.validateLogin(); err != nil {
		return err
	}
	switch {
	case len(in.Username) < MinUsernameLength || len(in.Username) > MaxUsernameLength:
		return fmt.Errorf("%w: username must be %d-%d bytes", ErrInvalid, MinUsernameLength, MaxUsernameLength)
	case len(in.Password) < MinPasswordLength:
		return fmt.Errorf("%w: password must be at least %d bytes", ErrInvalid, MinPasswordLength)
	case len(in.Password) > auth.MaxPasswordLength:
		return fmt.Errorf("%w: password must be at most %d bytes", ErrInvalid, auth.MaxPasswordLength)
	}
	return nil
}
