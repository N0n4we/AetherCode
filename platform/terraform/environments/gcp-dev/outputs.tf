output "environment" {
  description = "Environment name."
  value       = var.environment
}

output "cluster_name" {
  description = "GKE cluster name for kubectl credentials."
  value       = google_container_cluster.relay.name
}

output "cluster_location" {
  description = "GKE cluster location."
  value       = google_container_cluster.relay.location
}

output "network_name" {
  description = "VPC network name."
  value       = google_compute_network.platform.name
}

output "subnetwork_name" {
  description = "VPC subnetwork name."
  value       = google_compute_subnetwork.platform.name
}

output "database_connection_name" {
  description = "Cloud SQL instance connection name."
  value       = google_sql_database_instance.relay.connection_name
}

output "database_private_ip_address" {
  description = "Private IP address for the Cloud SQL instance."
  value       = google_sql_database_instance.relay.private_ip_address
}

output "database_name" {
  description = "Application database name."
  value       = google_sql_database.relay.name
}

output "database_user" {
  description = "Application database user."
  value       = google_sql_user.relay.name
}

output "ingress_ip_name" {
  description = "Reserved global address resource name for ingress or Gateway API."
  value       = google_compute_global_address.ingress.name
}

output "ingress_ip_address" {
  description = "Reserved global address for frontend traffic."
  value       = google_compute_global_address.ingress.address
}

output "relay_service_account_email" {
  description = "Google service account email for the relay workload."
  value       = google_service_account.relay.email
}

output "account_service_account_email" {
  description = "Google service account email for the account workload."
  value       = google_service_account.account.email
}

output "database_password" {
  description = "Sensitive database password. Store this in the runtime secret backend before deploying Kubernetes workloads."
  value       = local.db_password
  sensitive   = true
}

output "api_key_hash_secret" {
  description = "Sensitive API key hash secret value for relay/account workloads."
  value       = var.api_key_hash_secret
  sensitive   = true
}

output "account_service_key" {
  description = "Sensitive account service bearer token."
  value       = var.account_service_key
  sensitive   = true
}
