// Package postgres is the PostgreSQL adapter for the staff domain.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/DoIttikorn/hospital-middleware/internal/staff"
)

// uniqueViolation is PostgreSQL's SQLSTATE for a unique constraint failure.
const uniqueViolation = "23505"

// Repository stores staff in the "staff" table.
type Repository struct {
	db *sql.DB
}

var _ staff.Repository = (*Repository)(nil)

// New uses the shared pool; it doesn't open connections of its own.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Insert(ctx context.Context, s staff.Staff) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO staff
		(id, hospital_id, username, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		s.ID, s.HospitalID, s.Username, s.PasswordHash, s.CreatedAt, s.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return fmt.Errorf("%w: %q", staff.ErrDuplicate, s.Username)
	}
	return err
}

func (r *Repository) ByHospitalAndUsername(ctx context.Context, hospitalID, username string) (staff.Staff, error) {
	var s staff.Staff
	err := r.db.QueryRowContext(ctx, `SELECT id::text, hospital_id::text, username, password_hash, created_at, updated_at
		FROM staff WHERE hospital_id = $1::uuid AND username = $2`, hospitalID, username).
		Scan(&s.ID, &s.HospitalID, &s.Username, &s.PasswordHash, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return staff.Staff{}, fmt.Errorf("%w: %q", staff.ErrNotFound, username)
	}
	if err != nil {
		return staff.Staff{}, err
	}
	s.CreatedAt, s.UpdatedAt = s.CreatedAt.UTC(), s.UpdatedAt.UTC()
	return s, nil
}
