## ADDED Requirements

### Requirement: Route Compatibility Registry
The router SHALL maintain an explicit registry for recognized relay routes including method, path pattern, route family, endpoint capability, response format, and implementation status.

#### Scenario: Existing chat completions route remains implemented
- **WHEN** a client sends `POST /v1/chat/completions`
- **THEN** the request is handled by the OpenAI-compatible relay pipeline

#### Scenario: Existing text completions route remains implemented
- **WHEN** a client sends `POST /v1/completions`
- **THEN** the request is handled by the OpenAI-compatible relay pipeline

#### Scenario: Registered unsupported route is recognized
- **WHEN** a client calls a registered route whose implementation status is unsupported
- **THEN** the router returns a structured unsupported-endpoint response instead of a generic not-found response

#### Scenario: Unknown path remains not found
- **WHEN** a client calls an unregistered path
- **THEN** the router returns the normal not-found behavior

### Requirement: Cache-Backed Model Discovery
The router SHALL expose model discovery endpoints backed by configured provider cache metadata and modelId mappings.

#### Scenario: OpenAI model list returns configured modelIds
- **WHEN** a client sends `GET /v1/models`
- **THEN** the router returns an OpenAI-compatible model list containing configured modelIds from enabled providers
- **AND** the response does not expose groups or provider secrets

#### Scenario: OpenAI model lookup finds a configured modelId
- **WHEN** a client sends `GET /v1/models/{model}` for a configured modelId
- **THEN** the router returns an OpenAI-compatible model object for that modelId

#### Scenario: OpenAI model lookup misses
- **WHEN** a client sends `GET /v1/models/{model}` for an unknown modelId
- **THEN** the router returns a structured model-not-found error

#### Scenario: Gemini model routes are available
- **WHEN** a client sends `GET /v1beta/models` or `GET /v1beta/openai/models`
- **THEN** the router returns model metadata derived from the same provider cache without exposing groups

### Requirement: Unsupported Endpoint Errors
Registered but unimplemented routes MUST return a stable JSON error response.

#### Scenario: Unsupported route response is stable
- **WHEN** a client calls a registered unsupported route
- **THEN** the router responds with HTTP `501`
- **AND** the response includes a stable error code `unsupported_endpoint`
- **AND** the response identifies the route family or required capability without exposing provider secrets or groups

#### Scenario: Unsupported route is not dispatched upstream
- **WHEN** a client calls a registered unsupported route
- **THEN** the router does not select a provider
- **AND** the router does not send an upstream request

### Requirement: Capability Mapping For Routes
Each registered route MUST map to an endpoint capability identifier from provider endpoint capability metadata.

#### Scenario: Route descriptor includes capability
- **WHEN** the router registers a route descriptor
- **THEN** the descriptor includes the endpoint capability needed to serve that route

#### Scenario: Implemented routes use capability-aware selection
- **WHEN** an implemented relay route selects a provider
- **THEN** provider candidates are filtered by modelId and the route's required endpoint capability

### Requirement: User-Facing Group Hiding
The route shell MUST NOT expose group concepts in request contracts, response bodies, or user-facing errors.

#### Scenario: Model route hides group
- **WHEN** a client calls any model discovery endpoint
- **THEN** the response contains modelId-oriented metadata and no group fields

#### Scenario: Error route hides group
- **WHEN** a client receives any route shell error
- **THEN** the error payload does not include group names or group selection hints
