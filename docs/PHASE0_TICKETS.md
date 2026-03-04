# Phase 0 Tickets

## T0-01 Initialize Go workspace

Scope:
- Create `go.mod`
- Add `cmd/k11s` and `cmd/k11sd` entrypoints
- Wire shared config package

Acceptance criteria:
- `go build ./...` succeeds
- Running `k11s` prints version/build info and starts bootstrap flow

## T0-02 Daemon lifecycle and socket handshake

Scope:
- Unix socket location strategy (XDG runtime dir fallback)
- `k11s` connect with short timeout
- Auto-spawn daemon if unavailable
- Handshake with client/daemon compatibility check

Acceptance criteria:
- Warm path connects without respawn
- Cold path spawns daemon and reconnects automatically
- Incompatible versions produce clear error

## T0-03 Session persistence schema

Scope:
- Define `SessionState` struct and storage schema
- Implement `GetSession` and `SaveSession`
- Persist and restore context/namespace/resource/filter/selection

Acceptance criteria:
- Restart restores last location and selection
- Corrupt session store falls back to defaults safely

## T0-04 Perf instrumentation

Scope:
- Add startup timer spans: process start, daemon connect, handshake, session load, first paint
- Add debug command to print timings
- Emit machine-readable JSON for automation

Acceptance criteria:
- `k11s debug perf` prints timing breakdown
- First paint timing is captured reliably

## T0-05 Freshness model skeleton

Scope:
- Define `FreshnessState` enum
- Include freshness metadata in placeholder list responses
- Render status bar badge and age text in client

Acceptance criteria:
- Status bar always shows freshness state
- Manual stale simulation is visually distinct

## T0-06 Baseline CI checks

Scope:
- Add build/test/lint workflow
- Include simple startup smoke test
- Add a non-blocking perf sanity gate (initially advisory)

Acceptance criteria:
- PRs fail on build/test failures
- Startup smoke test validates daemon handshake path

## T0-07 Risk review and fallback policy

Scope:
- Document failure handling for socket, auth, and cache corruption
- Add user-facing degraded mode behavior rules

Acceptance criteria:
- Failure policy is documented and linked in architecture doc

## Suggested order

1. T0-01
2. T0-02
3. T0-03
4. T0-04
5. T0-05
6. T0-06
7. T0-07
