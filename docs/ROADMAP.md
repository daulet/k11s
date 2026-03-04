# Roadmap

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

## Phase 1: Core resource navigation (Weeks 2-3)

Goals:
- Implement watch-backed cache for pods/services/deployments.
- List and detail views served from cache.
- Staleness metadata shown in status bar.

Exit criteria:
- Navigation between core resource tabs is cache-fast.
- UI never blocks on kube API calls.
- `LIVE/CATCHING_UP/STALE` states behave correctly.

## Phase 2: CRD and CR support (Week 4)

Goals:
- Discover CRDs dynamically.
- Navigate CR instances by selected CRD.
- Cache/index CR lists similarly to core resources.

Exit criteria:
- User can browse CRDs and associated CRs with same freshness semantics.

## Phase 3: Mutating actions and guardrails (Week 5)

Goals:
- Add delete, scale, rollout restart, logs stream entry points.
- Revalidate stale views before mutating actions.
- Add clear action feedback and rollback-safe error handling.

Exit criteria:
- Mutations are safe, observable, and consistent with freshness policy.

## Phase 4: Hardening and tuning (Ongoing)

Goals:
- Memory pressure controls and eviction tuning.
- Watch failure resilience improvements.
- Expanded perf regressions and cluster-scale testing.

Exit criteria:
- Meets release perf budgets under representative cluster load.
