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

## Quickstart

```bash
go build -o ./bin/k11s ./cmd/k11s
go build -o ./bin/k11sd ./cmd/k11sd
K11SD_PATH="$PWD/bin/k11sd" ./bin/k11s
```

Optional overrides:
- `K11S_SOCKET`: explicit Unix socket path.
- `K11SD_PATH`: explicit daemon binary path used for auto-spawn.
