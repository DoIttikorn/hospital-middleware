package patient

import "context"

// Repository is the persistence port: the service's only view of storage.
// Every implementation must pass the contract in patienttest.
//
// Every method is scoped to one hospital; that scoping is what keeps staff
// from seeing other hospitals' patients.
type Repository interface {
	// Search returns the page of the hospital's patients matching c, ordered
	// by patient_hn then ID, and the total number of matches. Criteria are
	// documented on Criteria; p must already be normalized. The result is
	// never nil.
	Search(ctx context.Context, hospitalID string, c Criteria, p Page) ([]Patient, int, error)

	// Upsert stores the patient, replacing the hospital's existing record
	// with the same PatientHN if there is one (its ID and CreatedAt are
	// kept). The service has already set every other field. It returns the
	// stored patient.
	Upsert(ctx context.Context, p Patient) (Patient, error)
}
