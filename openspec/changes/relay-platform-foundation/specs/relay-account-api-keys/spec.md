## ADDED Requirements

### Requirement: Account API Key Lifecycle
The account service SHALL provide account-owned relay API key lifecycle operations for creating, listing, disabling, and revoking client API keys.

#### Scenario: API key is created
- **WHEN** an authenticated account creates a relay API key
- **THEN** the account service stores key metadata for that account and returns the raw API key secret only in the creation response

#### Scenario: API key is listed
- **WHEN** an authenticated account lists relay API keys
- **THEN** the account service returns key metadata without returning raw API key secrets

#### Scenario: API key is revoked
- **WHEN** an authenticated account revokes a relay API key
- **THEN** the key is no longer accepted for relay authentication

### Requirement: API Key Secret Storage
The system MUST NOT store raw client API key secrets after creation.

#### Scenario: Key is persisted
- **WHEN** a relay API key is created
- **THEN** persistent storage contains a verifier such as a hash plus metadata and does not contain the raw key secret

#### Scenario: Key lookup uses safe metadata
- **WHEN** the relay validates a presented key
- **THEN** it uses safe lookup metadata and verifier comparison without requiring raw key secrets in storage

### Requirement: Relay Authentication With Account Keys
The relay SHALL authenticate public relay requests using active account-issued API keys.

#### Scenario: Active key authorizes relay request
- **WHEN** a client sends a relay request with an active account-issued API key
- **THEN** the relay accepts the request and associates it with the owning account and key

#### Scenario: Missing key is rejected
- **WHEN** a client sends a relay request without a required API key
- **THEN** the relay rejects the request with an authentication error before selecting an upstream provider

#### Scenario: Revoked key is rejected
- **WHEN** a client sends a relay request with a revoked or disabled API key
- **THEN** the relay rejects the request with an authentication error before selecting an upstream provider

### Requirement: Key State Propagation
API key state changes SHALL become visible to relay authentication without requiring relay pod restarts.

#### Scenario: Revocation reaches relay
- **WHEN** an account revokes an API key
- **THEN** relay instances stop accepting that key after the configured key-state sync window

#### Scenario: Newly created key reaches relay
- **WHEN** an account creates an API key
- **THEN** relay instances can authenticate that key after the configured key-state sync window

### Requirement: Request Attribution
The relay MUST attach account and API key identity to authenticated requests for usage and billing events.

#### Scenario: Authenticated request is attributed
- **WHEN** the relay accepts a request authenticated by an account-issued API key
- **THEN** downstream usage and billing records can identify the owning account and key metadata
