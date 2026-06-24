## 1. Runtime Foundation

- [ ] 1.1 Select the first supported cloud Kubernetes and RDB target, then update `design.md` open questions with the decision.
- [ ] 1.2 Add Terraform environment structure for network, Kubernetes, RDB, and ingress or Gateway API prerequisites.
- [ ] 1.3 Add Terraform outputs for non-secret deployment references needed by Kubernetes and services.
- [ ] 1.4 Add Terraform sensitive variable and output handling for database and runtime secrets.
- [ ] 1.5 Add Kubernetes namespace, service account, config, and secret-reference structure for platform workloads.
- [ ] 1.6 Add ingress or Gateway API resources that route relay API paths to the relay service and account API paths to the account service.

## 2. Service Deployment

- [ ] 2.1 Add relay Kubernetes deployment and service configuration with image, replicas, resource settings, and probes.
- [ ] 2.2 Add account service Kubernetes deployment and service configuration with image, replicas, resource settings, and probes.
- [ ] 2.3 Add rollout/readiness behavior that gates relay and account traffic on database and cache readiness.
- [ ] 2.4 Document best-effort cluster availability expectations and operational recovery steps.

## 3. RDB Schema And Migrations

- [ ] 3.1 Add migration tooling or migration entrypoints for shared platform schema changes.
- [ ] 3.2 Add account and API key metadata tables with hashed-key verifier fields and key state.
- [ ] 3.3 Add provider channel tables with one public modelId per channel, capabilities, priority, weight, upstream mapping, and secret references.
- [ ] 3.4 Add config version tracking for API key and provider channel cache synchronization.
- [ ] 3.5 Add pricing configuration, usage event, and billable charge tables with effective-time and idempotency fields.

## 4. Account API Key Service

- [ ] 4.1 Create the account service module or package with database configuration and health endpoints.
- [ ] 4.2 Implement API key creation that returns the raw secret only once and stores only safe verifier data.
- [ ] 4.3 Implement API key list, disable, and revoke operations without exposing raw key secrets.
- [ ] 4.4 Add account API tests for key creation, listing, revocation, secret redaction, and ownership checks.
- [ ] 4.5 Add key-state version bumping so relay instances can observe account key changes without restart.

## 5. Provider Channel Control Plane

- [ ] 5.1 Implement provider channel create, update, list, disable, and delete operations against RDB.
- [ ] 5.2 Validate that platform provider channels accept exactly one non-empty public modelId.
- [ ] 5.3 Preserve existing router selection semantics for multiple channels sharing one modelId by priority and weight.
- [ ] 5.4 Redact upstream provider secret values from provider channel responses and logs.
- [ ] 5.5 Add compatibility projection or migration from existing router provider rows to single-modelId channel records.

## 6. Relay Integration

- [ ] 6.1 Replace or extend static `ROUTER_API_KEY` auth with account-issued API key validation in compatibility mode.
- [ ] 6.2 Add relay key-validation cache sync using the API key config version or key-state sync signal.
- [ ] 6.3 Attach account and API key identity to authenticated relay request context.
- [ ] 6.4 Load provider channel configuration into the relay provider cache through versioned RDB sync.
- [ ] 6.5 Add relay tests for active, missing, disabled, and revoked account-issued API keys.
- [ ] 6.6 Add relay tests for provider channel cache updates and single-modelId channel selection.

## 7. Usage And Dynamic Billing

- [ ] 7.1 Add finalized relay usage event creation for successful authenticated requests.
- [ ] 7.2 Add auditable usage event creation for authenticated failed requests according to billing policy.
- [ ] 7.3 Capture public modelId, selected channel, endpoint capability, usage units, cache state, upstream status, and timing metadata in usage events.
- [ ] 7.4 Implement dynamic price resolution by account, modelId, channel, usage class, cache state, and effective time.
- [ ] 7.5 Implement idempotent billable charge creation from usage events.
- [ ] 7.6 Add billing tests for price changes, channel-specific prices, cache-hit pricing, duplicate usage writes, and reconciliation traceability.

## 8. Verification And Documentation

- [ ] 8.1 Add local integration documentation for running RDB, relay, account service, and migrations together.
- [ ] 8.2 Add deployment documentation covering Terraform apply, Kubernetes rollout, secret setup, ingress or Gateway API, and rollback.
- [ ] 8.3 Add end-to-end verification that an account-issued key can call a relay route through ingress and produce usage/billing records.
- [ ] 8.4 Run router and platform test suites and record the verification commands in the change notes or final implementation summary.
