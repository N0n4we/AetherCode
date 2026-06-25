# Relay Platform Foundation

This document covers the initial best-effort platform introduced by
`relay-platform-foundation`.

## Local Integration

Run a local PostgreSQL database, then start migration, account, and relay
processes against the same `SQL_DSN`.

```bash
docker run --rm --name aether-relay-postgres \
  -e POSTGRES_DB=relay \
  -e POSTGRES_USER=relay \
  -e POSTGRES_PASSWORD=relay \
  -p 5432:5432 postgres:16-alpine
```

```bash
cd router
export SQL_DSN='postgres://relay:relay@localhost:5432/relay?sslmode=disable'
export API_KEY_HASH_SECRET='local-hash-secret'
export ACCOUNT_SERVICE_KEY='local-account-service-key'
export ROUTER_ADMIN_KEY='local-admin-key'
go run ./cmd/migrate
ROUTER_ADDR=:8081 go run ./cmd/account-service
RELAY_ACCOUNT_KEY_AUTH=true ROUTER_ADDR=:8080 go run ./cmd/router
```

Set `IMPORT_LEGACY_PROVIDERS=true` when running `/migrate` to create
single-model provider-channel records from existing `router_providers` rows.
The import creates one channel per legacy modelId and stores only a secret
reference such as `legacy-router-providers/{id}/api_key`, not the raw upstream
API key.

Create an account-issued relay key:

```bash
curl -s http://localhost:8081/account/api-keys \
  -H 'Authorization: Bearer local-account-service-key' \
  -H 'X-Aether-Account-ID: acct-local' \
  -H 'Content-Type: application/json' \
  -d '{"name":"local relay key"}'
```

Use the returned `secret` as `Authorization: Bearer ...` for relay requests.
The secret is returned only from the create response; list, disable, and revoke
responses include only safe metadata.

## Best-Effort Availability

The first cloud target is GKE Autopilot plus Cloud SQL for PostgreSQL. This is a
best-effort cluster: it has managed infrastructure, readiness checks, rolling
deployments, and Cloud SQL backups, but it does not provide a strict high
availability SLO, multi-region failover, or zero-downtime database maintenance.

Operational recovery steps:

1. Check `GET /readyz` on relay and account pods. Relay readiness requires the
   provider cache and, in account-key mode, API key cache to be loaded from RDB.
2. Check Cloud SQL availability and connection limits.
3. Restart unhealthy pods after confirming the database is reachable.
4. Re-run `/migrate` after failed or partial rollout attempts.
5. Roll back Kubernetes Deployments to the previous ReplicaSet if a new image
   cannot authenticate, sync provider config, or write usage records.
6. Fall back to static `ROUTER_API_KEY` compatibility only for controlled
   recovery windows.

## Deployment

Provision the first supported environment:

```bash
cd platform/terraform/environments/gcp-dev
terraform init
terraform apply \
  -var='project_id=YOUR_PROJECT' \
  -var='api_key_hash_secret=CHANGE_ME' \
  -var='account_service_key=CHANGE_ME'
```

Store sensitive Terraform outputs in the runtime secret backend and configure
the `ExternalSecret` references in `platform/k8s/base/secret-references.yaml`.
The Kubernetes workloads expect a Secret named `relay-runtime-secrets` with:

- `sql-dsn`
- `router-api-key`
- `router-admin-key`
- `api-key-hash-secret`
- `account-service-key`

Update `platform/k8s/base/serviceaccounts.yaml` with Terraform service account
outputs and update `platform/k8s/base/gateway.yaml` with the real hostname and
certificate references.

Apply:

```bash
kubectl apply -k platform/k8s/base
kubectl -n aether-relay rollout status deploy/relay
kubectl -n aether-relay rollout status deploy/account
```

Rollback:

```bash
kubectl -n aether-relay rollout undo deploy/relay
kubectl -n aether-relay rollout undo deploy/account
```

## End-To-End Verification

1. Run migrations with the deployment init container or `kubectl -n aether-relay
   create job --from=deployment/relay migrate-once`.
2. Create an account key through `/account/api-keys`.
3. Create at least one provider channel through `/internal/provider-channels`
   with exactly one `model_id`.
   Set `upstream_api_key_secret_ref` to `env:NAME` for an environment-backed
   secret or `file:/path/to/key` for a mounted secret file. The relay resolves
   that reference into the upstream API key only when projecting the channel into
   its routing cache; admin responses continue to expose only the reference.
4. Wait for `/readyz` to report ready on relay pods.
5. Call `POST /v1/chat/completions` through Gateway using the account-issued
   key.
6. Confirm `relay_usage_events` contains the request ID, account ID, API key ID,
   public modelId, selected provider channel, endpoint capability, cache state,
   upstream status, and timing metadata.
7. Confirm `relay_billable_charges` has one charge for the usage event and that
   retrying the same `X-Request-ID` does not create a duplicate charge.

Verification commands used for this implementation:

```bash
cd router
PATH=/private/tmp/aether-go/go/bin:$PATH GOPROXY=https://goproxy.cn,direct GOSUMDB=off go test ./internal/app -run 'TestRelayUsageDoesNotDeduplicateDistinctRequestsByClientRequestID|TestProviderChannelSecretRefSuppliesUpstreamAPIKey' -count=1
PATH=/private/tmp/aether-go/go/bin:$PATH GOPROXY=https://goproxy.cn,direct GOSUMDB=off go test ./...
cd ..
terraform fmt -recursive platform/terraform
terraform -chdir=platform/terraform/environments/gcp-dev init -backend=false
terraform -chdir=platform/terraform/environments/gcp-dev validate
kubectl kustomize platform/k8s/base
openspec validate relay-platform-foundation --strict
```
