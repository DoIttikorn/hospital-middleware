// Package handler is the REST adapter for the staff domain: it turns HTTP
// requests into Service calls and domain errors into RFC 9457 responses.
package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/DoIttikorn/hospital-middleware/internal/auth"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	"github.com/DoIttikorn/hospital-middleware/internal/httpx"
	"github.com/DoIttikorn/hospital-middleware/internal/staff"
)

// Service is the slice of staff.Service these endpoints need. It is declared
// here, where it is used, so handler tests can pass a fake.
type Service interface {
	Create(ctx context.Context, callerHospitalID string, in staff.Input) (staff.Account, error)
	Login(ctx context.Context, in staff.Input) (staff.Token, error)
}

// BasePath is where the server mounts these routes.
const BasePath = "/staff"

// Handler serves BasePath.
type Handler struct {
	svc Service
	log *slog.Logger
}

func New(svc Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Register mounts POST /staff/login on public and POST /staff/create on
// protected, which must run the auth middleware. Both take the router root,
// e.g. Register(r.Group(""), r.Group("", authMiddleware)).
func (h *Handler) Register(public, protected *gin.RouterGroup) {
	public.POST(BasePath+"/login", h.login)
	protected.POST(BasePath+"/create", h.create)
}

// fail writes the problem response that matches err. Unexpected errors are
// logged, and their details are not sent to the client.
func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, httpx.ErrBadRequest):
		httpx.WriteProblemCode(w, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, staff.ErrInvalid):
		httpx.WriteProblemCode(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, staff.ErrInvalidCredentials):
		httpx.WriteProblemCode(w, http.StatusUnauthorized, "invalid_credentials", err.Error())
	case errors.Is(err, staff.ErrForbidden):
		httpx.WriteProblemCode(w, http.StatusForbidden, "hospital_mismatch", err.Error())
	case errors.Is(err, hospital.ErrNotFound):
		httpx.WriteProblemCode(w, http.StatusNotFound, "hospital_not_found", err.Error())
	case errors.Is(err, staff.ErrDuplicate):
		httpx.WriteProblemCode(w, http.StatusConflict, "username_taken", err.Error())
	default:
		h.log.Error("staff", "error", err)
		httpx.WriteProblem(w, http.StatusInternalServerError, "")
	}
}

func (h *Handler) create(c *gin.Context) {
	id, ok := auth.FromContext(c)
	if !ok { // the route is mounted without the auth middleware
		h.log.Error("staff create reached without an identity")
		httpx.WriteProblem(c.Writer, http.StatusInternalServerError, "")
		return
	}
	var in staff.Input
	if err := httpx.DecodeJSON(c.Request, &in); err != nil {
		h.fail(c.Writer, err)
		return
	}
	acc, err := h.svc.Create(c.Request.Context(), id.HospitalID, in)
	if err != nil {
		h.fail(c.Writer, err)
		return
	}
	c.JSON(http.StatusCreated, acc)
}

func (h *Handler) login(c *gin.Context) {
	var in staff.Input
	if err := httpx.DecodeJSON(c.Request, &in); err != nil {
		h.fail(c.Writer, err)
		return
	}
	tok, err := h.svc.Login(c.Request.Context(), in)
	if err != nil {
		h.fail(c.Writer, err)
		return
	}
	c.JSON(http.StatusOK, tok)
}
