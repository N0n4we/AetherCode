## Context

The router currently stores providers in `router_providers` and indexes enabled providers by `group -> model -> provider IDs`. Public relay requests can influence selection through `group` in the body or `X-Aether-Group` / `X-Router-Group` headers. That leaks an internal routing concern to users and does not match the desired operating model, where administrators maintain providers directly and one modelId can map to many providers.

This change is intentionally small: it only changes provider selection and cache semantics. It prepares later relay pipeline work but does not introduce new relay routes, protocol adaptors, accounting, realtime, or task storage.

## Goals / Non-Goals

**Goals:**

- Select providers by public modelId, not by client-visible group.
- Allow multiple providers to serve the same modelId.
- Preserve current provider CRUD, model mapping, key selection, priority, weight, disabled status, and shared DB cache sync.
- Ignore and strip client-supplied group hints in public OpenAI-compatible requests.
- Keep existing provider rows operational without requiring manual migration.

**Non-Goals:**

- Adding new relay endpoints or changing route registration.
- Introducing new provider protocol adaptors.
- Adding user entitlements, billing, accounting hooks, or token-level access control.
- Removing the `groups` column immediately. It remains tolerated legacy/internal metadata.

## Decisions

### Decision: Treat `models` as modelId list

The existing `Provider.Models` field remains the authoritative list of public modelIds a provider can serve. This avoids a schema migration for the primary matching key and keeps existing provider admin payloads valid.

Alternative considered: add a new `model_ids` column and migrate from `models`. That is clearer long term but unnecessary for the first split and adds avoidable migration risk.

### Decision: Collapse cache index to modelId first

The cache will index enabled providers by `modelId -> []providerID`, with `*` retained as wildcard fallback. Provider IDs remain sorted by descending priority and stable ID order before weighted selection among the highest-priority candidates.

Alternative considered: keep group in the cache but always pass `default`. That preserves hidden complexity and makes it easier for group semantics to leak back into public request handling.

### Decision: Preserve `groups` as internal compatibility metadata

The `groups` field will still be accepted by provider CRUD and returned in admin responses for backward compatibility, but public request selection will not read client group hints. New code should not use `groups` as the primary routing dimension.

Alternative considered: remove `groups` from the provider model. That is a breaking database/API change and should only be considered after the modelId path has been proven.

### Decision: Ignore and strip public group hints

If a public request includes `group`, `X-Aether-Group`, or `X-Router-Group`, the router will not use those values for provider selection. The request body sent upstream will not include `group`, matching the existing behavior for body group removal while removing its selection effect.

Alternative considered: reject requests containing group hints. Ignoring is less disruptive for existing clients that may send stale fields.

## Risks / Trade-offs

- [Risk] Existing deployments rely on groups for routing separation. -> Mitigation: document the behavior change and keep `groups` stored/admin-visible as internal metadata while selection moves to modelId.
- [Risk] Weighted selection tests can be flaky if they depend on random distribution. -> Mitigation: test deterministic priority/exclusion behavior directly and use bounded distribution tests only where needed.
- [Risk] Removing group from selection changes fallback behavior. -> Mitigation: add tests for exact modelId match, wildcard fallback, and no-provider errors before changing handler behavior.
- [Risk] Later relay work may need internal pools. -> Mitigation: keep internal metadata possible, but do not expose it in public request semantics.

## Migration Plan

1. Add modelId-centric cache data structures while keeping provider CRUD payloads unchanged.
2. Update provider selection APIs to accept `modelId` and excluded provider IDs without a public group argument.
3. Update OpenAI-compatible handler request parsing to ignore group hints and strip the `group` body field before upstream dispatch.
4. Add tests for existing rows, multiple providers per modelId, wildcard fallback, disabled providers, priority/weight selection, retry exclusion, and ignored group hints.
5. Update README provider examples to explain modelId-based routing and legacy/internal group metadata.

Rollback can restore the previous `Select(group, model, excluded)` path because the database schema remains backward-compatible.

## Open Questions

- Should internal admin-only routing pools be introduced later as a separate capability, or is modelId plus priority/weight sufficient?
