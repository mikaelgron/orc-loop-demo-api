package counterapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fixedNow is the moment the tests record steps at: whole seconds in
// UTC, so a step's time survives its JSON round trip exactly.
func fixedNow() time.Time {
	return time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
}

// fixedClock answers fixedNow on every call.
func fixedClock() func() time.Time { return func() time.Time { return fixedNow() } }

// newStore opens a store in the test's own temp directory and closes it
// when the test ends.
func newStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.TempDir() + "/store.db")
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// newService builds the service the tests drive: a store in the test's
// own temp directory, logging into the buffer the logging tests read.
func newService(t *testing.T, log *bytes.Buffer) (http.Handler, *Store) {
	t.Helper()
	st := newStore(t)
	var logger *slog.Logger
	if log != nil {
		logger = slog.New(slog.NewJSONHandler(log, nil))
	} else {
		logger = slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	}
	return New(logger, st, "admin-tok", fixedClock()), st
}

// do runs one request against the service, optionally carrying the
// admin credential, and answers what the service answered.
func do(srv http.Handler, method, path, adminToken string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if adminToken != "" {
		req.Header.Set("X-Admin-Token", adminToken)
	}
	srv.ServeHTTP(rec, req)
	return rec
}

// decodeStep reads one step from the recorder's answer.
func decodeStep(t *testing.T, rec *httptest.ResponseRecorder) Step {
	t.Helper()
	var step Step
	if err := json.Unmarshal(rec.Body.Bytes(), &step); err != nil {
		t.Fatalf("decoding the step: %v (body %q)", err, rec.Body.String())
	}
	return step
}
