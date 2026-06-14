## Why

The router currently selects providers using a public-facing group concept, but provider management should be model-centric and users should not see or control routing groups. Moving selection to `modelId -> providers[]` makes the gateway simpler to operate and creates a stable foundation for later relay pipeline work.

## What Changes

- Change provider selection semantics from group/model matching to modelId/provider matching.
- Allow one modelId to map to multiple enabled providers.
- Preserve priority, weight, disabled-provider exclusion, wildcard model support, model mapping, key selection, and provider metadata needed by current OpenAI-compatible routing.
- Ignore client-supplied group hints from request body or headers and ensure they are not forwarded upstream.
- Keep existing provider CRUD and shared DB cache sync behavior compatible with current deployments.
- Exclude new relay route registration, multi-format adaptors, accounting hooks, realtime, and async task behavior from this change.

## Capabilities

### New Capabilities

- `modelid-provider-selection`: Defines modelId-centric provider metadata, cache indexing, candidate selection, and public request behavior for hidden routing labels.

### Modified Capabilities

- None.

## Impact

- Affected code: `router/internal/store/provider.go`, `router/internal/store/cache_test.go`, `router/internal/app/openai.go`, `router/internal/app/openai_test.go`, provider admin handlers, README examples.
- Affected APIs: existing provider admin payloads remain backward-compatible; public `/v1/chat/completions` and `/v1/completions` stop honoring group hints for routing.
- Affected data model: existing `router_providers.models` remains the public modelId list; `groups` becomes legacy/internal metadata and is no longer used for public request selection.
- No new external dependencies are expected.
