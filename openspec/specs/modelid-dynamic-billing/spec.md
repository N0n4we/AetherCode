## Purpose

Define relay usage-event and dynamic billing semantics for modelId-oriented provider channels.

## Requirements

### Requirement: Usage Events
The relay SHALL emit finalized usage events for authenticated relay requests.

#### Scenario: Successful request emits usage
- **WHEN** an authenticated relay request completes successfully
- **THEN** the system records a usage event with account identity, API key metadata, public modelId, endpoint capability, selected channel, request outcome, and available usage units

#### Scenario: Failed request emits auditable outcome
- **WHEN** an authenticated relay request fails after authentication
- **THEN** the system records an auditable request outcome according to the configured billing policy

### Requirement: Dynamic Price Resolution
The billing system SHALL resolve billable usage using dynamic pricing configuration effective for the account, public modelId, selected channel, usage class, and request completion time.

#### Scenario: Price changes over time
- **WHEN** a request completes after a new price configuration becomes effective
- **THEN** billing uses the price configuration effective at the request completion time

#### Scenario: Channel-specific price applies
- **WHEN** a selected channel has pricing that differs from the default modelId price
- **THEN** billing uses the applicable channel-specific pricing rule

### Requirement: Cache-Aware Billing
Cache hit state MUST be an input to dynamic billing and MUST NOT be treated as a hardcoded free or static-discount outcome.

#### Scenario: Cache hit is billed dynamically
- **WHEN** a relay response is classified as a cache hit
- **THEN** billing applies the dynamic price rule for the cache-hit usage class

#### Scenario: Cache miss is billed dynamically
- **WHEN** a relay response is classified as a cache miss
- **THEN** billing applies the dynamic price rule for the non-cache-hit usage class

### Requirement: Single ModelId Billing Attribution
Billing records SHALL use the public modelId requested by the client and the selected single-modelId provider channel as attribution dimensions.

#### Scenario: Multiple channels serve one modelId
- **WHEN** two upstream channels can serve the same public modelId and one channel is selected for a request
- **THEN** the usage event and billing calculation identify both the public modelId and the selected channel

#### Scenario: Upstream model mapping differs
- **WHEN** the selected channel maps the public modelId to a different upstream model name
- **THEN** billing attribution still records the public modelId and selected channel rather than exposing only the upstream model name

### Requirement: Idempotent Billing Records
Usage and billing writes MUST be idempotent for a relay request outcome.

#### Scenario: Usage write is retried
- **WHEN** the system retries recording usage for the same relay request outcome
- **THEN** it does not create duplicate billable charges for that outcome

#### Scenario: Billing can be reconciled
- **WHEN** operators reconcile usage events and billing records
- **THEN** each billable charge can be traced back to a stable usage event identifier
