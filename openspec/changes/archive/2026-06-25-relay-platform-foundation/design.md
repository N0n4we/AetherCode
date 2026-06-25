## Context

AetherCode currently has a Go router that serves OpenAI-compatible relay routes, stores provider configuration in SQL, polls a provider config version, and builds an in-memory modelId-to-provider cache. Existing specs already define public modelId selection, endpoint capabilities, route compatibility, and OpenAI-compatible relay behavior.

This change defines the platform around that router:

```
                 frontend / clients
                         |
                 Ingress or Gateway API
                  /                    \
                 v                      v
        account service             relay service
        key lifecycle                OpenAI-compatible proxy
                 \                      /
                  v                    v
                         RDB
        accounts, api keys, provider channels, prices, usage
                         |
                         v
                  upstream providers
```

The platform is expected to run on cloud Kubernetes as a best-effort cluster. Terraform owns cloud resources. Kubernetes owns workload rollout. The relational database is the control-plane source of truth for API keys, upstream provider/channel configuration, and billing data.

## Goals / Non-Goals

**Goals:**

- Define a Terraform-managed cloud and Kubernetes runtime for relay and account services.
- Define frontend-facing ingress or Gateway API routing for account and relay APIs.
- Move client API key issuance to the account service and validate those keys in the relay.
- Treat RDB as the source of truth for API keys, upstream provider channels, prices, and usage events.
- Normalize platform-managed upstream channels to one public modelId per channel while keeping multiple channels eligible for the same modelId.
- Support dynamic billing, including cache-hit-aware pricing.

**Non-Goals:**

- No multi-region or strict high-availability SLO is required for this best-effort cluster.
- No payment processor integration, invoice rendering, or customer billing UI is included.
- No breaking change is intended for existing OpenAI-compatible relay request paths.
- No provider-specific cache implementation is selected here; this design only requires cache state to be captured when it affects billing.

## Decisions

### Use Terraform for cloud primitives and Kubernetes for service rollout

Terraform will manage the cluster, database, network, load-balancer prerequisites, DNS/TLS integration hooks, and service account or secret-manager bindings. Kubernetes manifests or a chart will manage relay and account deployments, services, probes, autoscaling knobs, config references, and ingress or Gateway API resources.

Alternative considered: manage everything with Kubernetes operators. That would reduce Terraform surface area, but cloud network, database, and cluster lifecycle are cleaner and more auditable through Terraform modules.

### Keep the runtime explicitly best-effort

The initial cloud cluster should prefer low operational cost and simple recovery over strict availability guarantees. Services still need readiness checks, liveness checks, resource requests/limits, rollout health, and clear degradation behavior, but the design does not promise multi-zone quorum or zero-downtime database failover.

Alternative considered: design for production HA from the first change. That adds substantial cloud and migration complexity before the account, provider, and billing contracts are stable.

### Make RDB the control-plane source of truth

The account service writes API key metadata and key state to RDB. Provider/channel administration writes upstream channel metadata to RDB. The relay reads from RDB through a versioned cache, following the router's existing provider config sync pattern.

Alternative considered: issue keys and provider updates through relay-only admin APIs. That is simpler for a single binary, but it couples account workflows to relay internals and makes billing ownership unclear.

### Store API keys hashed, and show secrets only once

The account service generates client API keys, stores only a hash plus metadata, and returns the raw key only at creation time. Relay authentication uses the presented key to resolve an active account/key record and attaches that identity to the request context and usage events.

Alternative considered: keep a shared static `ROUTER_API_KEY`. That remains useful for local development, but it cannot support per-account revocation, usage attribution, or billing.

### Represent platform provider channels as single-modelId records

Platform-managed upstream channels will bind to exactly one public modelId, plus endpoint capabilities, priority, weight, upstream model mapping, and secret references. Multiple channels can share the same public modelId. During implementation, this can either project into the router's existing provider rows or migrate the router schema, but the platform contract is one public modelId per channel.

Alternative considered: keep multi-model provider rows as the control-plane primitive. That matches the current router DB shape, but it makes per-model pricing, cache policy, enablement, and billing attribution harder to reason about.

### Emit immutable usage events, then derive billable charges

The relay should emit one finalized usage event per authenticated request outcome with account/key identity, public modelId, selected channel, endpoint capability, token or unit counts, cache state, upstream status, and timing metadata. Billing resolution applies the effective price table at request completion and records the computed charge or billable units.

Alternative considered: compute balances inline without a usage ledger. Inline charging is lower latency, but it is harder to audit, retry, and reconcile when upstream usage fields or cache-hit classification arrive late.

### Treat cache hits as priced usage classes

Cache hit state is not a boolean discount hardcoded in the relay. It is an input to dynamic pricing. A cache-hit response can be billed differently by modelId, channel, account, and effective price window.

Alternative considered: make cache hits free or static-discounted. That is simple, but it conflicts with dynamic pricing and prevents provider-specific cache economics from being reflected accurately.

## Risks / Trade-offs

- RDB becomes a shared dependency for authentication, provider config, and billing -> mitigate with local relay caches, readiness checks, migrations, backups, and clear fail-closed behavior for auth.
- Best-effort Kubernetes can have visible downtime during node or database events -> mitigate with documented SLO expectations, probes, rollout strategy, and later HA upgrade path.
- Single-modelId channel records may require migration from current multi-model provider rows -> mitigate with a projection layer or compatibility migration that preserves existing router behavior during rollout.
- Dynamic billing can diverge from upstream usage if events are partial or duplicated -> mitigate with idempotent event IDs, request finalization rules, and reconciliation jobs.
- API key hashes need fast lookup without storing raw keys -> mitigate with keyed hashes or key prefixes for lookup plus constant-time hash verification.
- Provider secrets may leak through Terraform state, Kubernetes manifests, or logs -> mitigate with secret-manager references, sensitive outputs, redacted logs, and no raw secret exposure in admin responses.

## Migration Plan

1. Add Terraform modules for cloud network, best-effort Kubernetes, RDB, and ingress or Gateway API prerequisites.
2. Add Kubernetes deployment configuration for relay and account services using secret references and health probes.
3. Add RDB migrations for accounts, API keys, provider channels, price tables, usage events, and config versions.
4. Implement account API key lifecycle and relay key validation in compatibility mode alongside `ROUTER_API_KEY`.
5. Add provider channel administration and a projection or migration path from provider rows to single-modelId channels.
6. Emit usage events from relay request finalization, then add billing price resolution for modelId/channel/cache state.
7. Roll out by enabling account-issued keys and provider channels for a small model set before retiring static local-only configuration.

Rollback should preserve existing relay routes and allow operators to fall back to static `ROUTER_API_KEY` and existing provider rows while account, channel, or billing features are disabled.

## Open Questions

- First Terraform target: Google Cloud GKE Autopilot for Kubernetes and Cloud SQL for PostgreSQL as the managed RDB. This keeps the initial cluster best-effort and low-operations while using a PostgreSQL backend already supported by the router.
- Should the account service be implemented in Go to share router libraries, or as a separate stack?
- Should billing block responses on successful charge recording, or allow asynchronous charging with account balance safeguards?
- How should upstream cache hit state be normalized across providers that expose different usage metadata?
