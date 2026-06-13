# AetherCode Router

OpenAI-compatible completion router backed by shared database provider config.

## Run

```bash
cd router
go run ./cmd/router
```

Useful environment variables:

- `ROUTER_ADDR`: listen address, default `:8080`
- `SQL_DSN`: shared database DSN. Empty or `local` uses `router.db`
- `CONFIG_SYNC_INTERVAL`: provider cache reload interval, default `30s`
- `UPSTREAM_TIMEOUT`: upstream request timeout, default `5m`
- `UPSTREAM_MAX_RETRIES`: retry count across providers, default `2`
- `ROUTER_API_KEY`: optional public API key for `/v1/*`
- `ROUTER_ADMIN_KEY`: required for `/internal/providers*`

## Provider Config

Providers are stored in `router_providers`. Every container periodically reloads
the table and rebuilds an in-memory `group -> model -> providers` index, matching
the `new-api` cache-sync approach.

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
