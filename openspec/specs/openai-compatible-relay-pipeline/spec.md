## Purpose

Define the relay pipeline behavior for existing OpenAI-compatible completion routes.

## Requirements

### Requirement: Existing OpenAI Routes Use Pipeline
The router SHALL handle `POST /v1/chat/completions` and `POST /v1/completions` through a relay pipeline while preserving their existing route paths, methods, and public API key behavior.

#### Scenario: Chat completions route succeeds
- **WHEN** a client sends a valid `POST /v1/chat/completions` request with an authorized router API key
- **THEN** the router selects a provider, forwards the request upstream, and returns the upstream response

#### Scenario: Text completions route succeeds
- **WHEN** a client sends a valid `POST /v1/completions` request with an authorized router API key
- **THEN** the router selects a provider, forwards the request upstream, and returns the upstream response

### Requirement: Request Validation
The pipeline MUST enforce method, auth, body size, valid JSON, and non-empty string model validation before selecting a provider.

#### Scenario: Invalid method is rejected
- **WHEN** a client calls an OpenAI-compatible completion route with a non-POST method
- **THEN** the router returns method-not-allowed without selecting a provider

#### Scenario: Invalid JSON is rejected
- **WHEN** a client sends malformed JSON to an OpenAI-compatible completion route
- **THEN** the router returns a `400` OpenAI-compatible error without selecting a provider

#### Scenario: Missing model is rejected
- **WHEN** a client sends a valid JSON body without a non-empty string `model`
- **THEN** the router returns a `400` OpenAI-compatible error without selecting a provider

#### Scenario: Oversized body is rejected
- **WHEN** a client sends a body larger than the configured maximum
- **THEN** the router returns `413` without selecting a provider

### Requirement: Upstream Request Preservation
The OpenAI-compatible adaptor SHALL preserve request fields, remove client-only routing hints, rewrite `model` using provider model mapping, apply provider auth settings, and send provider extra headers.

#### Scenario: Model is mapped upstream
- **WHEN** a provider maps requested model `gpt-4o` to upstream model `deployment-a`
- **THEN** the upstream request body contains `model` set to `deployment-a`

#### Scenario: Non-model fields are preserved
- **WHEN** a request contains valid OpenAI-compatible fields other than `model`
- **THEN** the upstream request body contains those fields unchanged unless they are client-only routing hints

#### Scenario: Provider auth headers are applied
- **WHEN** a provider defines custom auth header and prefix settings
- **THEN** the upstream request uses those settings when calling the provider

### Requirement: Retry And Body Replay
The pipeline MUST retry retryable failures before response commit by excluding previously attempted providers and replaying the original upstream request body.

#### Scenario: Retryable status retries another provider
- **WHEN** the first provider returns `429` or `5xx` before response commit and another provider is available
- **THEN** the router retries the request with another provider using the original request body

#### Scenario: Network error retries another provider
- **WHEN** the upstream client returns a network error before response commit and another provider is available
- **THEN** the router retries the request with another provider

#### Scenario: Non-retryable status does not retry
- **WHEN** a provider returns a non-retryable `4xx` status
- **THEN** the router returns that provider response without trying another provider

#### Scenario: No remaining provider returns upstream error
- **WHEN** all eligible providers have failed with retryable errors
- **THEN** the router returns an OpenAI-compatible upstream error

### Requirement: Streaming Commit Safety
The pipeline MUST stream upstream responses incrementally and MUST NOT retry once response headers or body have been committed to the client.

#### Scenario: Streaming response is flushed
- **WHEN** an upstream provider returns a streaming response
- **THEN** the router forwards chunks incrementally and flushes them to the client

#### Scenario: Committed stream failure is final
- **WHEN** a streaming response fails after the router has committed headers or body to the client
- **THEN** the router does not retry the request on another provider

### Requirement: Response Metadata
Successful OpenAI-compatible relay responses SHALL include router/provider metadata headers compatible with current behavior.

#### Scenario: Successful response has provider headers
- **WHEN** an OpenAI-compatible relay request succeeds
- **THEN** the response includes `X-Aether-Router-Instance`, `X-Aether-Provider-ID`, `X-Aether-Provider-Name`, and `X-Aether-Provider-Version`
