## Why

The relay needs a production platform boundary around the existing router: cloud resources, Kubernetes runtime, account-owned API keys, RDB-backed provider configuration, and usage billing are currently not specified as one coherent system. Defining this foundation now keeps deployment, account, provider, and billing decisions aligned with the existing modelId-oriented router contracts.

## What Changes

- Add Terraform-managed cloud infrastructure for a best-effort Kubernetes runtime, relational database, network access, and ingress or Gateway API entry points.
- Add Kubernetes deployment contracts for the relay service and an account service that issues API keys used by relay clients.
- Add RDB-backed control-plane ownership for relay API keys and upstream provider/channel records.
- Normalize upstream provider channels around a single public modelId per channel for platform-managed configuration, while preserving the router's public modelId selection behavior.
- Add dynamic usage billing for relay requests, including dynamic pricing when provider cache behavior changes the billable cost of a response.
- No breaking changes are intended for the existing OpenAI-compatible relay routes.

## Capabilities

### New Capabilities

- `cloud-relay-runtime`: Terraform and Kubernetes runtime contracts for the relay platform, including best-effort cloud Kubernetes, service deployment, and ingress or Gateway API exposure.
- `relay-account-api-keys`: Account service API key lifecycle and relay authentication behavior for client-facing API keys.
- `relay-control-plane-rdb`: Relational database ownership of relay API keys and upstream provider/channel configuration.
- `modelid-dynamic-billing`: Single-modelId upstream channel semantics and dynamic request billing, including cache-hit pricing.

### Modified Capabilities

- None.

## Impact

- Affected systems: Terraform cloud modules, Kubernetes manifests or charts, relay service deployment, account service deployment, relational database schema, provider configuration flow, API key lifecycle, and usage billing pipeline.
- Affected APIs: frontend-facing account APIs for API key management, relay authentication against issued API keys, provider/channel administration APIs, and internal usage/billing events.
- Dependencies: cloud Kubernetes provider, ingress controller or Gateway API implementation, RDB service, secret management, migration tooling, and observability for request usage and billing reconciliation.
