provider "google" {
  project = var.project_id
  region  = var.region
}

locals {
  name        = "aether-relay-${var.environment}"
  labels      = { app = "aether-relay", environment = var.environment, availability = "best-effort" }
  db_password = var.database_password != "" ? var.database_password : random_password.database_password.result
}

resource "google_project_service" "required" {
  for_each = toset([
    "compute.googleapis.com",
    "container.googleapis.com",
    "servicenetworking.googleapis.com",
    "sqladmin.googleapis.com",
    "secretmanager.googleapis.com"
  ])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "random_password" "database_password" {
  length  = 32
  special = true
}

resource "google_compute_network" "platform" {
  name                    = "${local.name}-network"
  auto_create_subnetworks = false

  depends_on = [google_project_service.required]
}

resource "google_compute_subnetwork" "platform" {
  name                     = "${local.name}-subnet"
  ip_cidr_range            = var.network_cidr
  region                   = var.region
  network                  = google_compute_network.platform.id
  private_ip_google_access = true
}

resource "google_compute_global_address" "private_services" {
  name          = "${local.name}-private-services"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.platform.id
}

resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = google_compute_network.platform.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_services.name]
}

resource "google_container_cluster" "relay" {
  name                = "${local.name}-gke"
  location            = var.region
  enable_autopilot    = true
  deletion_protection = false
  network             = google_compute_network.platform.id
  subnetwork          = google_compute_subnetwork.platform.id

  ip_allocation_policy {}

  gateway_api_config {
    channel = "CHANNEL_STANDARD"
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  release_channel {
    channel = "REGULAR"
  }

  resource_labels = local.labels

  depends_on = [google_project_service.required]
}

resource "google_sql_database_instance" "relay" {
  name                = "${local.name}-postgres"
  database_version    = "POSTGRES_16"
  region              = var.region
  deletion_protection = false

  depends_on = [
    google_project_service.required,
    google_service_networking_connection.private_vpc_connection
  ]

  settings {
    tier              = var.database_tier
    availability_type = "ZONAL"
    disk_type         = "PD_SSD"
    disk_size         = 20
    user_labels       = local.labels

    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = true
    }

    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.platform.id
      ssl_mode        = "ENCRYPTED_ONLY"
    }
  }
}

resource "google_sql_database" "relay" {
  name     = var.database_name
  instance = google_sql_database_instance.relay.name
}

resource "google_sql_user" "relay" {
  name     = var.database_user
  instance = google_sql_database_instance.relay.name
  password = local.db_password
}

resource "google_compute_global_address" "ingress" {
  name = "${local.name}-ingress"
}

resource "google_service_account" "relay" {
  account_id   = "${local.name}-relay"
  display_name = "Aether relay workload identity"
}

resource "google_service_account" "account" {
  account_id   = "${local.name}-account"
  display_name = "Aether account service workload identity"
}
