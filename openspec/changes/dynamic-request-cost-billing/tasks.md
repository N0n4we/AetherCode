## 1. Pricing Schema

- [x] 1.1 Add per-token input, cached-input, and output rate fields to `PriceConfig`.
- [x] 1.2 Add cached-input and total token unit fields to `UsageEvent` with normalization.
- [x] 1.3 Add per-token rate and token-count breakdown fields to `BillableCharge`.

## 2. Charge Computation

- [x] 2.1 Implement `PriceConfig.AmountMicros` combining the base fee with input, cached-input, and output token charges.
- [x] 2.2 Default the cached-input rate to the input rate when unset.
- [x] 2.3 Update `CreateBillableChargeForEvent` to use the dynamic amount and persist the breakdown.
- [x] 2.4 Preserve flat per-request pricing when no token usage is reported.

## 3. Request-Cost Capture

- [x] 3.1 Add a bounded response tee that captures a JSON prefix and an SSE suffix.
- [x] 3.2 Parse OpenAI chat/completions and Responses API usage, including cached-token details.
- [x] 3.3 Tee upstream response bytes through the capture in the relay pipeline.
- [x] 3.4 Populate token units on the usage event from the captured usage.

## 4. Verification

- [x] 4.1 Add store tests for token-based charges and cached-rate fallback.
- [x] 4.2 Add usage-capture tests for JSON, streaming, Responses API, and no-usage cases.
- [x] 4.3 Confirm existing flat-price billing tests still pass (amount equals base fee times billable units).
