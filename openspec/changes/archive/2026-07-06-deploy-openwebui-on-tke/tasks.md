## 1. Configuration Decisions

- [x] 1.1 Select and record the initial pinned Open WebUI image tag.
- [x] 1.2 Decide whether Open WebUI public access is enabled by default through the Terraform application CLB.
- [x] 1.3 Decide whether the relay API key is operator-provided in `secrets.env` or generated during deployment.

## 2. Kubernetes Manifests

- [x] 2.1 Add an `openwebui` namespace manifest under `platform/k8s/tke`.
- [x] 2.2 Add an Open WebUI ConfigMap with the relay internal base URL and IaC-first OpenAI-compatible settings.
- [x] 2.3 Add Open WebUI Secret generation or Secret references without committing real credentials.
- [x] 2.4 Add an Open WebUI PVC mounted at `/app/backend/data`.
- [x] 2.5 Add an Open WebUI Deployment using the pinned image, port `8080`, one replica, resource settings, and health checks.
- [x] 2.6 Add an Open WebUI ClusterIP Service.
- [x] 2.7 Add a fixed public NodePort Service for Open WebUI test access through Terraform CLB.
- [x] 2.8 Include the Open WebUI resources in `platform/k8s/tke/kustomization.yaml`.

## 3. Runtime Integration

- [x] 3.1 Configure Open WebUI to use `http://relay.aether-relay.svc.cluster.local/v1` as the OpenAI-compatible base URL.
- [x] 3.2 Configure Open WebUI to load its relay API key from Kubernetes Secret.
- [x] 3.3 Ensure PersistentConfig behavior is explicit so IaC-managed relay settings do not silently drift.
- [x] 3.4 Confirm Open WebUI can use the declared relay endpoint through the shared application CLB.

## 4. Documentation

- [x] 4.1 Update `platform/k8s/tke/README.md` with Open WebUI deployment and verification steps.
- [x] 4.2 Update `platform/RUNBOOK.md` with Open WebUI restore, parking, rollback, and secret rotation guidance.
- [x] 4.3 Update the root `README.md` architecture diagram to include Open WebUI and its ownership boundary.
- [x] 4.4 Document that the Open WebUI public entrypoint is Terraform-managed, with Kubernetes NodePort backends.

## 5. Verification

- [x] 5.1 Render the TKE overlay with example secrets and confirm all Open WebUI resources are present.
- [x] 5.2 Apply the overlay to a restored TKE environment.
- [x] 5.3 Wait for Open WebUI rollout and confirm the Terraform application CLB exposes Open WebUI.
- [x] 5.4 Confirm Open WebUI can reach the relay through the configured relay base URL.
- [x] 5.5 Run an end-to-end chat request through Open WebUI using a relay-backed model.
- [x] 5.6 Park the environment and confirm Open WebUI workloads scale down, public entrypoint is removed, and PVC data remains.
