package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/DoIttikorn/hospital-middleware/internal/auth"
	"github.com/DoIttikorn/hospital-middleware/internal/database"
	"github.com/DoIttikorn/hospital-middleware/internal/his"
	"github.com/DoIttikorn/hospital-middleware/internal/hospital"
	hospitalstore "github.com/DoIttikorn/hospital-middleware/internal/hospital/postgres"
	"github.com/DoIttikorn/hospital-middleware/internal/httpx"
	"github.com/DoIttikorn/hospital-middleware/internal/patient"
	patientstore "github.com/DoIttikorn/hospital-middleware/internal/patient/postgres"
	"github.com/DoIttikorn/hospital-middleware/internal/staff"
	staffstore "github.com/DoIttikorn/hospital-middleware/internal/staff/postgres"
)

// Server is the HTTP server and the dependencies its handlers need.
type Server struct {
	log           *slog.Logger
	srv           *http.Server
	draining      atomic.Bool   // set by Shutdown; fails /readyz
	shutdownDelay time.Duration // SHUTDOWN_DELAY, see Shutdown
	closers       []func() error

	// checks are run by /readyz and /health, keyed by the name shown in the
	// response.
	checks   map[string]func(context.Context) error
	auth     *auth.Manager
	staff    staff.Service
	patients patient.Service
	db       database.Service
}

// New connects the dependencies and builds the HTTP server, logging through
// log. Call Close when done to release the dependencies.
func New(ctx context.Context, log *slog.Logger) (*Server, error) {
	delay, err := time.ParseDuration(getenv("SHUTDOWN_DELAY", "0s"))
	if err != nil {
		return nil, fmt.Errorf("SHUTDOWN_DELAY: %w", err)
	}
	s := &Server{
		log:           log,
		shutdownDelay: delay,
		checks:        map[string]func(context.Context) error{},
	}
	fail := func(what string, err error) (*Server, error) {
		s.Close()
		return nil, fmt.Errorf("%s: %w", what, err)
	}

	db, err := database.New()
	if err != nil {
		return fail("connect database", err)
	}
	s.db = db
	s.checks["database"] = db.Check
	s.closers = append(s.closers, db.Close)

	if err := database.Migrate(db.SQL()); err != nil {
		return fail("migrate database", err)
	}

	secret := os.Getenv("JWT_SECRET")
	ttl, err := envDuration("JWT_TTL_MINUTES", 60, time.Minute)
	if err != nil {
		return fail("config", err)
	}
	hisTimeout, err := envDuration("HIS_TIMEOUT_SECONDS", 5, time.Second)
	if err != nil {
		return fail("config", err)
	}
	s.auth, err = auth.NewManager(secret, ttl)
	if err != nil {
		return fail("JWT_SECRET/JWT_TTL_MINUTES", err)
	}

	// Composition root: pick each domain's adapters here. The domain packages
	// only know their Repository interfaces.
	hospitals := hospital.NewService(hospitalstore.New(db.SQL()))
	s.staff = staff.NewService(staffstore.New(db.SQL()), hospitals, s.auth)
	registry := his.NewRegistry(hisTimeout, hisBaseURLOverrides())
	s.patients = patient.NewService(patientstore.New(db.SQL()), hospitals, registry)

	s.srv = &http.Server{
		Addr:              ":" + getenv("PORT", "8080"),
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}
	return s, nil
}

// Handler serves the probes and the API. The probes are mounted outside the
// framework router and the request log, so they stay cheap and quiet. The
// API gets one request logger for every framework (httpx.Log).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", s.livez)
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.HandleFunc("GET /health", s.healthz)
	mux.Handle("/", httpx.Log(s.log)(s.RegisterRoutes()))
	return mux
}

// Addr is the address the server listens on, e.g. ":8080".
func (s *Server) Addr() string { return s.srv.Addr }

// ListenAndServe serves until Shutdown; it then returns http.ErrServerClosed.
func (s *Server) ListenAndServe() error { return s.srv.ListenAndServe() }

// Shutdown stops the server without dropping requests during a Kubernetes
// rolling update. It first fails /readyz so the pod is taken out of the
// Service, waits SHUTDOWN_DELAY for ingress-nginx to stop sending new
// requests, then stops listening and waits for in-flight requests to finish.
func (s *Server) Shutdown(ctx context.Context) error {
	s.draining.Store(true)
	if s.shutdownDelay > 0 {
		s.log.Info("draining", "delay", s.shutdownDelay)
		select {
		case <-time.After(s.shutdownDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.srv.Shutdown(ctx)
}

// Close releases the dependencies in reverse order of creation.
func (s *Server) Close() {
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i](); err != nil {
			s.log.Error("close dependency", "error", err)
		}
	}
}

// hisBaseURLOverrides reads HIS addresses that replace the ones stored with
// the hospitals, e.g. to point Hospital A at a local mock.
func hisBaseURLOverrides() map[string]string {
	return map[string]string{"hospital-a": os.Getenv("HOSPITAL_A_BASE_URL")}
}

// envDuration reads a positive whole number of units from the environment.
func envDuration(key string, fallback int, unit time.Duration) (time.Duration, error) {
	n := fallback
	if v := os.Getenv(key); v != "" {
		var err error
		if n, err = strconv.Atoi(v); err != nil || n <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer, got %q", key, v)
		}
	}
	return time.Duration(n) * unit, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
