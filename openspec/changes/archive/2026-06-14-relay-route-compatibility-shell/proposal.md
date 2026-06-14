## Why

After modelId-centric provider selection, an OpenAI-compatible relay pipeline, and provider endpoint capabilities are in place, the router still needs a predictable route surface for new-api compatibility work. A route compatibility shell lets clients and later implementation steps see which endpoint families are recognized without pretending that Claude, Gemini, media, realtime, or async task adaptors already exist.

## What Changes

- Add a route registry that maps method/path patterns to route family, endpoint capability, response format, and implementation status.
- Keep `/v1/chat/completions` and `/v1/completions` implemented through the OpenAI-compatible relay pipeline.
- Add model discovery routes backed by provider cache metadata: `/v1/models`, `/v1/models/{model}`, `/v1beta/models`, and `/v1beta/openai/models`.
- Register unsupported shells for core new-api route families such as OpenAI responses, embeddings, images, audio, rerank, Claude messages, Gemini generate routes, realtime, and video/task endpoints.
- Return a structured unsupported-endpoint error for registered-but-unimplemented routes instead of a generic 404.
- Keep unknown paths as normal not-found responses.
- Hide group concepts from route inputs, outputs, logs intended for users, and error payloads.
- Depend on `modelid-provider-selection`, `relay-pipeline-openai-compatible`, and `provider-endpoint-capabilities`.

## Capabilities

### New Capabilities

- `relay-route-compatibility-shell`: Defines the route registry, model discovery endpoints, and unsupported-route behavior for recognized but unimplemented endpoint families.

### Modified Capabilities

- None.

## Impact

- Affected code: `router/internal/app/server.go`, route registration, OpenAI error helpers, cache/model listing helpers, router tests, README route matrix.
- Affected APIs: existing completion routes remain compatible; model list routes are added; registered unsupported routes return stable JSON errors.
- Affected internals: route registration becomes descriptor-driven enough for future adaptor work.
- No new external dependencies are expected.
