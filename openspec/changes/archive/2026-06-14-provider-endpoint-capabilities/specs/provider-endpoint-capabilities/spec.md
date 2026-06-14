## ADDED Requirements

### Requirement: Provider Capability Metadata
Provider records SHALL support optional endpoint capability metadata describing which relay endpoint families the provider can serve.

#### Scenario: Existing provider has default capabilities
- **WHEN** a provider record has no endpoint capability metadata
- **THEN** the router treats it as supporting OpenAI-compatible chat completions and text completions

#### Scenario: Provider declares explicit capabilities
- **WHEN** a provider declares endpoint capabilities
- **THEN** the router stores and returns those capabilities through provider admin APIs

### Requirement: Capability-Aware Selection
Provider lookup MUST filter eligible providers by modelId and requested endpoint capability.

#### Scenario: Matching capability is eligible
- **WHEN** a provider lists modelId `gpt-4o` and capability `openai.chat_completions`
- **THEN** the provider is eligible for a chat completions request for `gpt-4o`

#### Scenario: Missing capability is excluded
- **WHEN** a provider lists modelId `gpt-4o` but does not support capability `openai.embeddings`
- **THEN** the provider is not eligible for an embeddings request for `gpt-4o`

### Requirement: Backward-Compatible Admin API
Provider admin create, update, and list APIs MUST remain compatible with existing provider payloads while exposing optional capability metadata.

#### Scenario: Legacy provider create succeeds
- **WHEN** an admin creates a provider using the existing payload fields
- **THEN** the provider is created successfully with default completion capabilities

#### Scenario: Provider list includes capabilities
- **WHEN** an admin lists providers
- **THEN** each provider response includes endpoint capability metadata without exposing provider secrets

### Requirement: Capability Cache Stats
Router status and readiness metadata SHALL expose aggregate capability availability without exposing provider secrets.

#### Scenario: Internal status includes capability counts
- **WHEN** an authorized operator calls `/internal/status`
- **THEN** the response includes aggregate counts for configured endpoint capabilities
