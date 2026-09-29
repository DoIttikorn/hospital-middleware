package his

import (
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

// Registry maps a hospital to its HIS client. Adding a hospital's HIS means
// writing its adapter and listing it in adapters below; nothing else in the
// service changes.
type Registry struct {
	timeout   time.Duration
	overrides map[string]string
	adapters  map[string]func(baseURL string, timeout time.Duration) patient.HISClient
}

var _ patient.HISRegistry = (*Registry)(nil)

// NewRegistry returns a Registry whose clients time out after timeout.
// baseURLs maps a hospital code to a HIS base URL that takes precedence over
// the one stored with the hospital, e.g. to point at a local mock.
func NewRegistry(timeout time.Duration, baseURLs map[string]string) *Registry {
	return &Registry{
		timeout:   timeout,
		overrides: baseURLs,
		adapters: map[string]func(string, time.Duration) patient.HISClient{
			"hospital-a": func(u string, d time.Duration) patient.HISClient { return NewHospitalA(u, d) },
		},
	}
}

// For returns the client for h, or false when h has no adapter or no HIS
// address.
func (r *Registry) For(h hospital.Hospital) (patient.HISClient, bool) {
	adapter, ok := r.adapters[h.Code]
	if !ok {
		return nil, false
	}
	baseURL := h.HISBaseURL
	if u := r.overrides[h.Code]; u != "" {
		baseURL = u
	}
	if baseURL == "" {
		return nil, false
	}
	return adapter(baseURL, r.timeout), true
}
