// Package dates converts the dates clients send, DD/MM/YYYY, into the
// YYYY-MM-DD dates the domains and stores keep.
package dates

import (
	"errors"
	"strings"
	"time"
)

// Layout is how clients write dates: DD/MM/YYYY.
const Layout = "02/01/2006"

// ErrFormat is returned for a date not in Layout or one that does not exist.
// Its message reads after a field name: "date_of_birth must be DD/MM/YYYY".
var ErrFormat = errors.New("must be DD/MM/YYYY")

// ToISO turns a DD/MM/YYYY date into YYYY-MM-DD; blank stays blank. Day and
// month need two digits, and the date must exist.
func ToISO(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	t, err := time.Parse(Layout, s)
	if err != nil {
		return "", ErrFormat
	}
	return t.Format(time.DateOnly), nil
}
