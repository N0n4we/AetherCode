## Context

AetherCode currently runs an OpenAI-compatible relay platform on Tencent Cloud
TKE. Terraform owns the cloud substrate under `platform/terraform/tencentcloud`;
Kubernetes manifests under `platform/k8s/tke` own in-cluster workloads and
fixed NodePort backends. Terraform owns the EIP-backed application CLB used for
public test access.

The relay runtime is restorable from Terraform state and Kubernetes manifests.
Public test access is provided through a Terraform-managed application CLB that
forwards to fixed Kubernetes NodePorts.

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
- Keep a clean ownership boundary: Terraform for Tencent Cloud substrate and
  the public application CLB; Kubernetes manifests for workloads and NodePort
  backends.
- Configure Open WebUI to call the relay through the declared OpenAI-compatible
  base URL used by the public test path.
- Provide persistent Open WebUI state so accounts, settings, and chat history
  survive pod restarts and parking.
- Provide a simple public access path for test usage.
- Extend runbooks so parking, restoration, and smoke tests include Open WebUI.

**Non-Goals:**

- Replace the shared TCP application CLB with L7 Ingress/Gateway routing.
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
allowing explicit relay endpoint configuration.

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

Using the shared application CLB URL exercises the same relay endpoint that
browser/API clients use, which matches the requested domain-mapping workflow.

Alternative considered: configure the cluster-internal relay Service URL. That
keeps UI-to-relay traffic inside TKE, but it does not validate the public relay
listener that Open WebUI users need during this test deployment.

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

### Public Access Via Terraform Application CLB

For immediate test access, Open WebUI and relay are exposed through a single
Terraform-managed EIP-backed application CLB. Kubernetes provides fixed NodePort
Services as CLB backends: Open WebUI on `31327` and relay on `31326`.

Alternative considered: shared Ingress/Gateway. Good future direction once a
production domain/certificate strategy is selected, but out of scope for this
change.

### Secrets Stay Out Of Git

Open WebUI admin bootstrap credentials, `WEBUI_SECRET_KEY`, and relay API key
will be sourced from a local `secrets.env`-style file or manually created
Kubernetes Secret. The repository will only include examples/placeholders.

## Risks / Trade-offs

- Open WebUI PersistentConfig may ignore changed environment variables after
  first startup -> Set `ENABLE_PERSISTENT_CONFIG=False` for IaC-first behavior
  or document the manual update/reset path.
- A public entrypoint creates Tencent CLB/EIP cost -> use one shared
  Terraform-managed application CLB instead of per-Service default CLBs.
- Open WebUI default SQLite storage is single-pod oriented -> Run one replica by
  default and avoid multi-replica deployment unless an external database design
  is added.
- Parking removes public access -> scale/delete workloads and remove the
  Terraform application CLB while keeping PVC data and declarative manifests for
  restoration.
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

## Resolved Open Questions

The following decisions were made during implementation:

- **Pinned image tag:** `ghcr.io/open-webui/open-webui:v0.10.2` (latest stable
  release as of 2026-07-05). Pinned in `platform/k8s/tke/openwebui.yaml` and
  overridable through the `images` block in `kustomization.yaml`.
- **Public access default:** enabled by default through the shared
  Terraform-managed application CLB. `openwebui-public` is a fixed NodePort
  backend, not a Tencent Cloud default-domain CLB.
- **Relay API key provisioning:** operator-provided in
  `platform/k8s/tke/secrets.env` (key `openwebui-relay-api-key`). The overlay
  does not add an automated account-service call to generate the key; the
  operator creates the relay API key via the account service (see RUNBOOK
  section 9) and places it in `secrets.env`. This keeps the overlay simple and
  avoids a key-generation Job with secret material handling.
