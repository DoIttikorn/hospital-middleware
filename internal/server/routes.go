package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	patienthandler "github.com/DoIttikorn/hospital-middleware/internal/patient/handler"
	staffhandler "github.com/DoIttikorn/hospital-middleware/internal/staff/handler"
)

// RegisterRoutes returns the router with every API route mounted. The
// probes (/livez, /readyz, /health) are served by Handler.
//
//	POST /staff/login      public
//	POST /staff/create     needs a token
//	GET  /patient/search   needs a token
func (s *Server) RegisterRoutes() http.Handler {
	// Requests are logged, and panics recovered, by httpx.Log in Handler.
	r := gin.New()

	public := r.Group("")
	protected := r.Group("", s.auth.Middleware())

	staffhandler.New(s.staff, s.log).Register(public, protected)
	patienthandler.New(s.patients, s.log).Register(protected)

	return r
}
