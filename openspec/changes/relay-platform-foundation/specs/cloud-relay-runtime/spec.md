## ADDED Requirements

### Requirement: Terraform Managed Cloud Runtime
The platform SHALL define Terraform-managed cloud resources for the relay runtime, including Kubernetes, relational database, networking, and ingress or Gateway API prerequisites.

#### Scenario: Runtime resources are provisioned
- **WHEN** Terraform is applied for a supported environment
- **THEN** the environment has a Kubernetes cluster, relational database, network access, and ingress or Gateway API prerequisites for the relay platform

#### Scenario: Runtime outputs support deployment
- **WHEN** Terraform completes successfully
- **THEN** it exposes the non-secret connection references required to deploy Kubernetes workloads and connect them to the database

### Requirement: Best Effort Kubernetes Cluster
The platform SHALL run on a best-effort Kubernetes cluster while still defining health, readiness, resource, and rollout controls for service workloads.

#### Scenario: Workloads declare health behavior
- **WHEN** relay and account workloads are deployed
- **THEN** each workload has liveness and readiness checks suitable for Kubernetes rollout and traffic gating

#### Scenario: Best effort availability is explicit
- **WHEN** operators inspect the runtime configuration
- **THEN** the configuration identifies the cluster as best-effort and does not imply a strict high-availability SLO

### Requirement: Relay And Account Service Deployment
The Kubernetes runtime SHALL deploy the relay service and account service as distinct workloads with independently configurable images, replicas, environment, and resource settings.

#### Scenario: Relay service is deployed
- **WHEN** the relay deployment is applied
- **THEN** Kubernetes creates a relay workload and service that can receive configured relay API traffic

#### Scenario: Account service is deployed
- **WHEN** the account deployment is applied
- **THEN** Kubernetes creates an account workload and service that can receive configured account API traffic

### Requirement: Frontend Gateway Exposure
The platform SHALL expose frontend-facing relay and account APIs through ingress or Gateway API resources.

#### Scenario: Relay routes reach relay service
- **WHEN** a frontend request targets configured relay API paths such as `/v1` or `/v1beta`
- **THEN** ingress or Gateway API routes the request to the relay service

#### Scenario: Account routes reach account service
- **WHEN** a frontend request targets the configured account API host or path prefix
- **THEN** ingress or Gateway API routes the request to the account service

### Requirement: Runtime Secret Handling
The runtime MUST inject database credentials, upstream provider credentials, and signing or hashing secrets through secret references rather than plain manifest values.

#### Scenario: Secret values are not plain manifests
- **WHEN** Kubernetes deployment artifacts are rendered
- **THEN** secret values are referenced through Kubernetes secrets or an external secret mechanism and are not embedded as raw values in workload manifests

#### Scenario: Non-secret configuration is separate
- **WHEN** workload configuration is rendered
- **THEN** non-secret settings can be inspected separately from secret references
