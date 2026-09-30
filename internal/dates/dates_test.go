package dates

import (
	"errors"
	"testing"
)

func TestToISO(t *testing.T) {
	valid := map[string]string{
		"17/05/1990":   "1990-05-17",
		"01/02/1985":   "1985-02-01",
		"29/02/2024":   "2024-02-29", // leap year
		" 17/05/1990 ": "1990-05-17",
		"":             "",
		"   ":          "",
	}
	for in, want := range valid {
		if got, err := ToISO(in); err != nil || got != want {
			t.Errorf("ToISO(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	invalid := []string{
		"1990-05-17", // YYYY-MM-DD
		"17-05-1990",
		"1/5/1990",   // day and month need two digits
		"05/17/1990", // MM/DD/YYYY
		"31/02/1990", // does not exist
		"29/02/2023", // not a leap year
		"17/05/90",
		"not-a-date",
	}
	for _, in := range invalid {
		if got, err := ToISO(in); !errors.Is(err, ErrFormat) || got != "" {
			t.Errorf("ToISO(%q) = %q, %v; want ErrFormat", in, got, err)
		}
	}
}
