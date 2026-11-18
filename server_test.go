package counterapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestPostRecordsOneStepPerRequest checks the append: one POST records
// exactly one step — the ordinal it took and the time it was recorded —
// and answers with that step.
// orc:ADR-0002#decision.c1
func TestPostRecordsOneStepPerRequest(t *testing.T) {
	srv, _ := newService(t, nil)

	rec := do(srv, http.MethodPost, "/counters/a/steps", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("the answer's status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	step := decodeStep(t, rec)
	if step.Ordinal != 1 {
		t.Errorf("the recorded ordinal = %d, want 1", step.Ordinal)
	}
	if !step.At.Equal(fixedNow()) {
		t.Errorf("the recorded time = %s, want %s", step.At, fixedNow())
	}

	// A second POST on the same counter records one step more — two
	// requests, two steps — and each answer carries its own step.
	rec2 := do(srv, http.MethodPost, "/counters/a/steps", "")
	if rec2.Code != http.StatusCreated {
		t.Fatalf("the second answer's status = %d, want %d", rec2.Code, http.StatusCreated)
	}
	step2 := decodeStep(t, rec2)
	if step2.Ordinal != 2 {
		t.Errorf("the second ordinal = %d, want 2", step2.Ordinal)
	}

	// An id that is not a counter id is refused before the store is
	// touched — the append route answers its own bad input.
	recBad := do(srv, http.MethodPost, "/counters/NO-PE/steps", "")
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("the bad id's answer = %d, want %d", recBad.Code, http.StatusBadRequest)
	}
}

// TestOrdinalsStartAtOneAndStepByOne checks the ordinal's arithmetic as
// the API behaves it: the first append takes ordinal one and the next
// one exactly one more.
// orc:ADR-0002#decision.c2
func TestOrdinalsStartAtOneAndStepByOne(t *testing.T) {
	var srv http.Handler
	srv, _ = newService(t, nil)

	for i := 1; i <= 3; i++ {
		rec := do(srv, http.MethodPost, "/counters/a/steps", "")
		if rec.Code != http.StatusCreated {
			t.Fatalf("the append %d answered %d, want %d", i, rec.Code, http.StatusCreated)
		}
		step := decodeStep(t, rec)
		if step.Ordinal != uint64(i) {
			t.Errorf("append %d recorded ordinal %d, want %d", i, step.Ordinal, i)
		}
	}
}

// TestStepsReadBackInOrder checks the read: the counter's steps come
// back in the order they were recorded.
// orc:ADR-0002#decision.c3
func TestStepsReadBackInOrder(t *testing.T) {
	var srv http.Handler
	srv, _ = newService(t, nil)

	for i := 1; i <= 3; i++ {
		do(srv, http.MethodPost, "/counters/a/steps", "")
	}
	rec := do(srv, http.MethodGet, "/counters/a/steps", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("the read's status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Steps []Step `json:"steps"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the steps: %v", err)
	}
	if len(body.Steps) != 3 {
		t.Fatalf("%d steps came back, want 3", len(body.Steps))
	}
	for i, step := range body.Steps {
		if step.Ordinal != uint64(i+1) {
			t.Errorf("the step in place %d has ordinal %d, want %d", i, step.Ordinal, i+1)
		}
	}

	// A counter that does not exist is not an empty one.
	recMissing := do(srv, http.MethodGet, "/counters/b/steps", "")
	if recMissing.Code != http.StatusNotFound {
		t.Errorf("reading a counter that was never written = %d, want %d", recMissing.Code, http.StatusNotFound)
	}
}

// TestGetCounterAnswersTally checks the count the counter reads: how
// many steps it has recorded.
// orc:ADR-0002#decision.c4
func TestGetCounterAnswersTally(t *testing.T) {
	var srv http.Handler
	srv, _ = newService(t, nil)

	for i := 1; i <= 3; i++ {
		do(srv, http.MethodPost, "/counters/a/steps", "")
	}
	rec := do(srv, http.MethodGet, "/counters/a", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("the counter read = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		ID    string `json:"id"`
		Tally int    `json:"tally"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the counter: %v", err)
	}
	if body.ID != "a" {
		t.Errorf("the id = %q, want %q", body.ID, "a")
	}
	if body.Tally != 3 {
		t.Errorf("the tally = %d, want 3", body.Tally)
	}

	// A counter that was never written answers no tally at all.
	recMissing := do(srv, http.MethodGet, "/counters/b", "")
	if recMissing.Code != http.StatusNotFound {
		t.Errorf("reading a counter that was never written = %d, want %d", recMissing.Code, http.StatusNotFound)
	}
}

// TestResetRequiresTheAdminCredential checks the gate: reset answers
// only the admin credential, and after a reset the same id begins again
// at ordinal one — a fresh counter, not a continued one.
// orc:ADR-0002#decision.c2
// orc:ADR-0002#decision.c5
func TestResetRequiresTheAdminCredential(t *testing.T) {
	var srv http.Handler
	srv, _ = newService(t, nil)

	do(srv, http.MethodPost, "/counters/a/steps", "")

	// Without the credential, reset is refused — as is the wrong
	// credential; neither reaches the store.
	if rec := do(srv, http.MethodDelete, "/counters/a", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no credential: reset = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec := do(srv, http.MethodDelete, "/counters/a", "not-the-token"); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong credential: reset = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	// The refused resets touched nothing: the counter still answers.
	if rec := do(srv, http.MethodGet, "/counters/a", ""); rec.Code != http.StatusOK {
		t.Fatalf("the counter after refused resets = %d, want %d", rec.Code, http.StatusOK)
	}

	rec := do(srv, http.MethodDelete, "/counters/a", "admin-tok")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("the reset = %d, want %d (body %q)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	// The counter is gone, and the same id starts again at one.
	if rec := do(srv, http.MethodGet, "/counters/a", ""); rec.Code != http.StatusNotFound {
		t.Errorf("the reset counter still answers = %d, want %d", rec.Code, http.StatusNotFound)
	}
	recFresh := do(srv, http.MethodPost, "/counters/a/steps", "")
	if recFresh.Code != http.StatusCreated {
		t.Fatalf("the fresh counter's first append = %d, want %d", recFresh.Code, http.StatusCreated)
	}
	if step := decodeStep(t, recFresh); step.Ordinal != 1 {
		t.Errorf("the fresh counter's first ordinal = %d, want 1", step.Ordinal)
	}
}

// TestLivenessNeverReadsTheStore checks liveness's independence: the
// liveness probe answers while the store is down — a store that is not
// answering does not take the process's liveness with it.
// orc:ADR-0002#decision.c8
func TestLivenessNeverReadsTheStore(t *testing.T) {
	srv, st := newService(t, nil)

	if rec := do(srv, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Fatalf("liveness with a healthy store = %d, want %d", rec.Code, http.StatusOK)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("closing the store: %v", err)
	}
	rec := do(srv, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Errorf("liveness with the store down = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

// TestReadinessReadsTheStore checks readiness's dependence: the probe
// reads the store, answers ready while it answers, and stops answering
// ready when the store does not.
// orc:ADR-0002#decision.c7
func TestReadinessReadsTheStore(t *testing.T) {
	srv, st := newService(t, nil)

	if rec := do(srv, http.MethodGet, "/readyz", ""); rec.Code != http.StatusOK {
		t.Fatalf("readiness with a healthy store = %d, want %d", rec.Code, http.StatusOK)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("closing the store: %v", err)
	}
	rec := do(srv, http.MethodGet, "/readyz", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readiness with the store down = %d, want %d (body %q)", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

// TestEveryRequestIsLogged checks the one line per request: the method,
// the route, the status and the time the answer took, for the requests
// that matched a route and for the one that did not.
// orc:ADR-0002#decision.c6
func TestEveryRequestIsLogged(t *testing.T) {
	var log bytes.Buffer
	var srv http.Handler
	srv, _ = newService(t, &log)

	do(srv, http.MethodPost, "/counters/a/steps", "")
	do(srv, http.MethodGet, "/counters/a", "")
	do(srv, http.MethodGet, "/no-such-route", "")

	lines := strings.Split(strings.TrimSpace(log.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("%d log lines for 3 requests, want at least 3: %s", len(lines), log.String())
	}
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line %d is not JSON: %q", i, line)
		}
		for _, key := range []string{"method", "route", "status", "duration"} {
			if _, ok := entry[key]; !ok {
				t.Errorf("log line %d carries no %s: %q", i, key, line)
			}
		}
	}
	// The lines carry what happened: the append created, the tally
	// read, and the unrouted request still logged.
	if got := lines[0]; !strings.Contains(got, `"method":"POST"`) || !strings.Contains(got, `"route":"/counters/{id}/steps"`) {
		t.Errorf("the append's log line = %q, want method POST and the route pattern", got)
	}
	if got := lines[1]; !strings.Contains(got, `"method":"GET"`) || !strings.Contains(got, `"route":"/counters/{id}"`) {
		t.Errorf("the tally's log line = %q, want method GET and the route pattern", got)
	}
	if got := lines[2]; !strings.Contains(got, `"route":"/no-such-route"`) {
		t.Errorf("the unrouted request's log line = %q, want the path in the route's place", got)
	}
}
