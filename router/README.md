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
- `RELAY_ACCOUNT_KEY_AUTH`: set `true` to validate account-issued relay API keys
- `API_KEY_HASH_SECRET`: HMAC secret used to verify account-issued API keys
- `ACCOUNT_SERVICE_KEY`: bearer token accepted by the account service

## Provider Config

Providers are stored in `router_providers`. Provider config changes bump the
shared `router_config_versions.providers` version in the same DB transaction.
Every container polls the version and reloads only when it changes, then rebuilds
an in-memory `modelId -> providers` index. This follows the `new-api`
shared-DB cache-sync model without requiring pod-to-pod communication.

Routing is based on the public model IDs listed in each provider's `models`
field plus optional endpoint capabilities. Multiple enabled providers may list
the same model ID; the router chooses among those matching the requested
capability by highest `priority`, then by `weight` when multiple candidates have
that priority. Providers listing `*` act as wildcard fallback providers when no
exact model ID is configured.

`endpoint_capabilities` is optional. Providers without explicit capabilities
default to the current OpenAI-compatible completion routes:
`openai.chat_completions` and `openai.completions`. Supported capability
identifiers are:

- `openai.chat_completions`
- `openai.completions`
- `openai.embeddings`
- `openai.images`
- `openai.audio`
- `openai.responses`
- `openai.rerank`
- `claude.messages`
- `gemini.generate`
- `realtime`
- `task.video`

`channel_type` and `relay_format` are optional metadata fields reserved for
future adaptor routing. `/internal/status` includes aggregate
`cache.capability_counts` for enabled providers.

The `groups` field is still accepted and returned by the admin API for backward
compatibility, but it is legacy/internal metadata. Public OpenAI-compatible
requests cannot steer routing with a body `group` field or with
`X-Aether-Group` / `X-Router-Group` headers. Body `group` is stripped before the
request is sent upstream.

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
    "endpoint_capabilities": [
      "openai.chat_completions",
      "openai.completions"
    ],
    "channel_type": "openai",
    "relay_format": "openai-compatible",
    "groups": "default",
    "status": 1,
    "priority": 10,
    "weight": 100
  }'
```

Operational routes:

- `GET /healthz`
- `GET /readyz`
- `GET /internal/status`

## Platform Mode

The image now contains three platform entrypoints:

- `/router`: OpenAI-compatible relay and provider-channel admin APIs
- `/account-service`: account-scoped relay API key lifecycle service
- `/migrate`: shared platform schema migration entrypoint

`/account-service` exposes:

- `POST /account/api-keys`
- `GET /account/api-keys`
- `POST /account/api-keys/{id}/disable`
- `POST /account/api-keys/{id}/revoke`

Account requests require `Authorization: Bearer $ACCOUNT_SERVICE_KEY` and
`X-Aether-Account-ID`. Key create responses return the raw `secret` once; list,
disable, and revoke responses only return safe key metadata.

`/router` exposes platform provider-channel admin routes:

- `GET /internal/provider-channels`
- `POST /internal/provider-channels`
- `GET /internal/provider-channels/{id}`
- `PUT /internal/provider-channels/{id}`
- `POST /internal/provider-channels/{id}/disable`
- `DELETE /internal/provider-channels/{id}`

Each provider channel accepts exactly one non-empty public `model_id`. Enabled
channels are projected into the router provider cache and preserve the existing
priority/weight selection behavior when multiple channels share a modelId.
`upstream_api_key_secret_ref` supports `env:NAME` and `file:/path/to/key`; the
relay resolves the reference into `Provider.APIKey` during cache projection and
does not expose raw upstream secrets through admin responses.

## Route Compatibility Matrix

Relay route registration is descriptor-driven. Registered routes include method,
path pattern, route family, endpoint capability, response format, and
implementation status. Unknown paths are left as normal not-found responses;
registered but unimplemented routes return HTTP `501` with error code
`unsupported_endpoint`.

Implemented relay routes:

| Method | Path | Family | Capability | Format |
| --- | --- | --- | --- | --- |
| POST | `/v1/chat/completions` | OpenAI | `openai.chat_completions` | OpenAI |
| POST | `/v1/completions` | OpenAI | `openai.completions` | OpenAI |

Metadata routes:

| Method | Path | Family | Source | Format |
| --- | --- | --- | --- | --- |
| GET | `/v1/models` | OpenAI | provider cache modelIds | OpenAI list |
| GET | `/v1/models/{model}` | OpenAI | provider cache modelIds | OpenAI model |
| GET | `/v1beta/models` | Gemini | provider cache modelIds | Gemini list |
| GET | `/v1beta/openai/models` | OpenAI | provider cache modelIds | OpenAI list |

Unsupported shell routes:

| Method | Path | Family | Capability |
| --- | --- | --- | --- |
| POST | `/v1/responses` | OpenAI | `openai.responses` |
| POST | `/v1/responses/compact` | OpenAI | `openai.responses` |
| POST | `/v1/embeddings` | OpenAI | `openai.embeddings` |
| POST | `/v1/images/generations` | OpenAI | `openai.images` |
| POST | `/v1/images/edits` | OpenAI | `openai.images` |
| POST | `/v1/audio/transcriptions` | OpenAI | `openai.audio` |
| POST | `/v1/audio/translations` | OpenAI | `openai.audio` |
| POST | `/v1/audio/speech` | OpenAI | `openai.audio` |
| POST | `/v1/rerank` | OpenAI | `openai.rerank` |
| POST | `/v1/messages` | Claude | `claude.messages` |
| POST | `/v1beta/models/{model}:generateContent` | Gemini | `gemini.generate` |
| POST | `/v1beta/models/{model}:streamGenerateContent` | Gemini | `gemini.generate` |
| GET | `/v1/realtime` | Realtime | `realtime` |
| POST | `/v1/tasks/video` | Task | `task.video` |
| GET | `/v1/tasks/{task_id}` | Task | `task.video` |
| POST | `/v1/videos/generations` | Task | `task.video` |
| GET | `/v1/videos/{task_id}` | Task | `task.video` |

Successful proxied completion responses include:

- `X-Aether-Router-Instance`
- `X-Aether-Provider-ID`
- `X-Aether-Provider-Name`
- `X-Aether-Provider-Version`

## OpenAI-Compatible Relay

`POST /v1/chat/completions` and `POST /v1/completions` use the same internal
relay pipeline. The pipeline validates method, public API key, maximum body
size, JSON syntax, and a non-empty string `model` before selecting a provider.
The request body is replayable across attempts: client-only routing hints such
as `group` are stripped, the selected provider's model mapping rewrites
`model`, and all other JSON fields pass through unchanged.

Provider selection filters by endpoint capability before dispatch:
`/v1/chat/completions` requires `openai.chat_completions`, and
`/v1/completions` requires `openai.completions`. Dispatch uses the configured
upstream auth header/prefix, API key, and extra provider headers. Hop-by-hop
response headers are filtered, while normal upstream response headers and the
`X-Aether-*` provider metadata are returned to the client.

Retries happen only before anything has been committed to the client. Network
errors, `429`, and `5xx` responses are retried up to `UPSTREAM_MAX_RETRIES`,
excluding providers already attempted for that request. If all eligible
providers fail with retryable errors, the router returns a `502`
OpenAI-compatible `upstream_error`. Non-retryable upstream `4xx` responses are
returned as-is. Once response headers or body bytes have been written, including
during streaming responses, the router does not retry another provider.

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
