# GCP Dev Relay Platform

This environment is the first supported cloud target for `relay-platform-foundation`:
GKE Autopilot plus Cloud SQL for PostgreSQL in one Google Cloud region.

The cluster is explicitly best-effort. It uses managed services, readiness gates,
and backups, but it does not promise multi-region or zero-downtime failover.

## Apply

```bash
terraform init
terraform apply \
  -var='project_id=YOUR_PROJECT' \
  -var='api_key_hash_secret=CHANGE_ME' \
  -var='account_service_key=CHANGE_ME'
```

After apply, consume non-secret outputs for Kubernetes config and store sensitive
outputs in the runtime secret backend before applying manifests:

```bash
terraform output cluster_name
terraform output cluster_location
terraform output database_connection_name
terraform output database_private_ip_address
terraform output -raw database_password
```

Do not commit Terraform state. It contains sensitive generated values even when
outputs are marked `sensitive`.
