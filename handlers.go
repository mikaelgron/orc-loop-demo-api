package counterapi

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
)

// idPattern is a counter id: lowercase letters, digits and hyphens, at
// most 64 — the id names the counter its first request creates.
var idPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// appendStep records exactly one step on the counter the path names, at
// the time the service was asked, and answers with the step it recorded.
// orc:ADR-0002#decision.c1
func appendStep(store *Store, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !validID(id) {
			httpError(w, http.StatusBadRequest,
				"a counter id is lowercase letters, digits and hyphens, at most 64 long")
			return
		}
		step, err := store.Append(id, now())
		if err != nil {
			httpError(w, http.StatusInternalServerError, "recording the step: "+err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, step)
	}
}

// listSteps answers the counter's recorded steps in the order they were
// recorded; a counter that does not exist is not an empty one.
// orc:ADR-0002#decision.c3
func listSteps(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !validID(id) {
			httpError(w, http.StatusBadRequest,
				"a counter id is lowercase letters, digits and hyphens, at most 64 long")
			return
		}
		steps, found, err := store.Steps(id)
		if err != nil {
			httpError(w, http.StatusInternalServerError, "reading the steps: "+err.Error())
			return
		}
		if !found {
			httpError(w, http.StatusNotFound, "no such counter")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps})
	}
}

// getCounter answers how many steps the counter has recorded.
// orc:ADR-0002#decision.c4
func getCounter(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !validID(id) {
			httpError(w, http.StatusBadRequest,
				"a counter id is lowercase letters, digits and hyphens, at most 64 long")
			return
		}
		tally, found, err := store.Tally(id)
		if err != nil {
			httpError(w, http.StatusInternalServerError, "reading the tally: "+err.Error())
			return
		}
		if !found {
			httpError(w, http.StatusNotFound, "no such counter")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "tally": tally})
	}
}

// resetCounter removes the counter and its steps, and only the admin
// credential reaches it.
// orc:ADR-0002#decision.c5
func resetCounter(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !validID(id) {
			httpError(w, http.StatusBadRequest,
				"a counter id is lowercase letters, digits and hyphens, at most 64 long")
			return
		}
		found, err := store.Reset(id)
		if err != nil {
			httpError(w, http.StatusInternalServerError, "resetting the counter: "+err.Error())
			return
		}
		if !found {
			httpError(w, http.StatusNotFound, "no such counter")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// live answers that the process is alive — without reading the store, so
// a store that is down does not take liveness with it.
// orc:ADR-0002#decision.c8
func live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready answers that the service is ready to serve: the store answers a
// read, or the service is not ready.
// orc:ADR-0002#decision.c7
func ready(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ping(); err != nil {
			httpError(w, http.StatusServiceUnavailable, "not ready: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

// validID says whether id is a counter id the service accepts.
func validID(id string) bool { return idPattern.MatchString(id) }

// writeJSON answers with v as JSON and the status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// httpError answers with the message as JSON.
func httpError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
