## 1. Route Registry

- [x] 1.1 Define route descriptor types for method, path pattern, route family, endpoint capability, response format, and implementation status.
- [x] 1.2 Add the initial curated route matrix for implemented, metadata, and unsupported routes.
- [x] 1.3 Refactor server route registration to use the route matrix where practical.
- [x] 1.4 Ensure unknown paths remain normal not-found responses.

## 2. Existing Implemented Routes

- [x] 2.1 Wire `POST /v1/chat/completions` through the OpenAI-compatible relay pipeline.
- [x] 2.2 Wire `POST /v1/completions` through the OpenAI-compatible relay pipeline.
- [x] 2.3 Ensure implemented routes use modelId plus endpoint capability provider selection.
- [x] 2.4 Preserve existing auth, request size, streaming, retry, and response metadata behavior.

## 3. Model Discovery Routes

- [x] 3.1 Add `GET /v1/models` backed by provider cache modelIds.
- [x] 3.2 Add `GET /v1/models/{model}` with model-not-found behavior.
- [x] 3.3 Add `GET /v1beta/models` and `GET /v1beta/openai/models` using provider cache metadata.
- [x] 3.4 Ensure model discovery responses omit groups and provider secrets.

## 4. Unsupported Route Shell

- [x] 4.1 Add a shared handler for registered but unsupported routes.
- [x] 4.2 Return HTTP `501` with stable error code `unsupported_endpoint`.
- [x] 4.3 Include route family or required capability in the error without exposing groups or secrets.
- [x] 4.4 Ensure unsupported routes do not select providers or dispatch upstream requests.
- [x] 4.5 Preserve method-not-allowed behavior for known paths with wrong methods.

## 5. Tests And Docs

- [x] 5.1 Add tests proving existing completion routes still use the relay pipeline.
- [x] 5.2 Add tests for OpenAI and Gemini model discovery responses.
- [x] 5.3 Add tests for unsupported registered routes returning structured `501` errors.
- [x] 5.4 Add tests for unknown paths returning normal not-found responses.
- [x] 5.5 Add tests that route shell responses do not expose group fields.
- [x] 5.6 Update `router/README.md` with the initial route compatibility matrix.
- [x] 5.7 Run `go test ./...` in `router/`.
