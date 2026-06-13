# AetherCode Router

OpenAI-compatible completion router backed by shared database provider config.

## Run

```bash
cd router
go run ./cmd/router
```

Useful environment variables:

- `ROUTER_ADDR`: listen address, default `:8080`
- `ROUTER_INSTANCE_ID`: instance id used in status and response headers, default hostname
- `SQL_DSN`: shared database DSN. Empty or `local` uses `router.db`
- `CONFIG_SYNC_INTERVAL`: provider config version polling interval, default `5s`
- `UPSTREAM_TIMEOUT`: upstream request timeout, default `5m`
- `UPSTREAM_MAX_RETRIES`: retry count across providers, default `2`
- `ROUTER_API_KEY`: optional public API key for `/v1/*`
- `ROUTER_ADMIN_KEY`: required for `/internal/providers*`

## Provider Config

Providers are stored in `router_providers`. Provider config changes bump the
shared `router_config_versions.providers` version in the same DB transaction.
Every container polls the version and reloads only when it changes, then rebuilds
an in-memory `group -> model -> providers` index. This follows the `new-api`
shared-DB cache-sync model without requiring pod-to-pod communication.

Minimal admin create request:

```bash
curl -X POST http://localhost:8080/internal/providers \
  -H 'Authorization: Bearer admin-key' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "OpenAI",
    "provider": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-...",
    "models": "gpt-4o-mini,gpt-4o",
    "groups": "default",
    "status": 1,
    "priority": 10,
    "weight": 100
  }'
```

Supported routes:

- `POST /v1/chat/completions`
- `POST /v1/completions`
- `GET /healthz`
- `GET /readyz`
- `GET /internal/status`

Successful `/v1/*` responses include:

- `X-Aether-Router-Instance`
- `X-Aether-Provider-Id`
- `X-Aether-Provider-Name`
- `X-Aether-Provider-Version`

## k3d Test

The local distributed test builds the router image, creates a k3d k3s cluster,
deploys Postgres, one mock OpenAI-compatible provider, and three router replicas.
It writes provider config through one router entrypoint, waits until each replica
reports the same DB/cache provider version, then verifies non-stream and stream
requests through every router pod.

```bash
cd ..
./AetherCode/router/scripts/k3d-test.sh
```

Set `KEEP_K3D=1` to keep the cluster after a run for debugging.
