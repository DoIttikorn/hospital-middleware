// Package dbtest helps the postgres adapters' tests run against a real
// database (INTEGRATION=1, database started with "make deps-up").
package dbtest

import (
	"database/sql"
	"os"
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/database"
	"github.com/DoIttikorn/hospital-middleware/internal/ids"
)

// Open returns a migrated pool, or skips the test when INTEGRATION is unset.
func Open(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 with the database running (make deps-up)")
	}
	svc, err := database.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })
	if err := database.Migrate(svc.SQL()); err != nil {
		t.Fatal(err)
	}
	return svc.SQL()
}

// NewHospital inserts a hospital with a unique code and removes it, with its
// staff and patients, when the test ends. Tests use their own hospitals
// instead of clearing tables, so packages can run in parallel against one
// database.
func NewHospital(t *testing.T, db *sql.DB) string {
	t.Helper()
	id := ids.New()
	if _, err := db.Exec(`INSERT INTO hospitals (id, code, name) VALUES ($1, $2, $2)`, id, "test-"+id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM patients WHERE hospital_id = $1`,
			`DELETE FROM staff WHERE hospital_id = $1`,
			`DELETE FROM hospitals WHERE id = $1`,
		} {
			if _, err := db.Exec(q, id); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	})
	return id
}
