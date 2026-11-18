// Package counterapi serves the counter over HTTP: append a step to a
// counter, read the recorded steps back in the order they happened, ask
// how many steps a counter has taken, and start over with the admin
// credential. The service's decisions are ADR-0002 (docs/adr); the steps
// are ordinal and time, and they live in a bbolt file that survives
// restarts.
package counterapi
