// Package postgres is the PostgreSQL adapter for the patient domain.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

// Repository stores patients in the "patients" table.
type Repository struct {
	db *sql.DB
}

var _ patient.Repository = (*Repository)(nil)

// New uses the shared pool; it doesn't open connections of its own.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

// columns are selected in the order scan reads them. Nullable text columns
// come back as "" (the API has no null names), the identifiers stay NULL-able.
const columns = `id::text, hospital_id::text, patient_hn, national_id, passport_id,
	COALESCE(first_name_th, ''), COALESCE(middle_name_th, ''), COALESCE(last_name_th, ''),
	COALESCE(first_name_en, ''), COALESCE(middle_name_en, ''), COALESCE(last_name_en, ''),
	COALESCE(to_char(date_of_birth, 'YYYY-MM-DD'), ''), COALESCE(phone_number, ''),
	COALESCE(email, ''), COALESCE(gender, ''), created_at, updated_at`

func (r *Repository) Search(ctx context.Context, hospitalID string, c patient.Criteria, p patient.Page) ([]patient.Patient, int, error) {
	where, args := buildWhere(hospitalID, c)

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM patients WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	out := []patient.Patient{}
	if total == 0 {
		return out, 0, nil
	}

	query := `SELECT ` + columns + ` FROM patients WHERE ` + where + ` ORDER BY patient_hn, id`
	if p.Limit > 0 {
		args = append(args, p.Limit)
		query += ` LIMIT $` + strconv.Itoa(len(args))
	}
	args = append(args, p.Offset)
	query += ` OFFSET $` + strconv.Itoa(len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		pt, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, pt)
	}
	return out, total, rows.Err()
}

// buildWhere turns the criteria into a WHERE clause over $n placeholders.
// Values only ever travel as arguments, never inside the SQL text.
func buildWhere(hospitalID string, c patient.Criteria) (string, []any) {
	conds := []string{"hospital_id::text = $1"}
	args := []any{hospitalID}
	// add appends a condition in which every "?" stands for the same value.
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	// name matches a case-insensitive prefix of the Thai or English column.
	name := func(value, th, en string) {
		if value != "" {
			add(fmt.Sprintf(`(%s ILIKE ? OR %s ILIKE ?)`, th, en), escapeLike(value)+"%")
		}
	}

	if c.NationalID != "" {
		add(`national_id = ?`, c.NationalID)
	}
	if c.PassportID != "" {
		add(`passport_id = ?`, c.PassportID)
	}
	name(c.FirstName, "first_name_th", "first_name_en")
	name(c.MiddleName, "middle_name_th", "middle_name_en")
	name(c.LastName, "last_name_th", "last_name_en")
	if c.DateOfBirth != "" {
		add(`date_of_birth = ?::date`, c.DateOfBirth)
	}
	if c.PhoneNumber != "" {
		add(`phone_number = ?`, c.PhoneNumber)
	}
	if c.Email != "" {
		add(`lower(email) = lower(?)`, c.Email)
	}
	return strings.Join(conds, " AND "), args
}

// escapeLike makes s match literally inside a LIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *Repository) Upsert(ctx context.Context, p patient.Patient) (patient.Patient, error) {
	// NULLIF turns the domain's "" into NULL for optional columns.
	row := r.db.QueryRowContext(ctx, `INSERT INTO patients (
			id, hospital_id, patient_hn, national_id, passport_id,
			first_name_th, middle_name_th, last_name_th,
			first_name_en, middle_name_en, last_name_en,
			date_of_birth, phone_number, email, gender, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5,
			NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''),
			NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''),
			NULLIF($12, '')::date, NULLIF($13, ''), NULLIF($14, ''), NULLIF($15, ''), $16, $17)
		ON CONFLICT (hospital_id, patient_hn) DO UPDATE SET
			national_id = EXCLUDED.national_id, passport_id = EXCLUDED.passport_id,
			first_name_th = EXCLUDED.first_name_th, middle_name_th = EXCLUDED.middle_name_th,
			last_name_th = EXCLUDED.last_name_th, first_name_en = EXCLUDED.first_name_en,
			middle_name_en = EXCLUDED.middle_name_en, last_name_en = EXCLUDED.last_name_en,
			date_of_birth = EXCLUDED.date_of_birth, phone_number = EXCLUDED.phone_number,
			email = EXCLUDED.email, gender = EXCLUDED.gender, updated_at = EXCLUDED.updated_at
		RETURNING `+columns,
		p.ID, p.HospitalID, p.PatientHN, p.NationalID, p.PassportID,
		p.FirstNameTH, p.MiddleNameTH, p.LastNameTH,
		p.FirstNameEN, p.MiddleNameEN, p.LastNameEN,
		p.DateOfBirth, p.PhoneNumber, p.Email, p.Gender, p.CreatedAt, p.UpdatedAt)
	return scan(row)
}

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (patient.Patient, error) {
	var (
		p                    patient.Patient
		nationalID, passport sql.NullString
	)
	if err := s.Scan(&p.ID, &p.HospitalID, &p.PatientHN, &nationalID, &passport,
		&p.FirstNameTH, &p.MiddleNameTH, &p.LastNameTH,
		&p.FirstNameEN, &p.MiddleNameEN, &p.LastNameEN,
		&p.DateOfBirth, &p.PhoneNumber, &p.Email, &p.Gender, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return patient.Patient{}, err
	}
	if nationalID.Valid {
		p.NationalID = &nationalID.String
	}
	if passport.Valid {
		p.PassportID = &passport.String
	}
	p.CreatedAt, p.UpdatedAt = p.CreatedAt.UTC(), p.UpdatedAt.UTC()
	return p, nil
}
