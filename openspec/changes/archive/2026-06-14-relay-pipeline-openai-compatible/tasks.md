## 1. Baseline Tests

- [x] 1.1 Add/extend tests for successful `/v1/chat/completions` and `/v1/completions` non-streaming proxy behavior.
- [x] 1.2 Add/extend tests for streaming response forwarding and flushing.
- [x] 1.3 Add tests for invalid method, invalid JSON, missing/empty model, oversized body, and unauthorized public API key.

## 2. Relay Pipeline Primitives

- [x] 2.1 Add minimal relay types for format, request envelope, selected provider metadata, attempt state, and adaptor result.
- [x] 2.2 Add response commit tracking for status/body writes.
- [x] 2.3 Add replayable request body handling for retry attempts.
- [x] 2.4 Keep pipeline types scoped to OpenAI-compatible routes and avoid registering new route families.

## 3. OpenAI-Compatible Adaptor

- [x] 3.1 Move model extraction and upstream request body construction into an OpenAI-compatible adaptor path.
- [x] 3.2 Preserve provider model mapping, request field passthrough, custom auth headers, extra provider headers, and hop-by-hop header filtering.
- [x] 3.3 Preserve `X-Aether-Router-Instance`, `X-Aether-Provider-ID`, `X-Aether-Provider-Name`, and `X-Aether-Provider-Version` response headers.

## 4. Retry Behavior

- [x] 4.1 Implement retry orchestration for network errors, `429`, and `5xx` before response commit.
- [x] 4.2 Exclude already-attempted providers from retry selection.
- [x] 4.3 Add tests for retryable status, network error, non-retryable `4xx`, exhausted providers, and committed streaming failure.

## 5. Verification And Docs

- [x] 5.1 Update `router/README.md` to describe the OpenAI-compatible relay pipeline and retry behavior.
- [x] 5.2 Run `go test ./...` in `router/`.
