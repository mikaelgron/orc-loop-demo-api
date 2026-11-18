package counterapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// New is the service: the counters behind their routes and the two probes
// — liveness that never reads the store, readiness that does, and the one
// admin-gated route.
// orc:ADR-0002#decision.c1
func New(logger *slog.Logger, store *Store, adminToken string, now func() time.Time) http.Handler {
	r := chi.NewRouter()
	r.Use(requestLogger(logger))
	r.Get("/healthz", live)
	r.Get("/readyz", ready(store))
	r.Post("/counters/{id}/steps", appendStep(store, now))
	r.Get("/counters/{id}/steps", listSteps(store))
	r.Get("/counters/{id}", getCounter(store))
	r.With(requireAdmin(adminToken)).Delete("/counters/{id}", resetCounter(store))
	return r
}
