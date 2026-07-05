## Why

The relay already resolves a dynamic price per account, public modelId, selected
channel, usage class, cache state, and effective time. However, the billable
amount is computed as a fixed one billable unit multiplied by a single
`unit_price_micros`. The actual request cost reported by the upstream response
(input, cached input, and output token counts) is never captured or priced, so
every request for a given price row costs the same regardless of how large the
prompt or completion is.

The requirement is that dynamic pricing reflects the real request cost across
all channels: each channel that serves a modelId can define its own per-token
rates, and the charge for a request must be computed from the token usage that
channel's upstream actually returned.

## What Changes

- Capture the upstream token usage (input, cached input, output, and total
  tokens) from OpenAI-compatible chat/completions and Responses API responses,
  for both non-streaming JSON and streaming SSE responses, without buffering
  unbounded response bodies.
- Extend price configuration with per-token input, cached-input, and output
  rates in addition to the existing flat per-request base fee.
- Compute the billable amount dynamically from the captured request cost and the
  applicable channel/account/model price, so charges scale with real token
  usage while remaining backward compatible with flat per-request pricing.
- Bill cached input tokens at a distinct cached rate, defaulting to the input
  rate when no cached rate is configured so cached tokens are never silently
  free.
- Record the token-count and per-token-rate breakdown on each billable charge
  for reconciliation.
- No breaking changes to the OpenAI-compatible relay routes or to existing
  flat-priced configurations.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `modelid-dynamic-billing`: Add request-cost (token-based) charge computation,
  per-token pricing rates, and charge traceability breakdown on top of the
  existing dynamic price resolution and cache-aware billing.

## Impact

- Affected systems: relay usage/billing pipeline, price configuration schema,
  usage event and billable charge tables.
- Affected APIs: internal usage/billing records (new pricing rate fields and
  usage breakdown columns); relay client APIs are unchanged.
- Dependencies: relational database schema migration for the new price rate
  columns and usage/charge token-count columns (additive, defaulted to zero).
