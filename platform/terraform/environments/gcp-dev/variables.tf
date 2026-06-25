variable "project_id" {
  description = "Google Cloud project that owns the relay platform resources."
  type        = string
}

variable "region" {
  description = "Google Cloud region for the best-effort runtime."
  type        = string
  default     = "us-central1"
}

variable "environment" {
  description = "Environment name used in resource names and labels."
  type        = string
  default     = "dev"
}

variable "network_cidr" {
  description = "Primary subnet CIDR for GKE nodes and private service access."
  type        = string
  default     = "10.42.0.0/20"
}

variable "database_name" {
  description = "Application database name."
  type        = string
  default     = "relay"
}

variable "database_user" {
  description = "Application database user."
  type        = string
  default     = "relay"
}

variable "database_password" {
  description = "Application database password. Leave empty to generate one and consume the sensitive output."
  type        = string
  default     = ""
  sensitive   = true
}

variable "database_tier" {
  description = "Cloud SQL tier for the best-effort PostgreSQL instance."
  type        = string
  default     = "db-f1-micro"
}

variable "api_key_hash_secret" {
  description = "HMAC secret used by the account service and relay for API key verifier hashes."
  type        = string
  sensitive   = true
}

variable "account_service_key" {
  description = "Bearer token accepted by the account service frontend/auth boundary in the initial platform mode."
  type        = string
  sensitive   = true
}
