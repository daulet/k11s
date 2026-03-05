# Failure And Degraded Mode Policy

This document defines expected behavior when core dependencies fail.

## Goals

- Keep `k11s` usable whenever safe.
- Avoid silent failure paths.
- Make degraded state obvious in UI.
- Preserve user progress where possible.

## Failure classes and behavior

### 1) Daemon socket/connect failures

Cases:
- daemon not running
- stale socket file
- permission errors on socket path

Behavior:
- client attempts daemon auto-start.
- if connect still fails, return explicit startup error with socket path.
- do not hang waiting indefinitely for daemon readiness.

User-facing:
- startup error includes likely cause and socket path.

### 2) Version and RPC incompatibility

Cases:
- client and daemon RPC versions differ
- daemon binary version differs from client

Behavior:
- RPC mismatch: fail fast with compatibility error.
- binary mismatch with compatible RPC: request graceful daemon shutdown and start matching daemon.

User-facing:
- explicit mismatch message includes client and daemon versions.

### 3) Session store corruption or I/O failure

Cases:
- unreadable/corrupt session JSON
- directory/file write errors

Behavior:
- on corrupt file: fall back to safe default session values.
- on write failure: keep current in-memory session but report save error.

User-facing:
- warn when defaults are used due to corruption.

### 4) Kubernetes auth/connectivity failures

Cases:
- expired kube credentials
- context misconfiguration
- API server unreachable

Behavior:
- keep last known data visible when possible.
- mark view freshness as `STALE` and watch health as degraded.
- refuse mutating operations that require fresh confirmation (future phase).

User-facing:
- clear error message and stale status indication.

### 5) Watch/relist failures (future watch-backed cache)

Cases:
- watch stream disconnect
- `410 Gone` / resourceVersion invalid
- relist failures

Behavior:
- transition freshness `LIVE -> CATCHING_UP -> LIVE/STALE`.
- continue serving last known snapshot during recovery.
- bounded retries with backoff.

User-facing:
- status area always shows freshness state and data age.

## Degraded mode UX rules

- Status and age must always be visible.
- `STALE` must be visually distinct from `LIVE`.
- Commands that cannot be safely completed must fail with actionable messages.
- Navigation should remain available on cached data whenever possible.

## Logging and diagnostics

- log daemon lifecycle events and handshake outcomes.
- log session corruption fallback usage.
- expose startup timing and span data via `k11s debug perf`.

## Scope note

This policy is currently enforced for daemon/bootstrap/session and placeholder data paths.
Additional enforcement for mutating actions and full watch cache recovery will be added in later phases.
