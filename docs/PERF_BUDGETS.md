# Performance Budgets

These budgets are release-blocking for the "speed-first" goal.

## 1) Startup

- Cold start (daemon not running): time-to-interactive <= 800 ms (p95, local machine)
- Warm start (daemon running): time-to-interactive <= 120 ms (p95)
- Session restore paint <= 60 ms from client process start (warm path)

## 2) Interaction

- Keypress to visual response <= 16 ms (p95) on cached views
- Resource list switch (pods -> services) <= 50 ms (p95) from cache
- Filter/search update <= 30 ms (p95) for up to 10k cached rows

## 3) Data freshness and update

- Watch event to UI update <= 120 ms (p95) on healthy connection
- Watch reconnect detection <= 2 s
- Recovery to `LIVE` after reconnect <= 10 s for hot resources (cluster dependent)

## 4) Resource and memory targets

- Daemon steady-state memory target: <= 250 MB on one medium cluster
- Client memory target: <= 120 MB
- CPU idle target: near-zero when no input/events

## 5) Measurement methodology

- Built-in benchmark command: `k11s debug perf`
- Required outputs:
  - startup timings (connect, handshake, session load, first paint)
  - interaction timings (input -> render)
  - cache hit ratios per resource type
  - watch health and reconnect counts

## 6) Gate criteria for initial GA

1. Warm startup and interaction budgets pass in CI perf job (or nightly perf env).
2. No synchronous Kubernetes API call in UI event/render code path.
3. Staleness badge and age metadata shown for all data views.
