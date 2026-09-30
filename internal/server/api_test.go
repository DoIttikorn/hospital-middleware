package server

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

// The tests here go through the whole stack (router, auth middleware,
// handlers, services, memory adapters) the way a client does.

func login(t *testing.T, h http.Handler, username, password, hospital string) string {
	t.Helper()
	var tok struct{ Token string }
	body := `{"username":"` + username + `","password":"` + password + `","hospital":"` + hospital + `"}`
	if rec := do(t, h, http.MethodPost, "/staff/login", body, &tok); rec.Code != http.StatusOK {
		t.Fatalf("login %s@%s = %d: %s", username, hospital, rec.Code, rec.Body)
	}
	return tok.Token
}

func TestStaffFlow(t *testing.T) {
	h := newTestServer().Handler()

	adminToken := login(t, h, "admin", adminPassword, "hospital-a")

	// A logged-in staff member creates a colleague in their own hospital...
	var created struct{ ID, Username, Hospital string }
	rec := doAs(t, h, adminToken, http.MethodPost, "/staff/create",
		`{"username":"nurse01","password":"s3cretPass!","hospital":"hospital-a"}`, &created)
	if rec.Code != http.StatusCreated || created.Username != "nurse01" || created.Hospital != "hospital-a" || created.ID == "" {
		t.Fatalf("create = %d %+v: %s", rec.Code, created, rec.Body)
	}

	// ...who can log in and use their own token.
	nurseToken := login(t, h, "nurse01", "s3cretPass!", "hospital-a")
	if rec := doAs(t, h, nurseToken, http.MethodGet, "/patient/search", "", nil); rec.Code != http.StatusOK {
		t.Errorf("search as new staff = %d", rec.Code)
	}
}

func TestStaffCreateRules(t *testing.T) {
	h := newTestServer().Handler()
	token := login(t, h, "admin", adminPassword, "hospital-a")
	const body = `{"username":"nurse01","password":"s3cretPass!","hospital":"hospital-a"}`

	if rec := do(t, h, http.MethodPost, "/staff/create", body, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("create without a token = %d, want 401", rec.Code)
	}
	if rec := doAs(t, h, "garbage", http.MethodPost, "/staff/create", body, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("create with a bad token = %d, want 401", rec.Code)
	}

	// Staff of hospital A may not create staff in hospital B.
	other := strings.Replace(body, `"hospital-a"`, `"hospital-b"`, 1)
	if rec := doAs(t, h, token, http.MethodPost, "/staff/create", other, nil); rec.Code != http.StatusForbidden {
		t.Errorf("create in another hospital = %d, want 403: %s", rec.Code, rec.Body)
	}
	unknown := strings.Replace(body, `"hospital-a"`, `"hospital-zzz"`, 1)
	if rec := doAs(t, h, token, http.MethodPost, "/staff/create", unknown, nil); rec.Code != http.StatusNotFound {
		t.Errorf("create in an unknown hospital = %d, want 404", rec.Code)
	}

	if rec := doAs(t, h, token, http.MethodPost, "/staff/create", body, nil); rec.Code != http.StatusCreated {
		t.Fatalf("first create = %d: %s", rec.Code, rec.Body)
	}
	if rec := doAs(t, h, token, http.MethodPost, "/staff/create", body, nil); rec.Code != http.StatusConflict {
		t.Errorf("duplicate create = %d, want 409", rec.Code)
	}
	if rec := doAs(t, h, token, http.MethodPost, "/staff/create", `{"username":"x"}`, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("create with missing fields = %d, want 400", rec.Code)
	}
}

func TestStaffLoginRules(t *testing.T) {
	h := newTestServer().Handler()
	for name, body := range map[string]string{
		"wrong password":   `{"username":"admin","password":"nope-nope","hospital":"hospital-a"}`,
		"unknown user":     `{"username":"ghost","password":"` + adminPassword + `","hospital":"hospital-a"}`,
		"unknown hospital": `{"username":"admin","password":"` + adminPassword + `","hospital":"hospital-zzz"}`,
	} {
		if rec := do(t, h, http.MethodPost, "/staff/login", body, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: login = %d, want 401", name, rec.Code)
		}
	}
	if rec := do(t, h, http.MethodPost, "/staff/login", `{"username":"admin"}`, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("login with missing fields = %d, want 400", rec.Code)
	}
}

type searchResponse struct {
	Total, Limit, Offset int
	Data                 []map[string]any
}

func TestPatientSearchFlow(t *testing.T) {
	e := newTestEnv()
	h := e.server.Handler()
	somchaiID := "1234567890123"
	e.his.Patients[somchaiID] = patient.Patient{
		PatientHN: "HN0001234", NationalID: &somchaiID,
		FirstNameTH: "สมชาย", LastNameTH: "ใจดี", FirstNameEN: "Somchai", LastNameEN: "Jaidee",
		DateOfBirth: "1990-05-17", Gender: "M",
	}
	tokenA := login(t, h, "admin", adminPassword, "hospital-a")
	tokenB := login(t, h, "admin", adminPassword, "hospital-b")

	search := func(token, query string) (int, searchResponse) {
		var res searchResponse
		rec := doAs(t, h, token, http.MethodGet, "/patient/search"+query, "", &res)
		return rec.Code, res
	}

	if rec := do(t, h, http.MethodGet, "/patient/search", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("search without a token = %d, want 401", rec.Code)
	}

	// Nothing stored yet: an unfiltered search is valid and empty.
	if code, res := search(tokenA, ""); code != http.StatusOK || res.Total != 0 || len(res.Data) != 0 {
		t.Errorf("empty search = %d %+v", code, res)
	}

	// Searching by national ID pulls the patient in from hospital A's HIS.
	code, res := search(tokenA, "?national_id="+somchaiID)
	if code != http.StatusOK || res.Total != 1 || res.Data[0]["patient_hn"] != "HN0001234" {
		t.Fatalf("search by national_id = %d %+v", code, res)
	}

	// From then on the patient is found by any field, in hospital A only.
	for _, q := range []string{"", "?first_name=som", "?first_name=สม", "?last_name=JAI", "?date_of_birth=1990-05-17"} {
		if code, res := search(tokenA, q); code != http.StatusOK || res.Total != 1 {
			t.Errorf("hospital A search %q = %d %+v, want 1 result", q, code, res)
		}
	}
	for _, q := range []string{"", "?national_id=" + somchaiID, "?first_name=som"} {
		if code, res := search(tokenB, q); code != http.StatusOK || res.Total != 0 {
			t.Errorf("hospital B search %q = %d %+v, want no result", q, code, res)
		}
	}

	if code, _ := search(tokenA, "?date_of_birth=not-a-date"); code != http.StatusBadRequest {
		t.Errorf("bad date = %d, want 400", code)
	}
	if code, _ := search(tokenA, "?limit=abc"); code != http.StatusBadRequest {
		t.Errorf("bad limit = %d, want 400", code)
	}
	if code, _ := search(tokenA, "?national_id=.."); code != http.StatusBadRequest {
		t.Errorf("national_id %q = %d, want 400", "..", code)
	}
	if calls := e.his.Calls(); !slices.Equal(calls, []string{somchaiID}) {
		t.Errorf("HIS calls = %v, want only the first national_id lookup", calls)
	}
}
