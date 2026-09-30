// Package postgres is the PostgreSQL adapter for the hospital domain.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
)

// Repository reads the "hospitals" table.
type Repository struct {
	db *sql.DB
}

var _ hospital.Repository = (*Repository)(nil)

// New uses the shared pool; it doesn't open connections of its own.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

const selectHospital = `SELECT id::text, code, name, COALESCE(his_base_url, ''), created_at FROM hospitals`

func (r *Repository) ByCode(ctx context.Context, code string) (hospital.Hospital, error) {
	h, err := scan(r.db.QueryRowContext(ctx, selectHospital+` WHERE code = $1`, code))
	if errors.Is(err, sql.ErrNoRows) {
		return hospital.Hospital{}, fmt.Errorf("%w: code %q", hospital.ErrNotFound, code)
	}
	return h, err
}

func (r *Repository) ByID(ctx context.Context, id string) (hospital.Hospital, error) {
	h, err := scan(r.db.QueryRowContext(ctx, selectHospital+` WHERE id = $1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return hospital.Hospital{}, fmt.Errorf("%w: id %q", hospital.ErrNotFound, id)
	}
	return h, err
}

func scan(row *sql.Row) (hospital.Hospital, error) {
	var h hospital.Hospital
	if err := row.Scan(&h.ID, &h.Code, &h.Name, &h.HISBaseURL, &h.CreatedAt); err != nil {
		return hospital.Hospital{}, err
	}
	h.CreatedAt = h.CreatedAt.UTC()
	return h, nil
}
