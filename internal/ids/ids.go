// Package ids mints the identifiers the domains store.
package ids

import (
	"crypto/rand"
	"fmt"
)

// New returns a random (version 4) UUID string. Domains mint IDs themselves,
// so the format doesn't depend on which store is behind a Repository.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
