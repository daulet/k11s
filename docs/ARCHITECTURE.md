# Architecture

## 1) System shape

`k11s` is split into two binaries:

- `k11s` (foreground client): input, rendering, view state, command dispatch.
- `k11sd` (background daemon): Kubernetes API I/O, watches, caches, indexing, session persistence.

This split keeps UI startup and interaction fast, even when clusters are slow.

## 2) Responsibilities by process

### `k11s` client

- Start fast and render an initial view from daemon snapshot.
- Keep event loop responsive (no blocking network or disk calls).
- Request list/detail/action operations via RPC.
- Display freshness status (`LIVE`, `CATCHING_UP`, `STALE`) per active view.
- Persist ephemeral UI-only state (pane focus, cursor position) via daemon session API.

### `k11sd` daemon

- Hold authenticated client-go clients per kube context.
- Maintain watch-backed caches for hot resources.
- Persist cache snapshots and session state to disk.
- Serve list/detail/action RPCs from cache when possible.
- Reconcile cache via watch streams + controlled relists on watch failures.

## 3) Startup and lifecycle

### Warm start (daemon running)

1. `k11s` connects to Unix socket.
2. Performs version/compat handshake.
3. Requests `GetSession + GetSnapshot`.
4. Renders within target budget.
5. Subscribes to incremental updates.

### Cold start (daemon not running)

1. `k11s` attempts daemon connect and times out quickly.
2. Spawns daemon process.
3. Loads last persisted snapshot if present.
4. Renders stale snapshot immediately.
5. Transitions to live data as daemon catches up.

## 4) Concurrency model

- UI thread: render + input only.
- UI worker goroutines: query dispatch, local filtering/sorting, cancellation handling.
- Daemon worker pools:
  - Watch ingest workers per (cluster, GVR, namespace scope)
  - Index update workers
  - RPC serving workers
  - Action executor workers (bounded concurrency)
- All async calls support cancellation via request IDs and context propagation.

## 5) Cache design

Two-tier cache:

1. In-memory hot cache (fast reads for active context/resource sets).
2. Persistent store (SQLite preferred) for restart continuity.

Cache keys:

`(cluster, namespace_scope, gvr, query_hash, sort_key, filter_hash)`

Stored metadata:

- `resourceVersion` or equivalent checkpoint
- `last_successful_sync_at`
- `last_event_at`
- `watch_health`
- `snapshot_age`

Invalidation/reconciliation:

- Primary path: watch event apply.
- On watch gap/410 Gone: relist and resume watch.
- TTL fallback for low-priority resource types.
- Explicit refresh command always available.

## 6) Freshness and stale-state UX

Each rendered view includes freshness metadata from daemon.

Status states:

- `LIVE`: watch healthy and age within threshold.
- `CATCHING_UP`: reconnecting/relisting.
- `STALE`: data is older than threshold or watch unhealthy.

Status bar content:

- state badge
- "as of <timestamp>"
- age counter (`N s old`)
- source (`cache/live`)

Actions with freshness requirements (delete, rollout, patch) perform revalidation if view is stale.

## 7) Session restore

Persisted session fields:

- kube context
- namespace
- current resource type (pods/services/crds/crs/etc.)
- active filters/search
- selected row/item
- split or panel state
- navigation history stack

On startup, restore session before live sync to avoid blank initial experience.

## 8) Failure model

- Daemon down: client can auto-spawn daemon and retry.
- Cluster unreachable: cached data remains browsable with stale marker.
- Watch broken: transition `LIVE -> CATCHING_UP -> LIVE/STALE`.
- Corrupt cache/session store: fallback to safe defaults and continue.

Detailed policy: [Failure And Degraded Mode Policy](FAILURE_POLICY.md)

## 9) Suggested Go module layout

```text
cmd/
  k11s/
  k11sd/
internal/
  app/              # client composition/bootstrap
  ui/               # rendering, keybindings, view model
  rpc/              # protobuf and rpc handlers/clients
  daemon/           # daemon lifecycle and supervisors
  kube/             # client-go wrappers, watch/relist logic
  cache/
    memory/
    sqlite/
  index/            # secondary indexes for fast nav/search
  session/          # persisted restore data
  model/            # shared resource/view models
  perf/             # metrics/timing helpers
```
