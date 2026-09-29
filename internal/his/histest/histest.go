// Package histest has fakes of the HIS ports, for tests of the patient
// service and of the server.
package histest

import (
	"context"
	"fmt"
	"sync"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

// Client is a patient.HISClient over a fixed set of patients, keyed by the
// national or passport ID they can be found with. It records its calls.
type Client struct {
	Patients map[string]patient.Patient
	// Err, if set, is returned by every call instead of a lookup.
	Err error

	mu    sync.Mutex
	calls []string
}

var _ patient.HISClient = (*Client)(nil)

func (c *Client) FindByID(ctx context.Context, id string) (patient.Patient, error) {
	c.mu.Lock()
	c.calls = append(c.calls, id)
	c.mu.Unlock()
	if c.Err != nil {
		return patient.Patient{}, c.Err
	}
	p, ok := c.Patients[id]
	if !ok {
		return patient.Patient{}, fmt.Errorf("%w: %s", patient.ErrNotFound, id)
	}
	return p, nil
}

// Calls returns the IDs looked up so far, in order.
func (c *Client) Calls() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

// Registry is a patient.HISRegistry keyed by hospital code.
type Registry map[string]patient.HISClient

var _ patient.HISRegistry = Registry(nil)

func (r Registry) For(h hospital.Hospital) (patient.HISClient, bool) {
	c, ok := r[h.Code]
	return c, ok
}
