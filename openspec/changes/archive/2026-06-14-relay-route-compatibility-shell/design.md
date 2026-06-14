## Context

The router currently exposes only two OpenAI-compatible completion endpoints. Earlier split changes make selection modelId-centric, isolate the existing OpenAI relay pipeline, and add provider endpoint capabilities. The remaining compatibility gap is route shape: new-api exposes many endpoint families, but implementing all adaptors, accounting hooks, task state, and realtime behavior in one step would make the change too broad.

This change adds a route compatibility shell. It recognizes a curated route matrix, keeps implemented completion routes working, exposes model discovery from provider metadata, and returns explicit unsupported errors for recognized routes that are not implemented yet.

## Goals / Non-Goals

**Goals:**

- Define a central route registry for recognized relay endpoint families.
- Preserve the existing implemented OpenAI-compatible completion routes.
- Add cache-backed model discovery routes.
- Return stable structured errors for registered but unimplemented routes.
- Map each registered route to an endpoint capability identifier.
- Keep groups hidden from all user-facing route behavior.

**Non-Goals:**

- Implementing Claude, Gemini, embeddings, images, audio, rerank, realtime, or async task adaptors.
- Implementing billing, quota, token accounting, or user entitlement behavior.
- Adding task persistence or webhook/callback behavior.
- Matching every new-api path in one step.
- Changing provider selection semantics beyond using the previous capability-aware lookup.

## Decisions

### Decision: Use an explicit route registry

Routes will be described by method, path pattern, route family, endpoint capability, response format, and implementation status. Implemented completion routes delegate to the OpenAI-compatible relay pipeline. Metadata routes read provider cache state. Unsupported routes use a shared unsupported handler.

Alternative considered: add handlers ad hoc as each route appears. That makes it hard to answer which new-api routes are recognized and whether a 404 means unknown path or not-yet-implemented path.

### Decision: Start with a curated core matrix

The first route matrix should include:

- Implemented: `POST /v1/chat/completions`, `POST /v1/completions`.
- Metadata: `GET /v1/models`, `GET /v1/models/{model}`, `GET /v1beta/models`, `GET /v1beta/openai/models`.
- Unsupported shell: `POST /v1/responses`, `POST /v1/responses/compact`, `POST /v1/embeddings`, `POST /v1/images/generations`, `POST /v1/images/edits`, `POST /v1/audio/transcriptions`, `POST /v1/audio/translations`, `POST /v1/audio/speech`, `POST /v1/rerank`, `POST /v1/messages`, `POST /v1beta/models/{model}:generateContent`, `POST /v1beta/models/{model}:streamGenerateContent`, `GET /v1/realtime`, and core video/task submission/status routes.

Alternative considered: register every route from new-api immediately. That risks inaccurate compatibility claims before the router has matching adaptors and state management.

### Decision: Registered unsupported routes return JSON 501

A registered route with no implementation will return `501 Not Implemented` and a structured JSON error with a stable code such as `unsupported_endpoint`. The error may include method, path pattern, and required capability, but it must not include provider secrets, internal group names, or hidden routing metadata.

Alternative considered: return 404 until implementation exists. That makes it impossible for clients and tests to distinguish unknown paths from recognized compatibility gaps.

### Decision: Model routes are backed by provider cache

Model listing should use provider cache metadata after modelId selection and endpoint capabilities exist. `/v1/models` and `/v1/models/{model}` use an OpenAI-compatible shape. Gemini-flavored model routes may use a minimal Gemini-compatible shape, but they still derive model names from the same provider cache and must not expose group.

Alternative considered: return a static route matrix. That would drift from actual configured providers.

### Decision: Unknown paths stay unknown

The shell should not add a broad catch-all that turns every path into an unsupported endpoint. Only registered route patterns get structured unsupported errors; truly unknown paths remain normal not-found responses.

Alternative considered: catch all `/v1/*` and `/v1beta/*`. That can mask typos and make debugging harder.

## Risks / Trade-offs

- [Risk] Route matrix becomes inaccurate as new-api evolves. -> Mitigation: document the initial curated matrix and require future route changes to update it.
- [Risk] Clients assume unsupported shell means full compatibility. -> Mitigation: use explicit `unsupported_endpoint` errors and README route status.
- [Risk] Model discovery shape differs from some provider protocols. -> Mitigation: keep OpenAI and Gemini model routes separate and test their payload shapes.
- [Risk] Route registration conflicts with existing handlers. -> Mitigation: test existing completion routes and method-not-allowed behavior.

## Migration Plan

1. Add route descriptor types and the initial route matrix.
2. Refactor server route registration to use descriptors where practical.
3. Attach existing completion routes to the implemented OpenAI-compatible pipeline.
4. Add cache-backed model discovery handlers.
5. Add unsupported-route handler and error envelope.
6. Add tests for implemented, metadata, unsupported, method mismatch, and unknown routes.
7. Update README route matrix.

Rollback is straightforward because new shell routes can be removed without changing provider storage or the existing completion pipeline.

## Open Questions

- Which video/task endpoints should be included in the first curated shell versus left for a dedicated task proposal?
- Should Gemini model discovery return only Gemini-capable models or all configured modelIds with provider capability annotations?
