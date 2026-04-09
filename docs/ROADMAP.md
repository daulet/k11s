# Roadmap

## Reassessment (April 8, 2026)

Overall phase status:
- Phase 0: Complete
- Phase 1: Mostly complete
- Phase 2: Complete
- Phase 3: Mostly complete
- Phase 4: Partial

Most important findings:
- Correctness blocker: a reproducible data race remains in resource watch idle reaping (`internal/cache/resources/cache.go:321`), caught by `go test -race`.
- Perf gates are only partially enforced in CI: startup checks exist, but interaction/watch/memory budgets from `docs/PERF_BUDGETS.md` are not yet gated.
- Action-path behavior is implemented broadly, but daemon action test coverage is still thin relative to the mutation surface.

Benchmark snapshot from latest reassessment:
- Warm startup (`k11s debug perf`) p95 `totalStartupMs`: 2 ms (budget: <= 120 ms)
- Cold startup (`k11s debug perf`) p95 `totalStartupMs`: 206 ms (budget: <= 800 ms)
- Warm first paint p95: 1 ms (budget: <= 60 ms)

## Phase 0: Foundation and instrumentation (Week 1)

Goals:
- Create client/daemon skeleton.
- Implement handshake and health reporting.
- Add perf timers for startup and first paint.
- Define session persistence schema.

Exit criteria:
- `k11s` starts, connects/spawns daemon, and renders a placeholder view.
- Timings are printed via debug command.
- Session load/save round trip works.

Completeness (April 8, 2026): Complete

Evidence:
- Client/daemon bootstrap, handshake, session persistence, and startup timing recorder are all active and covered by tests.
- `k11s debug perf` emits machine-readable startup spans and totals used by smoke/perf checks.

## Phase 1: Core resource navigation (Weeks 2-3)

Goals:
- Implement watch-backed cache for pods/services/deployments.
- List and detail views served from cache.
- Staleness metadata shown in status bar.

Exit criteria:
- Navigation between core resource tabs is cache-fast.
- UI never blocks on kube API calls.
- `LIVE/CATCHING_UP/STALE` states behave correctly.

Completeness (April 8, 2026): Mostly complete

Evidence:
- Watch-backed cache and freshness state rendering are active, and targeted repeated tests for warm cache tab-switch paths pass.
- UI resource loading is asynchronous via command-based loaders (no direct kube client calls in UI code paths).

Open gaps:
- Data race in cache watch-idle reaper path prevents this phase from being marked fully correct under concurrency stress.

## Phase 2: CRD and CR support (Week 4)

Goals:
- Discover CRDs dynamically.
- Navigate CR instances by selected CRD.
- Cache/index CR lists similarly to core resources.

Exit criteria:
- User can browse CRDs and associated CRs with same freshness semantics.

Completeness (April 8, 2026): Complete

Evidence:
- CRDs and CRs are listed and watched.
- CR selection by CRD filter and CR detail resolution are implemented with the same freshness model used by core resources.

## Phase 3: Mutating actions and guardrails (Week 5)

Goals:
- Add delete, scale, rollout restart, logs stream entry points.
- Revalidate stale views before mutating actions.
- Add clear action feedback and rollback-safe error handling.

Exit criteria:
- Mutations are safe, observable, and consistent with freshness policy.

Completeness (April 8, 2026): Mostly complete

Evidence:
- Stale-view guardrail is enforced before mutations (`STALE_DATA` result on non-LIVE views).
- Delete/scale/rollout-restart/label/annotate actions are wired end-to-end.
- Bulk action flows and action feedback are implemented in the TUI.

Open gaps:
- Automated daemon/action coverage is still narrow compared with current mutation surface and error branches.

## Phase 4: Hardening and tuning (Ongoing)

Goals:
- Memory pressure controls and eviction tuning.
- Watch failure resilience improvements.
- Expanded perf regressions and cluster-scale testing.

Exit criteria:
- Meets release perf budgets under representative cluster load.

Completeness (April 8, 2026): Partial

Evidence:
- Startup smoke and startup perf sanity checks are in CI and currently pass comfortably vs startup budgets.

Open gaps:
- CI does not yet gate key interaction budgets, watch reconnect/recovery SLOs, or memory targets from `docs/PERF_BUDGETS.md`.
- Expanded perf regression and representative cluster-scale benchmarks are still pending.
