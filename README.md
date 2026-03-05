# k11s

Speed-first CLI/TUI for Kubernetes navigation and operations.

Primary goals:
- Fast launch, including warm starts from a background daemon
- Fast navigation across resources and namespaces
- Non-blocking UI (no network calls on the UI thread)
- Clear stale/live state indicators
- Session restore so users continue where they left off

## Docs

- [Architecture](docs/ARCHITECTURE.md)
- [Performance Budgets](docs/PERF_BUDGETS.md)
- [RPC Contract](docs/RPC.md)
- [Failure And Degraded Mode Policy](docs/FAILURE_POLICY.md)
- [Roadmap](docs/ROADMAP.md)
- [Phase 0 Tickets](docs/PHASE0_TICKETS.md)

## Near-term plan

1. Build `k11s` client and `k11sd` daemon skeleton.
2. Implement daemon handshake + snapshot/session restore path.
3. Add first resources (`pods`, `services`, `deployments`) via watch-backed cache.
4. Ship stale-state UX in status bar with age and sync metadata.

## Current status

- `k11s` and `k11sd` binaries exist.
- `k11s` performs warm connect first, then auto-spawns `k11sd` on cold start.
- Daemon handshake enforces RPC version compatibility.
- If daemon version differs from client version, `k11s` requests graceful daemon shutdown and starts a matching daemon.
- Session state is persisted by daemon and restored on launch.
- Daemon serves `pods`, `services`, and `deployments` from a watch-backed cache with relist recovery, including detail lookups from the same cache.
- Namespace autocomplete is loaded via daemon RPC and refreshed per selected kube context.
- CRD-name autocomplete is loaded via daemon RPC and refreshed per selected kube context.
- `crds` and `crs` views are backed by daemon cache; most other resources still use placeholder responses with freshness metadata.
- Status bar renders freshness badge, age, snapshot time, source, and watch health.
- `k11s` runs an interactive Bubble Tea TUI by default, with periodic background list refresh.

## Quickstart

```bash
go build -o ./bin/k11s ./cmd/k11s
go build -o ./bin/k11sd ./cmd/k11sd
K11SD_PATH="$PWD/bin/k11sd" ./bin/k11s
```

Optional overrides:
- `K11S_SOCKET`: explicit Unix socket path.
- `K11SD_PATH`: explicit daemon binary path used for auto-spawn.
- `K11S_SESSION`: explicit session state file path.

Session override flags (persisted through daemon):
- `--context`
- `--namespace`
- `--resource`
- `--filter`
- `--selection`
- `--simulate-stale` (for stale-state visual validation)

Perf measurement:
- `k11s debug perf` prints startup output, span timings, and JSON report.
- `k11s debug perf --json-only` prints machine-readable JSON only.

TUI keybindings:
- `j` / `k` or arrow keys: move selection
- `ctrl+d` / `ctrl+u` (or PgDn/PgUp): jump selection by 10 rows
- `/`: search items; `n` / `N` jumps between matches
- `enter` (normal mode): load detail for selected row from daemon cache
- `q`: quit (current selection is persisted into session)
- `:`: open command line
- Mouse left-click in pod rows:
  - `NAMESPACE` switches namespace
  - `NODE` opens `nodes` view
  - `OWNER` opens owner resource when supported
- Rows that change between refreshes temporarily flash to highlight updates.
- In command mode autocomplete: `tab` expands/cycles suggestions, `enter`/`->` accepts current suggestion, `esc` clears suggestion.
- In command mode without autocomplete: `enter` applies the typed command.

TUI layout:
- Top untitled input field (command line, `:` to activate)
- Middle large bordered pane titled with `context > namespace > resource`
- Bottom single row:
  - Left: status box + age box
  - Right: keyboard legend
- For cluster-scoped resources (`nodes`, `namespaces`, `crds`), the middle segment renders as `<cluster>`.

Command line examples:
- `:ns payments` or `:namespace payments`
- `:ns all` (all namespaces view)
- `:ctx dev-cluster` or `:context dev-cluster`
- `:pods`, `:services`, `:deployments`, `:nodes`, `:namespaces` (switch resource view)
- `:crds` (list custom resource definitions)
- `:crs widgets.example.com` or `:crd widgets.example.com` (list CRs for a selected CRD)
- `:delete` (delete selected row), `:delete <name>`, or `:delete <namespace>/<name>` when in `all` namespace
- `:scale <replicas>` (scale selected item), or `:scale <replicas> <name>`
- `:restart` (rollout restart selected item), or `:rollout restart [name]`
- `:logs` (tail selected pod logs), or `:logs <pod-name> [tail-lines]`

Autocomplete notes:
- `:ctx ` / `:context ` suggestions are loaded from kubeconfig (`KUBECONFIG` or `~/.kube/config`).
- `:ns ` / `:namespace ` suggestions are loaded via daemon RPC and refreshed per selected kube context.
- `:crd ` / `:crs ` / `:filter ` suggestions are loaded from cached `crds` via daemon RPC and refreshed per selected kube context.
- Mutating commands require a `LIVE` view; stale or recovering views fail with `STALE_DATA` guardrail feedback.

CI checks:
- `.github/workflows/ci.yml` runs format, vet, tests, startup smoke test, and advisory perf sanity.
