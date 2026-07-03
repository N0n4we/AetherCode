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

`relay-public` is a Tencent Cloud `LoadBalancer` service exposing the relay HTTP
API. `account-public` is a Tencent Cloud `LoadBalancer` service exposing the
account key lifecycle API, which still requires `Authorization: Bearer
$ACCOUNT_SERVICE_KEY` plus `X-Aether-Account-ID`.

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
