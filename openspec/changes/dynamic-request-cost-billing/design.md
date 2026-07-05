## Context

`PriceConfig` resolution already selects the most specific price for an account,
public modelId, provider channel, usage class, and cache state effective at the
request completion time (`ResolvePrice` + `PriceConfig.matches`). The gap is the
charge computation: `CreateBillableChargeForEvent` multiplied a single
`unit_price_micros` by a hardcoded `BillableUnits` of 1. `UsageEvent` already
declared `InputUnits`/`OutputUnits` fields, but nothing populated them because
the relay never inspected the upstream response body.

## Goals

- Charge = real request cost, resolved per channel/account/model.
- Preserve existing flat per-request pricing and existing billing tests.
- Bounded memory when inspecting responses (including large streaming bodies).
- Idempotent, reconcilable charges (unchanged contract).

## Decisions

### Token capture without unbounded buffering

The relay streams upstream responses straight to the client. To read usage we
tee the response bytes through a `usageCapture` sink while copying:

- Non-streaming JSON: retain a bounded prefix (1 MiB) and parse the top-level
  `usage` object. If the prefix limit is exceeded the body is treated as having
  no parseable usage and billing falls back to the base fee.
- Streaming SSE: retain a bounded suffix (64 KiB) and scan `data:` frames for
  the last frame carrying a `usage` object (as emitted with
  `stream_options.include_usage`). A partial leading frame in the retained tail
  simply fails to parse and is skipped.

The sink never returns an error, so usage capture can never interfere with
delivering the response to the client.

Both OpenAI chat/completions style usage (`prompt_tokens`, `completion_tokens`,
`prompt_tokens_details.cached_tokens`) and Responses API style usage
(`input_tokens`, `output_tokens`, `input_tokens_details.cached_tokens`) are
supported.

### Charge formula

```
amount = base_fee * billable_units
       + input_rate  * (input_units - cached_units)
       + cached_rate * cached_units
       + output_rate * output_units
```

- `cached_rate` defaults to `input_rate` when unset, so cached tokens are never
  silently free.
- `cached_units` is clamped to `input_units`.
- When no token usage is reported, all token terms are zero and the amount is
  `base_fee * billable_units`, matching prior behavior.

### Per-channel pricing

"All channels" is satisfied by the existing per-channel price resolution: each
channel serving a modelId can have its own `PriceConfig` row (matched by
`ProviderChannelID`), which now carries per-token rates. The request is billed
using the price for the channel that was actually selected, so the effective
price is derived from the real cost of that channel rather than a single global
rate.

## Risks / Trade-offs

- Providers that do not return a `usage` object (or that stream without
  `include_usage`) yield no token cost; those requests fall back to the base
  fee. This is acceptable and matches prior behavior.
- The tail window for SSE must be large enough to contain the final usage frame;
  64 KiB comfortably covers usage frames.

## Migration

The new columns (`input_unit_price_micros`, `output_unit_price_micros`,
`cached_input_unit_price_micros` on price configs; `cached_input_units`,
`total_units` on usage events; rate and unit breakdown on billable charges) are
additive and default to zero, so `AutoMigrate` adds them without touching
existing rows or amounts.
