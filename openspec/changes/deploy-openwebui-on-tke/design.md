## Context

AetherCode currently runs an OpenAI-compatible relay platform on Tencent Cloud
TKE. Terraform owns the cloud substrate under `platform/terraform/tencentcloud`;
Kubernetes manifests under `platform/k8s/tke` own in-cluster workloads and
`Service type=LoadBalancer` business entrypoints.

The relay runtime is currently parked: node pool maximum size is set to `0`,
relay/account/postgres workloads are scaled to `0`, and the public relay/account
LoadBalancer Services were deleted. The platform remains restorable from the
existing Terraform state and Kubernetes manifests.

Open WebUI should be deployed as a browser UI for the relay. It should call the
relay through the in-cluster OpenAI-compatible endpoint:

```text
http://relay.aether-relay.svc.cluster.local/v1
```

Official Open WebUI deployment guidance uses
`ghcr.io/open-webui/open-webui:<tag>`, exposes container port `8080`, and
persists `/app/backend/data`. Its OpenAI-compatible configuration is controlled
by `ENABLE_OPENAI_API`, `OPENAI_API_BASE_URL`, and `OPENAI_API_KEY`. These
OpenAI-related settings are PersistentConfig values, so after first startup they
may be read from Open WebUI's internal database instead of the external
environment unless persistent config is disabled.

## Goals / Non-Goals

**Goals:**

- Add IaC-managed Kubernetes resources for Open WebUI on TKE.
- Keep the existing ownership boundary: Terraform for Tencent Cloud substrate,
  Kubernetes manifests for workloads and cloud-controller-created CLBs.
- Configure Open WebUI to call the relay through the cluster-internal Service
  URL.
- Provide persistent Open WebUI state so accounts, settings, and chat history
  survive pod restarts and parking.
- Provide a simple public access path for test usage.
- Extend runbooks so parking, restoration, and smoke tests include Open WebUI.

**Non-Goals:**

- Replace the current `Service type=LoadBalancer` approach with shared
  Ingress/Gateway routing.
- Integrate SSO/OAuth, external identity providers, or production-grade user
  lifecycle management.
- Migrate Open WebUI to an external PostgreSQL database.
- Manage TCR Personal or Open WebUI upstream image repositories with Terraform.
- Store generated admin credentials, relay API keys, or OpenRouter keys in the
  repository.

## Decisions

### Deploy Open WebUI With Kubernetes Manifests

Open WebUI will be added under `platform/k8s/tke`, not Terraform. This keeps the
existing owner boundary intact and avoids mixing Terraform Kubernetes provider
state with direct `kubectl apply -k` usage.

Alternative considered: manage Open WebUI with Terraform's Kubernetes or Helm
provider. Rejected for now because it creates a second Kubernetes ownership
model and complicates the current restore workflow.

### Use A Dedicated Namespace

Open WebUI will run in a dedicated namespace, `openwebui`, with its own
Deployment, Service, PVC, ConfigMap, and Secret.

This keeps UI lifecycle and data separate from `aether-relay` while still
allowing stable cluster DNS access to `relay.aether-relay.svc.cluster.local`.

Alternative considered: place Open WebUI in `aether-relay`. Rejected because
the UI has different data, access, and parking concerns than the relay control
plane.

### Use The Official Container Image And Pin A Version

The workload will use `ghcr.io/open-webui/open-webui:<version>` and expose port
`8080`. The implementation should pin a concrete release tag rather than `main`
to avoid uncontrolled upgrades and database migrations.

Alternative considered: use `:main` for fastest setup. Rejected because the
deployment is IaC-managed and should be reproducible.

### Persist `/app/backend/data` With A PVC

Open WebUI data will be stored in a PersistentVolumeClaim mounted at
`/app/backend/data`. This preserves the SQLite database, uploaded data, and
internal configuration across pod restarts and parking.

Alternative considered: stateless Open WebUI. Rejected because it would lose
accounts and settings on pod replacement.

### Connect To Relay Through The In-Cluster Service

Open WebUI will configure:

```text
ENABLE_OPENAI_API=True
OPENAI_API_BASE_URL=http://relay.aether-relay.svc.cluster.local/v1
OPENAI_API_KEY=<relay API key from Kubernetes Secret>
```

Using the cluster-internal relay URL avoids public CLB hairpinning and keeps UI
to relay traffic inside TKE networking.

Alternative considered: configure the public relay CLB URL. Rejected for the
default path because the relay public LoadBalancer may be deleted during
parking, while the internal Service is recreated by the overlay.

### Treat Open WebUI PersistentConfig Explicitly

Because Open WebUI persists some environment-derived settings internally, the
implementation must choose one of these operating modes:

1. Set `ENABLE_PERSISTENT_CONFIG=False` so IaC environment values remain the
   source of truth on every restart.
2. Leave persistent config enabled and document that later changes to
   `OPENAI_API_BASE_URL`, `OPENAI_API_KEY`, and related settings may require UI
   admin changes or PVC reset.

For an IaC-first test environment, prefer `ENABLE_PERSISTENT_CONFIG=False`.
This makes relay endpoint and key rotation predictable from manifests and
Secrets, at the cost of not persisting admin UI changes to those settings.

### Public Access Via A Kubernetes LoadBalancer Service

For immediate test access, Open WebUI will expose a public
`Service type=LoadBalancer`. The resulting Tencent CLB will be owned by the TKE
cloud controller, not Terraform, matching the relay/account service pattern.

Alternative considered: shared Ingress/Gateway. Good future direction, but out
of scope for this change.

### Secrets Stay Out Of Git

Open WebUI admin bootstrap credentials, `WEBUI_SECRET_KEY`, and relay API key
will be sourced from a local `secrets.env`-style file or manually created
Kubernetes Secret. The repository will only include examples/placeholders.

## Risks / Trade-offs

- Open WebUI PersistentConfig may ignore changed environment variables after
  first startup -> Set `ENABLE_PERSISTENT_CONFIG=False` for IaC-first behavior
  or document the manual update/reset path.
- A public LoadBalancer creates another Tencent CLB -> Accept for the first
  implementation; track shared Ingress/Gateway as a later optimization.
- Open WebUI default SQLite storage is single-pod oriented -> Run one replica by
  default and avoid multi-replica deployment unless an external database design
  is added.
- Parking deletes public LoadBalancer Services -> Restore with
  `kubectl apply -k platform/k8s/tke` and wait for the new Open WebUI CLB
  hostname.
- Relay API key rotation can break Open WebUI -> Store the key in a Secret and
  restart Open WebUI after rotation.

## Migration Plan

1. Restore the TKE node pool if the environment is parked.
2. Restore relay/postgres/account from `platform/k8s/tke` and confirm relay
   health.
3. Generate or provide a relay API key for Open WebUI.
4. Apply the Open WebUI Kubernetes resources.
5. Wait for the Deployment rollout and public Service hostname.
6. Verify that Open WebUI can list relay models and complete a chat request.

Rollback:

1. Delete the Open WebUI public Service to release its CLB.
2. Scale Open WebUI Deployment to `0` or delete the Open WebUI overlay.
3. Keep the PVC if data should be preserved; delete the PVC only when a full
   reset is intended.

## Open Questions

- Which Open WebUI release tag should be pinned for the initial deployment?
- Should Open WebUI be reachable publicly, or only through port-forward/Tailscale
  during testing?
- Should the implementation add an automated account-service call to create the
  relay API key, or require the operator to provide it in `secrets.env`?
