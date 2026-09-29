package staff

import "context"

// Repository is the persistence port: the service's only view of storage.
// Every implementation must pass the contract in stafftest.
type Repository interface {
	// Insert stores a new staff member. The service has already set every
	// field. It returns an error wrapping ErrDuplicate when the hospital
	// already has that username.
	Insert(ctx context.Context, s Staff) error

	// ByHospitalAndUsername returns the staff member, or an error wrapping
	// ErrNotFound. Usernames are only unique within a hospital.
	ByHospitalAndUsername(ctx context.Context, hospitalID, username string) (Staff, error)
}
