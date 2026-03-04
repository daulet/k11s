# RPC Contract (Draft v0)

Transport recommendation:
- gRPC over Unix domain socket (`/tmp/k11sd.sock` or XDG runtime dir)
- Protobuf messages for typed evolvable contracts
- Server-streaming for watch/update channels

## 1) Core services

### `DaemonService`

- `Handshake(HandshakeRequest) -> HandshakeResponse`
- `GetHealth(HealthRequest) -> HealthResponse`
- `SubscribeHealth(HealthSubscribeRequest) -> stream HealthEvent`

### `SessionService`

- `GetSession(GetSessionRequest) -> SessionState`
- `SaveSession(SaveSessionRequest) -> SaveSessionResponse`

### `ResourceService`

- `ListResources(ListResourcesRequest) -> ListResourcesResponse`
- `WatchResources(WatchResourcesRequest) -> stream ResourceEvent`
- `GetResource(GetResourceRequest) -> GetResourceResponse`

### `ActionService`

- `ExecuteAction(ActionRequest) -> ActionResponse`

## 2) Freshness metadata (required on list/get responses)

Every response includes:
- `freshness_state` (`LIVE`, `CATCHING_UP`, `STALE`)
- `snapshot_time_unix_ms`
- `age_ms`
- `watch_healthy` (bool)
- `source` (`CACHE`, `LIVE_MIXED`, `LIVE`)

## 3) Request identity and cancellation

- Every request carries `request_id`.
- Client can cancel in-flight requests.
- Daemon should coalesce duplicate in-flight list requests with same key.

## 4) Minimal message sketch

```proto
message ResourceScope {
  string kube_context = 1;
  string namespace = 2; // empty means all
  string group = 3;
  string version = 4;
  string resource = 5;  // plural, e.g. pods
}

message ListResourcesRequest {
  string request_id = 1;
  ResourceScope scope = 2;
  string filter = 3;
  string sort_by = 4;
  int32 page_size = 5;
  string page_token = 6;
}

enum FreshnessState {
  FRESHNESS_UNSPECIFIED = 0;
  LIVE = 1;
  CATCHING_UP = 2;
  STALE = 3;
}

message FreshnessMeta {
  FreshnessState state = 1;
  int64 snapshot_time_unix_ms = 2;
  int64 age_ms = 3;
  bool watch_healthy = 4;
  string source = 5;
}
```

## 5) Versioning

- Handshake includes `client_version`, `rpc_version`, `feature_flags`.
- Daemon rejects incompatible major `rpc_version`.
- Additive protobuf fields only within a minor line.

## 6) Error model

Error classes:
- `TRANSIENT` (retryable, e.g. apiserver timeout)
- `STALE_DATA` (action requires freshness revalidation)
- `AUTH` (context auth invalid)
- `NOT_FOUND` / `VALIDATION`
- `INTERNAL`

Return machine-readable code + concise human message.
