## Context

The current OpenAI-compatible path lives mostly in `router/internal/app/openai.go`. It validates method/auth, reads and parses the body, extracts model/group, selects a provider, rewrites the upstream model, retries retryable upstream failures, copies headers, and streams the response. This works for two endpoints, but the responsibilities are packed into one handler and are hard to extend safely.

This change is the second split from the larger new-api relay alignment roadmap. It assumes provider selection is modelId-centric as defined by `modelid-provider-selection`, but it does not introduce new route families or non-OpenAI adaptors.

## Goals / Non-Goals

**Goals:**

- Move `/v1/chat/completions` and `/v1/completions` onto a small relay pipeline.
- Preserve existing client-visible behavior for successful non-streaming and streaming OpenAI-compatible requests.
- Make request body replay, retry decisions, committed-response detection, upstream dispatch, and error normalization explicit and testable.
- Create internal primitives that can later support more relay formats without implementing those formats now.

**Non-Goals:**

- Registering additional `/v1`, `/v1beta`, `/mj`, `/suno`, `/kling`, realtime, or task routes.
- Implementing Claude, Gemini, Responses, image, audio, embedding, rerank, realtime, or video adaptors.
- Adding accounting, billing, user entitlements, token-level controls, or telemetry sinks beyond existing logs.
- Changing provider admin behavior beyond what the prerequisite selection change already requires.

## Decisions

### Decision: Introduce a minimal relay package inside `internal/app`

The first implementation can keep relay primitives near the HTTP server in `internal/app` to avoid premature package boundaries. Candidate types include route format, request envelope, selected provider metadata, attempt state, adaptor response, and response commit tracking.

Alternative considered: add a large `internal/relay` package immediately. That is likely useful later, but this change should first prove the pipeline around existing OpenAI-compatible behavior.

### Decision: Keep OpenAI adaptor mostly pass-through

The OpenAI-compatible adaptor will preserve current behavior: clone JSON body, remove client-only fields, map `model` using provider model mapping, build upstream request through existing upstream client behavior, and copy upstream response headers/body.

Alternative considered: port new-api request DTO parsing now. That would increase parity but also increases scope and risks changing accepted request shapes.

### Decision: Track response commit before retrying

The pipeline will only retry if no response status/body has been committed to the client. Streaming copy marks the response as committed after writing headers or the first body chunk.

Alternative considered: attempt stream retries on read errors. That can produce invalid mixed responses and is not safe without client-visible retry framing.

### Decision: Keep retry policy equivalent to current behavior

The first pipeline will retry network errors, `429`, and `5xx` responses up to `UPSTREAM_MAX_RETRIES`, excluding already-attempted providers. Retry policy can become configurable later.

Alternative considered: port new-api's richer retry settings now. That belongs in a later provider policy proposal after the pipeline is stable.

### Decision: Preserve existing OpenAI error envelope

Pipeline errors will continue using the existing OpenAI-style `{"error": ...}` response shape. The change may reorganize helpers, but it should not introduce Claude/Gemini-specific errors.

Alternative considered: add multi-format error normalization now. That is necessary later but not for the two existing OpenAI-compatible routes.

## Risks / Trade-offs

- [Risk] Refactoring a working handler can regress streaming behavior. -> Mitigation: add mock-provider streaming tests and assert incremental flush remains supported.
- [Risk] Generic relay types may overfit future work. -> Mitigation: keep types minimal and only model behavior needed by current endpoints.
- [Risk] Retry after partial response can corrupt output. -> Mitigation: gate retry on committed-response tracking and test committed streaming failures as final.
- [Risk] ModelId provider selection prerequisite may not be applied yet. -> Mitigation: document ordering and keep compile-time changes aligned with that selection API.

## Migration Plan

1. Add tests around the current OpenAI-compatible behavior before moving code.
2. Introduce minimal relay primitives and response commit tracking.
3. Move body parsing, model extraction, provider selection, retry loop, upstream dispatch, and response copy into the pipeline.
4. Keep `/v1/chat/completions` and `/v1/completions` registered at the same paths and methods.
5. Verify non-streaming, streaming, retry, no-provider, invalid JSON, missing model, oversized body, and auth behavior.

Rollback can restore the direct `openAIRoute` implementation because this change does not alter route paths or persistent schema.

## Open Questions

- Should the relay primitives stay in `internal/app` after this change, or move to `internal/relay` when the next route family is added?
