## Purpose

Define the Open WebUI runtime capability on the Tencent Cloud TKE platform: an IaC-managed Kubernetes workload backed by the in-cluster relay, with persistent state, optional public test access, secret hygiene, and operational verification.

## Requirements

### Requirement: Open WebUI Kubernetes Deployment
The platform SHALL define an IaC-managed Open WebUI Kubernetes workload for the
Tencent Cloud TKE runtime.

#### Scenario: Open WebUI workload is applied
- **WHEN** the Open WebUI Kubernetes overlay is applied
- **THEN** Kubernetes creates an Open WebUI Deployment and ClusterIP Service in
  a dedicated namespace

#### Scenario: Open WebUI uses a pinned image
- **WHEN** operators inspect the Open WebUI Deployment
- **THEN** the container image uses a concrete Open WebUI release tag rather
  than an unpinned floating tag

#### Scenario: Open WebUI exposes the expected container port
- **WHEN** Open WebUI is deployed
- **THEN** the workload exposes container port `8080` through a Kubernetes
  Service

### Requirement: Open WebUI Persistent State
The platform SHALL persist Open WebUI runtime state across pod restarts and
parking cycles.

#### Scenario: Persistent volume is mounted
- **WHEN** Open WebUI starts
- **THEN** a PersistentVolumeClaim is mounted at `/app/backend/data`

#### Scenario: Parking preserves Open WebUI data
- **WHEN** Open WebUI is scaled to zero during parking
- **THEN** its PersistentVolumeClaim remains available for later restoration

### Requirement: Relay Backing Configuration
Open WebUI SHALL be configured to use the in-cluster relay as its
OpenAI-compatible backend.

#### Scenario: Open WebUI points at relay service DNS
- **WHEN** operators inspect the Open WebUI configuration
- **THEN** the OpenAI-compatible base URL is
  `http://relay.aether-relay.svc.cluster.local/v1`

#### Scenario: Open WebUI uses a relay API key from Secret
- **WHEN** Open WebUI calls the relay
- **THEN** it uses an API key sourced from a Kubernetes Secret rather than a
  repository-tracked value

#### Scenario: Open WebUI can reach relay through the declared relay endpoint
- **WHEN** relay is running and the relay endpoint is configured in Open WebUI
- **THEN** Open WebUI can send OpenAI-compatible traffic to the relay through
  the configured base URL

### Requirement: Open WebUI Public Test Access
The platform SHALL provide an IaC-managed public test entrypoint for Open WebUI
when public access is enabled.

#### Scenario: Public service creates TKE-managed CLB
- **WHEN** the Open WebUI public Service and Terraform public CLB are applied
- **THEN** Open WebUI is reachable through the Terraform-managed application CLB
  with a fixed Kubernetes NodePort backend

#### Scenario: Public entrypoint deletion releases public access
- **WHEN** operators remove the Terraform application CLB during parking
- **THEN** the public entrypoint is removed without deleting Open WebUI
  persistent data

### Requirement: Open WebUI Secret Hygiene
The Open WebUI deployment SHALL keep credentials and generated keys outside
version control.

#### Scenario: Repository contains placeholders only
- **WHEN** operators inspect tracked Open WebUI secret examples
- **THEN** they contain placeholders and do not contain real admin credentials,
  relay API keys, OpenRouter keys, or session secrets

#### Scenario: Runtime secret can be rotated
- **WHEN** the relay API key or Open WebUI secret key changes
- **THEN** operators can update the Kubernetes Secret and restart Open WebUI
  without modifying tracked manifests with raw secrets

### Requirement: Open WebUI Operational Verification
The platform SHALL document and support smoke tests proving that Open WebUI is
connected to the relay.

#### Scenario: Open WebUI rollout succeeds
- **WHEN** Open WebUI manifests are applied
- **THEN** operators can wait for the Deployment rollout to complete

#### Scenario: Open WebUI relay smoke test succeeds
- **WHEN** Open WebUI is configured with a valid relay API key and the relay has
  an enabled OpenAI-compatible provider channel
- **THEN** a user can submit a chat request through Open WebUI and receive a
  relay-backed model response