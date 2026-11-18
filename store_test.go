package counterapi

import (
	"bytes"
	"testing"
)

// TestStoreAppendAndStepsRoundtrip checks that one append records the
// step the reading answers with: the ordinal and the recorded time,
// neither more nor less.
// orc:ADR-0002#decision.c1
func TestStoreAppendAndStepsRoundtrip(t *testing.T) {
	st := newStore(t)
	step, err := st.Append("a", fixedNow())
	if err != nil {
		t.Fatalf("appending: %v", err)
	}
	if step.Ordinal != 1 {
		t.Errorf("the first step's ordinal = %d, want 1", step.Ordinal)
	}
	if !step.At.Equal(fixedNow()) {
		t.Errorf("the step's time = %s, want %s", step.At, fixedNow())
	}
	steps, found, err := st.Steps("a")
	if err != nil || !found {
		t.Fatalf("reading the steps back: found=%v err=%v", found, err)
	}
	if len(steps) != 1 {
		t.Fatalf("the round trip gave %d steps, want 1", len(steps))
	}
	if steps[0].Ordinal != 1 || !steps[0].At.Equal(fixedNow()) {
		t.Errorf("the step read back = %+v, want ordinal 1 at %s", steps[0], fixedNow())
	}
}

// TestStoreOrdinalsStartAtOneAndStepByOne checks the ordinal's own rule:
// a counter's first step takes ordinal one and each later step exactly
// one more.
// orc:ADR-0002#decision.c2
func TestStoreOrdinalsStartAtOneAndStepByOne(t *testing.T) {
	st := newStore(t)
	for range 3 {
		if _, err := st.Append("a", fixedNow()); err != nil {
			t.Fatalf("appending: %v", err)
		}
	}
	steps, found, err := st.Steps("a")
	if err != nil || !found {
		t.Fatalf("reading the steps: found=%v err=%v", found, err)
	}
	if len(steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(steps))
	}
	for i, step := range steps {
		if step.Ordinal != uint64(i+1) {
			t.Errorf("step %d has ordinal %d, want %d", i, step.Ordinal, i+1)
		}
	}
}

// TestStoreTallyCountsSteps checks the count: how many steps the counter
// has recorded.
// orc:ADR-0002#decision.c4
func TestStoreTallyCountsSteps(t *testing.T) {
	st := newStore(t)
	for range 3 {
		if _, err := st.Append("a", fixedNow()); err != nil {
			t.Fatalf("appending: %v", err)
		}
	}
	tally, found, err := st.Tally("a")
	if err != nil || !found {
		t.Fatalf("reading the tally: found=%v err=%v", found, err)
	}
	if tally != 3 {
		t.Errorf("tally = %d, want 3", tally)
	}
}

// TestStoreResetThenFreshCounter checks reset's half: the counter with
// its steps is gone after reset, and a counter started again on the same
// id begins at ordinal one — a fresh counter, not a continued one.
// orc:ADR-0002#decision.c2
// orc:ADR-0002#decision.c5
func TestStoreResetThenFreshCounter(t *testing.T) {
	st := newStore(t)
	if _, err := st.Append("a", fixedNow()); err != nil {
		t.Fatalf("appending: %v", err)
	}
	found, err := st.Reset("a")
	if err != nil || !found {
		t.Fatalf("resetting: found=%v err=%v", found, err)
	}
	if _, found, err := st.Steps("a"); err != nil || found {
		t.Fatalf("the steps after reset: found=%v err=%v", found, err)
	}
	step, err := st.Append("a", fixedNow())
	if err != nil {
		t.Fatalf("appending to the fresh counter: %v", err)
	}
	if step.Ordinal != 1 {
		t.Errorf("the fresh counter's first ordinal = %d, want 1", step.Ordinal)
	}
}

// TestStoreReopenKeepsSteps checks the volume's promise: the steps
// survive the store closing and opening again.
// orc:ADR-0002#decision.c9
func TestStoreReopenKeepsSteps(t *testing.T) {
	path := t.TempDir() + "/store.db"
	st, err := Open(path)
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	if _, err := st.Append("a", fixedNow()); err != nil {
		t.Fatalf("appending: %v", err)
	}
	if _, err := st.Append("a", fixedNow()); err != nil {
		t.Fatalf("appending again: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer func() { _ = again.Close() }()
	steps, found, err := again.Steps("a")
	if err != nil || !found {
		t.Fatalf("reading the steps back after reopen: found=%v err=%v", found, err)
	}
	if len(steps) != 2 {
		t.Fatalf("%d steps survived the reopen, want 2", len(steps))
	}
	if steps[0].Ordinal != 1 || steps[1].Ordinal != 2 {
		t.Errorf("the surviving ordinals = %d, %d, want 1, 2", steps[0].Ordinal, steps[1].Ordinal)
	}
	if !steps[0].At.Equal(fixedNow()) {
		t.Errorf("the surviving time = %s, want %s", steps[0].At, fixedNow())
	}
}

// TestStoreSnapshotRestores checks one snapshot round trip: the bytes a
// store writes answer every later read the store's own counters would.
// orc:ADR-0002#decision.c9
func TestStoreSnapshotRestores(t *testing.T) {
	st := newStore(t)
	for range 2 {
		if _, err := st.Append("a", fixedNow()); err != nil {
			t.Fatalf("appending: %v", err)
		}
	}
	var buf bytes.Buffer
	if err := st.SaveSnapshot(&buf); err != nil {
		t.Fatalf("saving the snapshot: %v", err)
	}
	restored, err := RestoreFrom(&buf, t.TempDir())
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	defer func() { _ = restored.Close() }()
	steps, found, err := restored.Steps("a")
	if err != nil || !found {
		t.Fatalf("reading the restored steps: found=%v err=%v", found, err)
	}
	if len(steps) != 2 {
		t.Fatalf("%d steps restored, want 2", len(steps))
	}
	if steps[0].Ordinal != 1 || !steps[0].At.Equal(fixedNow()) {
		t.Errorf("the restored first step = %+v, want ordinal 1 at %s", steps[0], fixedNow())
	}
}

// TestStorePingReadsTheStore checks readiness's own question: a read of
// the store that answers only while the store answers at all.
// orc:ADR-0002#decision.c7
func TestStorePingReadsTheStore(t *testing.T) {
	st := newStore(t)
	if _, err := st.Append("a", fixedNow()); err != nil {
		t.Fatalf("appending: %v", err)
	}
	if err := st.Ping(); err != nil {
		t.Errorf("a healthy store does not answer its ping: %v", err)
	}
}

// TestStorePingAnswersAfterCloseSoReadinessFalls checks the other side of
// readiness: a store that no longer answers read fails its ping, and the
// readiness probe the ping serves says not ready.
// orc:ADR-0002#decision.c7
func TestStorePingAnswersAfterCloseSoReadinessFalls(t *testing.T) {
	path := t.TempDir() + "/store.db"
	st, err := Open(path)
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	if err := st.Ping(); err == nil {
		t.Error("a closed store answered its ping; readiness would stay up with no store behind it")
	}
}
