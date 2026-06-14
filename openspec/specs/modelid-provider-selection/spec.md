## Purpose

Define modelId-based provider selection semantics for public relay requests.

## Requirements

### Requirement: ModelId Provider Index
The router SHALL index enabled providers by public modelId and endpoint compatibility, allowing multiple providers to serve the same modelId.

#### Scenario: Existing provider remains eligible
- **WHEN** a provider row has `models` containing `gpt-4o` and no new modelId-specific fields
- **THEN** the provider is eligible for requests whose model is `gpt-4o`

#### Scenario: Multiple providers serve one modelId
- **WHEN** three enabled providers list modelId `gpt-4o`
- **THEN** provider selection treats all three as candidates before applying priority, weight, and retry exclusions

### Requirement: Hidden Group Semantics
The router MUST NOT use client-supplied group, pool, or routing labels as provider selection inputs for public relay requests.

#### Scenario: Body group hint is ignored
- **WHEN** a public OpenAI-compatible request includes body field `group`
- **THEN** provider selection ignores that field and the upstream request body does not include it

#### Scenario: Header group hint is ignored
- **WHEN** a public OpenAI-compatible request includes `X-Aether-Group` or `X-Router-Group`
- **THEN** provider selection ignores the header value and does not forward it as an upstream routing hint

### Requirement: Provider Status And Priority
The router MUST exclude disabled providers and choose among eligible providers using descending priority before applying weight-based selection among providers at the highest eligible priority.

#### Scenario: Disabled provider is excluded
- **WHEN** a disabled provider lists the requested modelId
- **THEN** provider selection does not return that provider

#### Scenario: Higher priority provider wins
- **WHEN** two enabled providers list the same modelId and have different priorities
- **THEN** selection only considers the provider or providers with the highest priority for the current attempt

#### Scenario: Same priority providers use weights
- **WHEN** multiple enabled providers list the same modelId and share the highest priority
- **THEN** selection chooses among those providers according to configured weights

### Requirement: Wildcard And No-Provider Handling
The router SHALL support wildcard model providers and return a clear no-provider error when no exact or wildcard provider is available.

#### Scenario: Wildcard provider is used
- **WHEN** no enabled provider lists the exact requested modelId and an enabled provider lists `*`
- **THEN** provider selection may return the wildcard provider

#### Scenario: No provider exists
- **WHEN** no enabled provider lists the requested modelId and no enabled wildcard provider exists
- **THEN** provider selection returns a no-provider error naming the requested modelId

### Requirement: Retry Exclusion
The provider selector MUST exclude providers already attempted by the current request when retry exclusion is supplied.

#### Scenario: Failed provider is excluded
- **WHEN** provider `1` already failed for the current request
- **THEN** the next provider selection for the same modelId does not return provider `1`

#### Scenario: No remaining provider
- **WHEN** every eligible provider for a modelId is excluded by retry state
- **THEN** provider selection returns a no-remaining-provider error
