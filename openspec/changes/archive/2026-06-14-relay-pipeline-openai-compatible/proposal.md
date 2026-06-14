## Why

The router's current OpenAI-compatible handlers combine request validation, provider selection, retry, upstream dispatch, and response copying in one path. A narrow relay pipeline for the existing chat/completions endpoints creates the tracer bullet needed for later route expansion without taking on Claude, Gemini, media, realtime, or task complexity.

## What Changes

- Replace the current `/v1/chat/completions` and `/v1/completions` handler path with a small relay pipeline.
- Introduce internal relay primitives for request envelope, route format, selected provider metadata, adaptor result, retry state, and committed-response tracking.
- Preserve existing OpenAI-compatible behavior: model mapping, custom auth headers, extra headers, provider response headers, streaming flush, request body size limit, API key auth, and `X-Aether-*` response metadata.
- Add replayable request body handling so retries can resend the original request to another provider.
- Normalize OpenAI-compatible errors for invalid requests, missing model, no provider, upstream failure, and unsupported internal states.
- Add retry rules that retry before response commit for network errors, `429`, and `5xx`, and do not retry committed streaming responses.
- Depend on the modelId-centric provider selection semantics from `modelid-provider-selection`.
- Exclude new route families, route compatibility shell, accounting hooks, realtime WebSocket, async task storage, and non-OpenAI adaptors.

## Capabilities

### New Capabilities

- `openai-compatible-relay-pipeline`: Defines the relay pipeline behavior for the existing OpenAI-compatible chat and text completion routes.

### Modified Capabilities

- None.

## Impact

- Affected code: `router/internal/app/server.go`, `router/internal/app/openai.go`, `router/internal/app/errors.go`, `router/internal/upstream/client.go`, router tests, mock provider fixtures, README.
- Affected APIs: existing `/v1/chat/completions` and `/v1/completions` should remain source-compatible for clients except that public group hints are ignored by the prerequisite selection change.
- Affected internals: request handling becomes a reusable pipeline, but only OpenAI-compatible completion routes are registered by this change.
- No new external dependencies are expected unless streaming/response commit detection requires a small local helper.
