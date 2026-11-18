package counterapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// statusWriter captures the status a handler answered with — the one
// part of the response the middleware logs but the response writer
// hides.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader remembers the status the handler answered with.
func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// requestLogger logs every request: its method, its route, its response
// status and how long the answer took. The route is the pattern the
// router matched; a request that matched no route still logs, with the
// path itself in the route's place.
// orc:ADR-0002#decision.c6
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: 0}
			next.ServeHTTP(sw, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}
			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.Int("status", status),
				slog.Duration("duration", time.Since(start)))
		})
	}
}
