## 1. Data Model

- [x] 1.1 Add endpoint capability metadata fields to provider storage and public provider responses.
- [x] 1.2 Add constants and normalization helpers for endpoint capability identifiers.
- [x] 1.3 Default legacy providers to OpenAI-compatible chat completions and text completions.

## 2. Cache And Selection

- [x] 2.1 Extend cache indexing or filtering to support modelId plus endpoint capability lookup.
- [x] 2.2 Preserve existing modelId selection behavior for completion routes.
- [x] 2.3 Add aggregate capability counts to cache stats.

## 3. Admin Compatibility

- [x] 3.1 Update provider create/update/list handling to accept and return optional capabilities.
- [x] 3.2 Ensure existing provider payloads still create valid providers.
- [x] 3.3 Ensure provider secrets remain omitted from public/admin-safe responses.

## 4. Tests And Docs

- [x] 4.1 Add tests for default capabilities on legacy providers.
- [x] 4.2 Add tests for explicit capability filtering and missing capability exclusion.
- [x] 4.3 Add tests for provider admin backward compatibility.
- [x] 4.4 Update `router/README.md` provider examples with endpoint capabilities.
- [x] 4.5 Run `go test ./...` in `router/`.
