## 1. Cache And Selection

- [ ] 1.1 Replace the cache index with modelId-to-provider lookup while retaining wildcard model support.
- [ ] 1.2 Change provider selection API to accept modelId and excluded provider IDs without a public group argument.
- [ ] 1.3 Preserve priority ordering and weight-based selection among providers at the highest eligible priority.
- [ ] 1.4 Preserve disabled-provider exclusion and no-provider/no-remaining-provider errors.

## 2. Public Request Behavior

- [ ] 2.1 Update OpenAI-compatible request parsing so body `group` is ignored for selection and removed before upstream dispatch.
- [ ] 2.2 Ignore `X-Aether-Group` and `X-Router-Group` headers for public provider selection.
- [ ] 2.3 Keep provider admin CRUD backward-compatible with existing `models` and `groups` fields.

## 3. Tests

- [ ] 3.1 Add cache tests for exact modelId match and multiple providers serving one modelId.
- [ ] 3.2 Add cache tests for wildcard model fallback and no-provider errors.
- [ ] 3.3 Add cache tests for disabled providers, priority ordering, weighted selection, and retry exclusion.
- [ ] 3.4 Add handler tests proving body/header group hints do not affect provider choice and body `group` is not forwarded upstream.

## 4. Documentation And Verification

- [ ] 4.1 Update `router/README.md` to describe modelId-based provider routing and legacy/internal group metadata.
- [ ] 4.2 Run `go test ./...` in `router/`.
