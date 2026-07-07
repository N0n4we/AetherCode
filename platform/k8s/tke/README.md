# TKE Deployment

This overlay deploys Postgres, the router, and the account service into the
existing TKE cluster namespace `aether-relay`.

Before applying, create a local `secrets.env` file from the example:

```bash
cp platform/k8s/tke/secrets.env.example platform/k8s/tke/secrets.env
```

Fill in the Postgres password and keys, including `openrouter-api-key`, then
deploy:

```bash
kubectl apply -k platform/k8s/tke
kubectl -n aether-relay rollout status statefulset/postgres
kubectl -n aether-relay rollout status deployment/relay
kubectl -n aether-relay rollout status deployment/account
kubectl -n aether-relay wait --for=condition=complete job/sync-openrouter-provider-channels-once --timeout=300s
kubectl -n aether-relay get svc relay-public account-public
```

`relay-public` is a fixed `NodePort` Service used as the Kubernetes backend for
the Terraform-managed application CLB. `account-public` is intentionally omitted
from the default public path; the account key lifecycle API remains internal
unless a separate authenticated public Service is applied.

`SQL_DSN` points at the in-cluster Postgres service:

```text
postgres://router:<url-encoded-password>@postgres.aether-relay.svc.cluster.local:5432/router?sslmode=disable
```

Use the raw password for `postgres-password`, but URL-encode it in `sql-dsn`
when it contains characters such as `/`, `+`, `=`, `%`, or `@`.

The OpenRouter provider-channel sync job reconciles free-model channels for:

- `poolside/laguna-xs-2.1:free`
- `cohere/north-mini-code:free`

It stores only `env:OPENROUTER_API_KEY` in the database. The raw key stays in
the Kubernetes Secret generated from local `secrets.env`.

`sync-openrouter-provider-channels-once` initializes a fresh environment
immediately. `sync-openrouter-provider-channels` runs every 15 minutes and
updates managed fields on existing channels, so DB drift such as priority,
upstream URL, auth header/prefix, model mapping, or enabled status is corrected
from this overlay. It does not delete unmanaged provider channels.

## Open WebUI

In addition to the relay runtime, this overlay deploys Open WebUI into a
dedicated `openwebui` namespace as a browser UI for the relay. The running test
environment points Open WebUI at the Terraform-managed public relay listener so
browser and Open WebUI traffic use the same public application CLB.

Open WebUI resources live in `openwebui.yaml`:

- Namespace `openwebui`
- ConfigMap `openwebui-config` with the relay OpenAI-compatible base URL and
  IaC-first settings (`ENABLE_PERSISTENT_CONFIG=False`)
- Secret `openwebui-secrets` generated from `openwebui-secrets.env`
- PVC `openwebui-data` mounted at `/app/backend/data`
- Deployment `openwebui` using pinned image
  `ghcr.io/open-webui/open-webui:v0.10.2` (overridable via `kustomization.yaml`)
- ClusterIP Service `openwebui`
- NodePort Service `openwebui-public` on `31327` for the Terraform CLB backend

Create the local Open WebUI secret file:

```bash
cp platform/k8s/tke/openwebui-secrets.env.example \
   platform/k8s/tke/openwebui-secrets.env
```

Fill in `openwebui-relay-api-key` with a relay API key created via the account
service (see `platform/RUNBOOK.md` section 9), and `webui-secret-key` with a
random value (`openssl rand -base64 32`). Then deploy alongside the relay:

```bash
kubectl apply -k platform/k8s/tke
kubectl -n openwebui rollout status deployment/openwebui
kubectl -n openwebui get svc openwebui openwebui-public
kubectl -n openwebui get pvc openwebui-data
```

`openwebui-public` is a fixed `NodePort` Service used as the Kubernetes backend
for the Terraform-managed application CLB. The shared public CLB exposes Open
WebUI on `https://openwebui.n0n4w3.cn` and relay on
`https://relay.n0n4w3.cn`.

Open WebUI is configured with:

```text
ENABLE_OPENAI_API=True
OPENAI_API_BASE_URL=https://relay.n0n4w3.cn/v1
OPENAI_API_KEY=<openwebui-relay-api-key from openwebui-secrets Secret>
ENABLE_PERSISTENT_CONFIG=False
```

`ENABLE_PERSISTENT_CONFIG=False` keeps manifest and Secret values as the source
of truth on every restart, so relay endpoint or key rotation applied via the
overlay takes effect after a pod restart rather than being shadowed by Open
WebUI's internal database.
