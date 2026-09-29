package staff

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/auth"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	"github.com/DoIttikorn/hospital-middleware/internal/ids"
)

// Service is everything the staff domain can do. Handlers depend on it,
// never on a Repository directly.
type Service interface {
	// Create adds a staff member to a hospital. callerHospitalID is the
	// hospital of the logged-in staff member making the request; it must be
	// the hospital being created in.
	Create(ctx context.Context, callerHospitalID string, in Input) (Account, error)
	// Login checks the credentials and returns an access token.
	Login(ctx context.Context, in Input) (Token, error)
}

// Tokens issues the access tokens Login returns. *auth.Manager implements
// it.
type Tokens interface {
	Issue(staffID, hospitalID string) (string, error)
	TTL() time.Duration
}

// Hospitals is the part of the hospital domain staff uses: resolving the
// hospital code clients send. hospital.Service implements it.
type Hospitals interface {
	ByCode(ctx context.Context, code string) (hospital.Hospital, error)
}

var _ Hospitals = hospital.Service(nil)

type service struct {
	repo      Repository
	hospitals Hospitals
	tokens    Tokens
	now       func() time.Time
}

var _ Service = (*service)(nil)

// NewService wires the domain to its adapters.
func NewService(repo Repository, hospitals Hospitals, tokens Tokens) Service {
	return &service{repo: repo, hospitals: hospitals, tokens: tokens, now: now}
}

// now is truncated to milliseconds, the finest precision every supported
// store keeps.
func now() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

// dummyHash is compared against when the hospital or user does not exist, so
// a failed login takes about as long whatever the reason.
var dummyHash, _ = auth.HashPassword("dummy-password-for-timing")

func (s *service) Create(ctx context.Context, callerHospitalID string, in Input) (Account, error) {
	in = in.normalize()
	if err := in.validateCreate(); err != nil {
		return Account{}, err
	}
	h, err := s.hospitals.ByCode(ctx, in.Hospital)
	if err != nil {
		return Account{}, err // wraps hospital.ErrNotFound
	}
	if h.ID != callerHospitalID {
		return Account{}, fmt.Errorf("%w: cannot create staff in %q", ErrForbidden, h.Code)
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return Account{}, fmt.Errorf("hash password: %w", err)
	}
	t := s.now()
	st := Staff{ID: ids.New(), HospitalID: h.ID, Username: in.Username, PasswordHash: hash, CreatedAt: t, UpdatedAt: t}
	if err := s.repo.Insert(ctx, st); err != nil {
		return Account{}, err
	}
	return Account{ID: st.ID, Username: st.Username, Hospital: h.Code}, nil
}

func (s *service) Login(ctx context.Context, in Input) (Token, error) {
	in = in.normalize()
	if err := in.validateLogin(); err != nil {
		return Token{}, err
	}
	h, err := s.hospitals.ByCode(ctx, in.Hospital)
	if errors.Is(err, hospital.ErrNotFound) {
		auth.CheckPassword(dummyHash, in.Password)
		return Token{}, ErrInvalidCredentials
	}
	if err != nil {
		return Token{}, err
	}
	st, err := s.repo.ByHospitalAndUsername(ctx, h.ID, in.Username)
	if errors.Is(err, ErrNotFound) {
		auth.CheckPassword(dummyHash, in.Password)
		return Token{}, ErrInvalidCredentials
	}
	if err != nil {
		return Token{}, err
	}
	if !auth.CheckPassword(st.PasswordHash, in.Password) {
		return Token{}, ErrInvalidCredentials
	}
	tok, err := s.tokens.Issue(st.ID, st.HospitalID)
	if err != nil {
		return Token{}, fmt.Errorf("issue token: %w", err)
	}
	return Token{AccessToken: tok, TokenType: "Bearer", ExpiresIn: int(s.tokens.TTL().Seconds())}, nil
}
