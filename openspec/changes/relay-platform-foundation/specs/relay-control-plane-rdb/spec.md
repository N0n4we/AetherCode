## ADDED Requirements

### Requirement: RDB Control Plane Source Of Truth
The relational database SHALL be the source of truth for account API key metadata, upstream provider/channel configuration, pricing configuration, and usage event records.

#### Scenario: Control-plane state is persisted
- **WHEN** account keys, provider channels, prices, or usage events are created or updated
- **THEN** the authoritative state is persisted in the relational database

#### Scenario: Services restart from RDB state
- **WHEN** relay or account service pods restart
- **THEN** they recover required control-plane state from the relational database without requiring in-memory state from a previous pod

### Requirement: Provider Channel Records
The control plane SHALL represent platform-managed upstream providers as channel records with routing, capability, authentication, and billing metadata.

#### Scenario: Channel contains routing metadata
- **WHEN** an upstream channel is stored
- **THEN** it includes provider identity, public modelId, endpoint capabilities, enabled state, priority, weight, upstream endpoint, and upstream model mapping metadata

#### Scenario: Channel contains billing metadata
- **WHEN** an upstream channel is stored
- **THEN** it can be associated with pricing or cost metadata used by dynamic billing

### Requirement: Single ModelId Per Platform Channel
Each platform-managed upstream channel MUST bind to exactly one public modelId.

#### Scenario: Channel has one modelId
- **WHEN** an operator creates or updates an upstream channel
- **THEN** the control plane accepts exactly one non-empty public modelId for that channel

#### Scenario: Multiple channels share one modelId
- **WHEN** multiple enabled channels are configured with the same public modelId and compatible endpoint capabilities
- **THEN** the relay can treat those channels as candidates for that modelId using priority and weight behavior

### Requirement: Versioned Relay Configuration Sync
The relay SHALL consume RDB-backed API key and provider/channel configuration through versioned cache synchronization.

#### Scenario: Provider channel update propagates
- **WHEN** a provider/channel record changes in the relational database
- **THEN** relay instances refresh their local provider/channel cache after observing the corresponding config version change

#### Scenario: API key update propagates
- **WHEN** API key state changes in the relational database
- **THEN** relay instances refresh their local key-validation state after observing the corresponding config version change or key-state sync signal

### Requirement: Secret Redaction
Control-plane APIs and logs MUST NOT expose raw client API keys or upstream provider secrets.

#### Scenario: Provider channel is listed
- **WHEN** an operator lists provider/channel records
- **THEN** the response omits or redacts upstream provider secret values

#### Scenario: Key metadata is logged
- **WHEN** account or relay services log API key-related events
- **THEN** logs include only safe key metadata and do not include raw API key secrets
