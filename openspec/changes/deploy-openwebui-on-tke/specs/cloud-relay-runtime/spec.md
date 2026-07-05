## MODIFIED Requirements

### Requirement: Best Effort Kubernetes Cluster
The platform SHALL run on a best-effort Kubernetes cluster while still defining health, readiness, resource, rollout, and parking controls for service workloads and optional UI workloads.

#### Scenario: Workloads declare health behavior
- **WHEN** relay, account, and optional UI workloads are deployed
- **THEN** each workload has health behavior suitable for Kubernetes rollout and traffic gating

#### Scenario: Best effort availability is explicit
- **WHEN** operators inspect the runtime configuration
- **THEN** the configuration identifies the cluster as best-effort and does not imply a strict high-availability SLO

#### Scenario: Parking preserves restorable state
- **WHEN** operators park the environment for short-term cost control
- **THEN** workloads can be scaled down and public LoadBalancer Services can be removed while preserving Terraform state and persistent volumes needed for restoration
