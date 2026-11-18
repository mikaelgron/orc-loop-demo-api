# orc-loop-demo-api

Counter API — the counter system's second member: an HTTP service that
serves the counter's counting. Append a step to a counter, read the
recorded steps back in the order they happened, ask how many steps a
counter has taken, and start over with the admin credential.

This repo is the phase-2 exercise of the orc drive (Mikael Grön's
#pm-1-213 mandate, 2026-11-16): the first member of the counter system
was [orc-loop-demo](https://github.com/mikaelgron/orc-loop-demo), a
tiny counting library, driven to its gates by orc. This member is
deliberately worse for the gates: a service with routes to authorize and
probe, two third-party dependencies for the supply chain to account for
(the chi router and the bbolt store), more code for the mutation bar to
hold, and its own decisions to cite — ADR-0002, below.

The service's decisions live in `docs/adr/0002-adr-counter-api-serves-the-counter-over-http.md`;
every behavior test and every behavior-bearing symbol carries its
`orc:ADR-0002#…` citation. The member's declarations — its checks, its
route list, its objectives — live in `.orc/`.

## Run it

```sh
COUNTER_API_ADMIN_TOKEN=desired-admin-token go run ./cmd/counter-api
```

The service listens on `:8080` (flags: `-addr`, `-store`), keeps its
steps in `counter-api.boltdb`, and answers:

- `POST /counters/{id}/steps` — record one step; answers the step
- `GET /counters/{id}/steps` — the recorded steps, oldest first
- `GET /counters/{id}` — the counter's tally
- `DELETE /counters/{id}` — reset the counter (admin credential:
  `X-Admin-Token`)
- `GET /healthz` — liveness, no store read
- `GET /readyz` — readiness, reads the store

```sh
curl -sX POST localhost:8080/counters/purdy/steps
curl -s localhost:8080/counters/purdy/steps
curl -s localhost:8080/counters/purdy
curl -sX DELETE -H 'X-Admin-Token: desired-admin-token' localhost:8080/counters/purdy
curl -s localhost:8080/readyz     # {"status":"ready"}
curl -s localhost:8080/healthz    # {"status":"ok"}
```

A counter id is lowercase letters, digits and hyphens, at most 64 long.
Reading a counter that was never written answers 404; a step's time is
the server's clock at the append.

## Verify it

The member's checks, as orc's runner runs them (`.orc/checks.yaml`):

```sh
go mod download && go build ./... && go test ./...
```

— the checks.yaml also declares the module fetch that fills the cache
the build and the tests read, and gotestsum's JUnit report that the
loop reads back.
