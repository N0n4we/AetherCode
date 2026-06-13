## Context

The first split moves provider selection to modelId, and the second split puts the current OpenAI-compatible endpoints behind a small relay pipeline. The next missing piece is provider capability metadata: the router needs to know whether a provider supports chat completions, text completions, embeddings, images, audio, responses, Claude messages, Gemini routes, realtime, or async tasks before future route shells can select it.

This change adds metadata only. It should not register new routes or implement new protocol adaptors.

## Goals / Non-Goals

**Goals:**

- Add explicit provider endpoint capability metadata.
- Default existing providers to the current OpenAI-compatible completion capabilities.
- Allow provider lookup to filter by modelId and endpoint capability.
- Keep provider admin CRUD backward-compatible.
- Make cache status expose capability counts without exposing secrets.

**Non-Goals:**

- Adding new public relay routes.
- Implementing Claude, Gemini, media, realtime, or task adaptors.
- Adding pricing/accounting or user entitlement behavior.
- Removing legacy fields such as `groups`.

## Decisions

### Decision: Store capabilities as structured string list metadata

Provider capabilities will be represented as a compact structured list of endpoint capability identifiers, such as `openai.chat_completions`, `openai.completions`, `openai.embeddings`, `openai.images`, `openai.audio`, `openai.responses`, `claude.messages`, `gemini.generate`, `realtime`, and `task.video`.

Alternative considered: infer capabilities from provider name and model list. That is convenient initially but breaks once one modelId can be served by providers with different endpoint support.

### Decision: Default existing providers to current completion support

When a provider has no capability metadata, it will be treated as supporting `openai.chat_completions` and `openai.completions`, matching current router behavior.

Alternative considered: require admins to update every provider. That would make this metadata change unnecessarily disruptive.

### Decision: Keep channel type optional

Channel type or provider family metadata can be stored for future adaptor dispatch, but it should not be required for the two existing OpenAI-compatible routes. Provider records with only the existing `provider` string remain valid.

Alternative considered: require a full new-api channel type immediately. That imports more taxonomy than this router needs at this step.

### Decision: Capability filtering belongs in cache/selection

Cache lookup should expose a method that filters candidates by modelId and endpoint capability. This keeps future route handlers from reimplementing provider filtering.

Alternative considered: filter after selection. That can waste retries on providers that could never serve the route.

## Risks / Trade-offs

- [Risk] Capability names drift as route work evolves. -> Mitigation: define constants and tests for the first supported capability set.
- [Risk] Admin payloads become noisy. -> Mitigation: keep fields optional and document defaults.
- [Risk] Existing providers are accidentally excluded. -> Mitigation: default missing capabilities to current completion support and test migration behavior.

## Migration Plan

1. Add capability metadata fields with safe defaults.
2. Add constants and parsing/normalization helpers for endpoint capability identifiers.
3. Extend provider public/admin representation while preserving old payloads.
4. Extend cache lookup and stats for modelId plus endpoint capability.
5. Add tests for default capabilities, explicit capabilities, capability filtering, and disabled providers.
6. Update README provider examples.

Rollback is safe because new metadata is optional and existing rows remain compatible.

## Open Questions

- Should capability identifiers mirror new-api route names exactly, or use router-specific stable names and map routes to them later?
