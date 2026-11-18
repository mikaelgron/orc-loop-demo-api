package counterapi

import (
	"encoding/json"
	"time"
)

// Step is one recorded step: the ordinal it took and the time it was
// recorded — the pair ADR-0002 asks every append to carry.
// orc:ADR-0002#decision.c1
type Step struct {
	Ordinal uint64    `json:"ordinal"`
	At      time.Time `json:"at"`
}

// newStep makes the step for ordinal n recorded at t.
func newStep(n uint64, t time.Time) Step { return Step{Ordinal: n, At: t} }

// json is the step's own bytes: the store and the API answers write the
// same bytes, so the two can never drift.
func (s Step) json() ([]byte, error) { return json.Marshal(s) }

// stepFromJSON reads a step back from its bytes.
func stepFromJSON(b []byte) (Step, error) {
	var s Step
	if err := json.Unmarshal(b, &s); err != nil {
		return Step{}, err
	}
	return s, nil
}
