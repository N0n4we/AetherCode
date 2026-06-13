# Project Agent Instructions

## Version Control

- Use `jj` for version-control operations by default.
- Do not use `git` unless the user explicitly asks for it or `jj` cannot perform the required operation.
- If `git` is necessary as a fallback, state why before using it.

## OpenSpec Change Order

Implement the router/new-api alignment changes in this order:

1. `modelid-provider-selection`
2. `relay-pipeline-openai-compatible`
3. `provider-endpoint-capabilities`
4. `relay-route-compatibility-shell`

Do not implement a later change before the earlier changes it depends on are complete. The proposals may mention these dependencies in prose, but this file is the canonical order for agents working through the active OpenSpec changes.
