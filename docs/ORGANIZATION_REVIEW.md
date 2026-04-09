# Organization Review And Follow-up Plan

Last updated: 2026-04-08

## Objective

Track repository organization improvements that keep functionality unchanged while improving maintainability, ownership boundaries, and change velocity.

## Constraints

- No behavior or UX changes.
- No protocol changes.
- Keep all existing tests passing.
- Prefer small, reviewable refactor PRs.

## Baseline Snapshot

- `internal/ui/app.go`: 9,341 LOC
- `internal/ui/app_test.go`: 8,233 LOC
- `internal/kube/resources.go`: 1,179 LOC
- `internal/kube/resource_detail.go`: 1,053 LOC
- `internal/daemon/server.go`: 667 LOC
- `internal/protocol/handshake.go`: 434 LOC

## Prioritized Findings

1. `internal/ui` is a god-package with state model, update loop, command parsing, shell integration, and rendering mixed in one file.
2. UI tests are concentrated in one large file, making targeted ownership and maintenance difficult.
3. `internal/kube/resources.go` mixes discovery/watch transport and presentation mapping helpers.
4. `internal/kube/resource_detail.go` mixes enrichment flow, field flattening, YAML shaping, and owned-children caching/indexing.
5. `internal/daemon/server.go` centralizes routing/validation/normalization/action logic in one switch-heavy file.
6. `internal/protocol/handshake.go` combines many message types and response builders in one unit.
7. Query normalization and resource alias/scope logic are duplicated across UI/daemon/cache/kube layers.
8. Docs drift from implementation shape (`docs/ARCHITECTURE.md` and `docs/RPC.md` represent target-state, not current-state).
9. CI/release shell flows duplicate setup/verification logic.

## Workstreams

### WS1: Split UI Runtime Surface

Goal: reduce `internal/ui/app.go` from one monolith to focused files.

Planned file split:

- `internal/ui/model.go`
- `internal/ui/update_main.go`
- `internal/ui/update_modes.go`
- `internal/ui/commands.go`
- `internal/ui/actions_ui.go`
- `internal/ui/logs_ui.go`
- `internal/ui/view_main.go`
- `internal/ui/view_detail.go`
- `internal/ui/autocomplete.go`
- `internal/ui/resources_catalog.go`

Acceptance:

- `ui.Run` entrypoint unchanged.
- Zero behavior change (golden behavior preserved by existing tests).
- No file > ~2,000 LOC unless justified.

### WS2: Split UI Test Surface

Goal: break `internal/ui/app_test.go` into domain-focused suites.

Planned test files:

- `internal/ui/commands_test.go`
- `internal/ui/navigation_test.go`
- `internal/ui/render_test.go`
- `internal/ui/actions_test.go`
- `internal/ui/logs_test.go`
- `internal/ui/autocomplete_test.go`

Acceptance:

- Test names unchanged where possible.
- Same test count and pass rate.

### WS3: Split Kube Resource Fetcher Concerns

Goal: separate discovery/watch/list mechanics from row-mapping/display helpers.

Planned file split:

- `internal/kube/discovery.go`
- `internal/kube/watch_loop.go`
- `internal/kube/resource_list_fetch.go`
- `internal/kube/resource_item_mapper.go`
- `internal/kube/age_format.go`

Acceptance:

- `ResourceFetcher` public behavior unchanged.
- Existing kube tests pass without rewrites except imports/helpers.

### WS4: Split Kube Detail Enricher Concerns

Goal: isolate detail fetch/enrichment from owned-children cache/index mechanics.

Planned file split:

- `internal/kube/resource_detail_fetch.go`
- `internal/kube/resource_detail_fields.go`
- `internal/kube/resource_detail_children_cache.go`
- `internal/kube/resource_detail_yaml.go`

Acceptance:

- `ResourceDetailEnricher.Enrich` output unchanged for existing tests.

### WS5: Split Daemon RPC Routing

Goal: replace single large request switch with handler-focused files.

Planned file split:

- `internal/daemon/router.go`
- `internal/daemon/handlers_session.go`
- `internal/daemon/handlers_resource.go`
- `internal/daemon/handlers_action.go`
- `internal/daemon/handlers_logs.go`
- `internal/daemon/query_normalization.go`

Acceptance:

- Socket protocol unchanged.
- Existing daemon/client integration behavior preserved.

### WS6: Split Protocol Surface

Goal: make protocol artifacts discoverable and easier to evolve.

Planned file split:

- `internal/protocol/intents.go`
- `internal/protocol/types_session.go`
- `internal/protocol/types_resource.go`
- `internal/protocol/types_actions.go`
- `internal/protocol/types_logs.go`
- `internal/protocol/response_builders.go`

Acceptance:

- JSON shape unchanged.
- `RPCVersion` and handshake behavior unchanged.

### WS7: Centralize Resource And Query Normalization

Goal: remove duplicated aliases/scope/normalization logic.

Planned package:

- `internal/resourcecatalog` with shared:
  - alias canonicalization
  - namespace-scope metadata
  - normalization helpers

Adopters:

- `internal/ui`
- `internal/daemon`
- `internal/cache/resources`
- `internal/kube`

Acceptance:

- No duplicated alias tables in UI/daemon.
- Existing command/query behavior unchanged.

### WS8: Docs Alignment

Goal: keep docs explicit about current-state vs target-state.

Planned updates:

- Add "Current Implementation" sections to:
  - `docs/ARCHITECTURE.md`
  - `docs/RPC.md`
- Keep target-state content, but label it as "Target Design".

Acceptance:

- No ambiguity between current protocol/runtime and future plan.

### WS9: CI/Script Reuse

Goal: remove duplicated shell/setup logic.

Planned changes:

- Consolidate repeated verify logic into one script or composite action.
- Reuse common startup harness in `scripts/smoke_startup.sh` and `scripts/perf_sanity.sh`.

Acceptance:

- Same CI coverage.
- Fewer duplicated command blocks.

## Tracking Checklist

- [ ] WS1 split UI runtime surface
- [ ] WS2 split UI test surface
- [ ] WS3 split kube resource fetcher concerns
- [ ] WS4 split kube detail enricher concerns
- [ ] WS5 split daemon RPC routing
- [ ] WS6 split protocol surface
- [ ] WS7 centralize resource/query normalization
- [ ] WS8 docs alignment
- [ ] WS9 CI/script reuse

## Suggested PR Sequence

1. WS1 + WS2 (UI only, mechanical split, no logic edits).
2. WS3 (kube resources split).
3. WS4 (kube detail split).
4. WS5 (daemon split).
5. WS6 (protocol split).
6. WS7 (shared catalog + adoptions).
7. WS8 + WS9 (docs/CI cleanup).

## Definition Of Done

- All current tests pass in CI.
- No user-visible behavior changes.
- Large files reduced to manageable ownership units.
- Docs clearly distinguish current-state and target-state.
