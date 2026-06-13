## 1. Data Model

- [ ] 1.1 Add endpoint capability metadata fields to provider storage and public provider responses.
- [ ] 1.2 Add constants and normalization helpers for endpoint capability identifiers.
- [ ] 1.3 Default legacy providers to OpenAI-compatible chat completions and text completions.

## 2. Cache And Selection

- [ ] 2.1 Extend cache indexing or filtering to support modelId plus endpoint capability lookup.
- [ ] 2.2 Preserve existing modelId selection behavior for completion routes.
- [ ] 2.3 Add aggregate capability counts to cache stats.

## 3. Admin Compatibility

- [ ] 3.1 Update provider create/update/list handling to accept and return optional capabilities.
- [ ] 3.2 Ensure existing provider payloads still create valid providers.
- [ ] 3.3 Ensure provider secrets remain omitted from public/admin-safe responses.

## 4. Tests And Docs

- [ ] 4.1 Add tests for default capabilities on legacy providers.
- [ ] 4.2 Add tests for explicit capability filtering and missing capability exclusion.
- [ ] 4.3 Add tests for provider admin backward compatibility.
- [ ] 4.4 Update `router/README.md` provider examples with endpoint capabilities.
- [ ] 4.5 Run `go test ./...` in `router/`.
