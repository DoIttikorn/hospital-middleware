// Package handler is the REST adapter for the patient domain: it turns HTTP
// requests into Service calls and domain errors into RFC 9457 responses.
package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/DoIttikorn/hospital-middleware/internal/auth"
	"github.com/DoIttikorn/hospital-middleware/internal/dates"
	"github.com/DoIttikorn/hospital-middleware/internal/httpx"
	"github.com/DoIttikorn/hospital-middleware/internal/patient"
)

// Service is the slice of patient.Service these endpoints need. It is
// declared here, where it is used, so handler tests can pass a fake.
type Service interface {
	Search(ctx context.Context, hospitalID string, c patient.Criteria, p patient.Page) (patient.Result, error)
}

// BasePath is where the server mounts these routes.
const BasePath = "/patient"

// Handler serves BasePath.
type Handler struct {
	svc Service
	log *slog.Logger
}

func New(svc Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Register mounts GET /patient/search on protected, which must run the auth
// middleware, e.g. Register(r.Group("", authMiddleware)).
func (h *Handler) Register(protected *gin.RouterGroup) {
	protected.GET(BasePath+"/search", h.search)
}

// fail writes the problem response that matches err. Unexpected errors are
// logged, and their details are not sent to the client.
func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, patient.ErrInvalid):
		httpx.WriteProblemCode(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, patient.ErrHISUnavailable):
		h.log.Warn("HIS unavailable", "error", err)
		httpx.WriteProblemCode(w, http.StatusBadGateway, "his_unavailable", "the hospital information system could not be reached")
	default:
		h.log.Error("patient", "error", err)
		httpx.WriteProblem(w, http.StatusInternalServerError, "")
	}
}

func (h *Handler) search(c *gin.Context) {
	// The hospital always comes from the token, never from the request.
	id, ok := auth.FromContext(c)
	if !ok { // the route is mounted without the auth middleware
		h.log.Error("patient search reached without an identity")
		httpx.WriteProblem(c.Writer, http.StatusInternalServerError, "")
		return
	}
	page, err := parsePage(c)
	if err != nil {
		h.fail(c.Writer, err)
		return
	}
	dob, err := dates.ToISO(c.Query("date_of_birth"))
	if err != nil {
		h.fail(c.Writer, fmt.Errorf("%w: date_of_birth %w", patient.ErrInvalid, err))
		return
	}
	crit := patient.Criteria{
		NationalID:  c.Query("national_id"),
		PassportID:  c.Query("passport_id"),
		FirstName:   c.Query("first_name"),
		MiddleName:  c.Query("middle_name"),
		LastName:    c.Query("last_name"),
		DateOfBirth: dob,
		PhoneNumber: c.Query("phone_number"),
		Email:       c.Query("email"),
	}
	res, err := h.svc.Search(c.Request.Context(), id.HospitalID, crit, page)
	if err != nil {
		h.fail(c.Writer, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// parsePage reads limit and offset; absent or blank means "use the default".
func parsePage(c *gin.Context) (patient.Page, error) {
	var p patient.Page
	for name, dst := range map[string]*int{"limit": &p.Limit, "offset": &p.Offset} {
		raw := c.Query(name)
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return p, fmt.Errorf("%w: %s must be an integer", patient.ErrInvalid, name)
		}
		*dst = n
	}
	return p, nil
}
