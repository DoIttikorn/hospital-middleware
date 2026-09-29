package ids

import (
	"regexp"
	"testing"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id := New()
		if !uuidV4.MatchString(id) {
			t.Fatalf("New() = %q, not a v4 UUID", id)
		}
		if seen[id] {
			t.Fatalf("New() repeated %q", id)
		}
		seen[id] = true
	}
}
