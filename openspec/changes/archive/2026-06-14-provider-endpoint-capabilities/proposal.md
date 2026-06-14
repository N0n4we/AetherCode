## Why

After provider selection becomes modelId-centric and the OpenAI-compatible pipeline is isolated, the router still needs a way to know which providers can serve which endpoint families. Adding explicit provider endpoint capabilities lets later route registration select compatible providers without overloading model names or hidden groups.

## What Changes

- Add provider metadata for endpoint capabilities, channel type, and optional relay format family.
- Keep existing provider admin payloads backward-compatible by defaulting existing providers to OpenAI-compatible chat and text completions.
- Extend cache stats and provider lookup so future route handlers can filter by modelId plus endpoint capability.
- Preserve current `/v1/chat/completions` and `/v1/completions` behavior.
- Exclude new route registration, non-OpenAI adaptors, accounting hooks, realtime, and async task behavior from this change.
- Depend on `modelid-provider-selection` and fit after `relay-pipeline-openai-compatible` in the implementation sequence.

## Capabilities

### New Capabilities

- `provider-endpoint-capabilities`: Defines provider metadata and cache behavior for endpoint capability matching.

### Modified Capabilities

- None.

## Impact

- Affected code: `router/internal/store/provider.go`, provider admin handlers, cache stats/tests, README examples, k3d seed payloads if they validate provider JSON strictly.
- Affected APIs: provider admin payloads gain optional fields; existing payloads continue to work.
- Affected data model: `router_providers` gains nullable/defaulted capability metadata.
- No new external dependencies are expected.
