## Why

The platform currently exposes an OpenAI-compatible relay API, but there is no
hosted browser UI for interactive model testing and account workflows. Operators
need Open WebUI deployed on Tencent Cloud with the same IaC discipline as the
relay runtime, so the environment can be recreated after parking and tested
without manual console setup.

This change makes Open WebUI a declared TKE workload that connects to the
in-cluster relay endpoint instead of depending on public CLB traffic between
services.

## What Changes

- Add an Open WebUI Kubernetes runtime under `platform/k8s/tke`.
- Deploy Open WebUI into a dedicated namespace with Deployment, Service,
  ConfigMap, Secret references, and persistent storage.
- Configure Open WebUI to use the relay's in-cluster OpenAI-compatible base URL:
  `http://relay.aether-relay.svc.cluster.local/v1`.
- Provide an IaC-managed public access option for Open WebUI through a Kubernetes
  `Service type=LoadBalancer`, while documenting that the resulting CLB is owned
  by the TKE cloud controller rather than Terraform.
- Extend platform runbooks to cover Open WebUI deployment, parking, restoration,
  and smoke testing.
- Keep Open WebUI secrets and generated credentials outside the repository.

Out of scope for this change:

- Replacing the current per-Service CLB approach with a shared Ingress or
  Gateway.
- Managing the TCR Personal repository through Terraform.
- Storing Open WebUI admin passwords, OpenRouter keys, or relay API keys in
  version control.

## Capabilities

### New Capabilities

- `openwebui-tke-runtime`: Deploy and operate Open WebUI on Tencent Cloud TKE as
  an IaC-managed Kubernetes workload connected to the relay.

### Modified Capabilities

- `cloud-relay-runtime`: Extend the platform runtime boundary to include an
  optional UI workload and its restore/parking implications.
