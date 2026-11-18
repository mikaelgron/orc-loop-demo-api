# ADR-0002: Counter API serves the counter over HTTP

---

- **Status**: Accepted
- **Author(s)**: the counter-api mission seat (an autonomous agent, per Mikael Grön's #pm-1-213 mandate of 2026-11-16)
- **Created**: 2026-11-18
- **Deciders**: Mikael Grön
- **Supersedes**: N/A
- **Superseded by**: N/A

---

## 1. Context

The counter system's first member is a counting library: a thing its
caller embeds. Every consumer that does not embed Go — an operations
script, a dashboard, another service — cannot use it at all, and asking
each of them to speak Go to count things is the cost of not having a
door. The system needs the counting reachable from wherever it is used,
behind one address, with a contract that does not change with the caller.

## 2. Decision

Counter API serves the counter over HTTP: append a step, read the
recorded steps back, ask how much, and start over.

- Every POST to a counter's steps path MUST record exactly one step,
  carrying the step's ordinal and the time it was recorded.
- A counter's first step MUST take ordinal one, and each later step's
  ordinal MUST be exactly one greater than the last.
- Reading a counter's steps MUST return every recorded step in the order
  the steps were recorded.
- Reading a counter MUST return how many steps the counter has recorded.
- Resetting a counter MUST require the admin credential.
- Every request MUST be logged with its method, its route, its response
  status and how long the answer took.
- The readiness probe MUST read the store, and answer ready only while
  the store answers.
- The liveness probe MUST answer without reading the store.
- Recorded steps MUST survive the service restarting.

## 3. Alternatives Considered

- Depending directly on the counting member's library, pinned at its
  import commit: rejected — the pin freezes a library whose own step
  log is not built yet, so the service would wait forever on another
  member's promise. The service records its own steps; the two members
  converge when their histories do.
- An in-memory counter with no persistence: rejected — steps that do
  not survive a restart are steps the service never really took.
- Serving the tally alone, without the step memory: rejected — the count
  answers "how much" but never "what happened", and a count nobody can
  audit is a number on trust.

## 4. Consequences

The service holds a bbolt file: a volume with every step on it, whose
snapshot is a byte-exact copy and whose restore is a store of its own;
backup and restore answer to the same bytes. The admin credential is a
single token from the environment — rotating it means restarting the
service, which the surviving steps make cheap. The routes are fixed at
six; a seventh route is a new decision. The one log line per request is
the service's own record, in the format the objectives are judged from.
