## ADDED Requirements

### Requirement: Request-Cost Based Charge Computation
The billing system SHALL compute the billable amount for a usage event from the actual request cost reported by the upstream response, rather than from a fixed per-request unit count. Pricing configuration SHALL support per-token rates for input, cached input, and output tokens in addition to an optional flat per-request base fee.

#### Scenario: Token usage drives the charge
- **WHEN** an upstream response reports input and output token counts for a request
- **THEN** the billable amount combines the configured per-request base fee with the per-token input and output rates applied to the reported token counts

#### Scenario: Cached input tokens are priced distinctly
- **WHEN** an upstream response reports cached input tokens and the applicable price defines a cached input rate
- **THEN** the cached input tokens are billed at the cached input rate and the remaining input tokens are billed at the input rate

#### Scenario: Cached rate defaults to the input rate
- **WHEN** the applicable price defines an input rate but no explicit cached input rate
- **THEN** cached input tokens are billed at the input rate rather than treated as free

#### Scenario: Usage without reported tokens falls back to the base fee
- **WHEN** an upstream response does not report usable token counts
- **THEN** the billable amount is the configured per-request base fee applied to the billable request units

### Requirement: Charge Traceability Breakdown
Billable charge records SHALL retain the token counts and per-token rates used to compute the amount so operators can reconcile the dynamic amount against the recorded request cost.

#### Scenario: Charge records the request-cost breakdown
- **WHEN** a billable charge is created for a usage event
- **THEN** the charge stores the input, cached input, and output token counts together with the applied per-token rates and the resulting amount
