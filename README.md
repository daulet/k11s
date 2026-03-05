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
- Daemon serves `pods`, `services`, and `deployments` from an async cache refreshed via `client-go` in the background.
- Namespace autocomplete is loaded via daemon RPC and refreshed per selected kube context.
- Other resources currently use placeholder responses with freshness metadata.
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
- `q`: quit (current selection is persisted into session)
- `:`: open command line

TUI layout:
- Top untitled input field (command line, `:` to activate)
- Middle large bordered pane titled with `context > namespace > resource`
- Bottom single row:
  - Left: status box + age box
  - Right: keyboard legend

Command line examples:
- `:ns payments` or `:namespace payments`
- `:ctx dev-cluster` or `:context dev-cluster`
- `:pods`, `:services`, `:deployments` (switch resource view)

Autocomplete notes:
- `:ctx ` / `:context ` suggestions are loaded from kubeconfig (`KUBECONFIG` or `~/.kube/config`).
- `:ns ` / `:namespace ` suggestions are loaded via daemon RPC and refreshed per selected kube context.

CI checks:
- `.github/workflows/ci.yml` runs format, vet, tests, startup smoke test, and advisory perf sanity.
