## 1. Configuration Decisions

- [ ] 1.1 Select and record the initial pinned Open WebUI image tag.
- [ ] 1.2 Decide whether Open WebUI public access is enabled by default with a `LoadBalancer` Service.
- [ ] 1.3 Decide whether the relay API key is operator-provided in `secrets.env` or generated during deployment.

## 2. Kubernetes Manifests

- [ ] 2.1 Add an `openwebui` namespace manifest under `platform/k8s/tke`.
- [ ] 2.2 Add an Open WebUI ConfigMap with the relay internal base URL and IaC-first OpenAI-compatible settings.
- [ ] 2.3 Add Open WebUI Secret generation or Secret references without committing real credentials.
- [ ] 2.4 Add an Open WebUI PVC mounted at `/app/backend/data`.
- [ ] 2.5 Add an Open WebUI Deployment using the pinned image, port `8080`, one replica, resource settings, and health checks.
- [ ] 2.6 Add an Open WebUI ClusterIP Service.
- [ ] 2.7 Add an optional public `Service type=LoadBalancer` for Open WebUI test access.
- [ ] 2.8 Include the Open WebUI resources in `platform/k8s/tke/kustomization.yaml`.

## 3. Runtime Integration

- [ ] 3.1 Configure Open WebUI to use `http://relay.aether-relay.svc.cluster.local/v1` as the OpenAI-compatible base URL.
- [ ] 3.2 Configure Open WebUI to load its relay API key from Kubernetes Secret.
- [ ] 3.3 Ensure PersistentConfig behavior is explicit so IaC-managed relay settings do not silently drift.
- [ ] 3.4 Confirm Open WebUI does not depend on the relay public LoadBalancer Service for backend traffic.

## 4. Documentation

- [ ] 4.1 Update `platform/k8s/tke/README.md` with Open WebUI deployment and verification steps.
- [ ] 4.2 Update `platform/RUNBOOK.md` with Open WebUI restore, parking, rollback, and secret rotation guidance.
- [ ] 4.3 Update the root `README.md` architecture diagram to include Open WebUI and its ownership boundary.
- [ ] 4.4 Document that the Open WebUI public CLB is TKE cloud-controller-owned, not Terraform-managed.

## 5. Verification

- [ ] 5.1 Render the TKE overlay with example secrets and confirm all Open WebUI resources are present.
- [ ] 5.2 Apply the overlay to a restored TKE environment.
- [ ] 5.3 Wait for Open WebUI rollout and confirm the public Service receives a CLB hostname when enabled.
- [ ] 5.4 Confirm Open WebUI can reach the relay through cluster DNS.
- [ ] 5.5 Run an end-to-end chat request through Open WebUI using a relay-backed model.
- [ ] 5.6 Park the environment and confirm Open WebUI workloads scale down, public entrypoint is removed, and PVC data remains.
