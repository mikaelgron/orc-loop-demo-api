package counterapi

import (
	"encoding/json"
	"testing"
)

// TestStepJSONCarriesOrdinalAndTime checks the step's own pair — the
// ordinal and the recorded time — through the bytes the store keeps and
// the API answers with.
// orc:ADR-0002#decision.c1
func TestStepJSONCarriesOrdinalAndTime(t *testing.T) {
	step := newStep(2, fixedNow())
	b, err := step.json()
	if err != nil {
		t.Fatalf("the step's JSON: %v", err)
	}
	back, err := stepFromJSON(b)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if back.Ordinal != 2 {
		t.Errorf("ordinal = %d, want 2", back.Ordinal)
	}
	if !back.At.Equal(fixedNow()) {
		t.Errorf("time = %s, want %s", back.At, fixedNow())
	}

	// The JSON the wire sees names the two fields of the step: the
	// ordinal and the recorded time.
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("the step's JSON as the wire sees it: %v", err)
	}
	if _, ok := raw["ordinal"]; !ok {
		t.Errorf("the step's JSON carries no ordinal: %s", b)
	}
	if _, ok := raw["at"]; !ok {
		t.Errorf("the step's JSON carries no at: %s", b)
	}
}
