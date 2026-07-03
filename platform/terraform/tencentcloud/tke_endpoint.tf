resource "tencentcloud_security_group" "kube_api_endpoint" {
  name        = "aether-tke-api-endpoint"
  description = "Restrict public TKE Kubernetes API endpoint access."
  project_id  = 0

  tags = {}
}

resource "tencentcloud_security_group_rule_set" "kube_api_endpoint" {
  security_group_id = tencentcloud_security_group.kube_api_endpoint.id

  egress {
    action      = "ACCEPT"
    cidr_block  = "0.0.0.0/0"
    port        = "ALL"
    protocol    = "ALL"
    description = "Allow outbound from endpoint load balancer"
  }

  dynamic "ingress" {
    for_each = toset(var.kube_api_allowed_cidrs)

    content {
      action      = "ACCEPT"
      cidr_block  = ingress.value
      port        = "443"
      protocol    = "TCP"
      description = "Allow kubectl access from approved CIDR"
    }
  }
}

resource "tencentcloud_kubernetes_cluster_endpoint" "aether_public" {
  cluster_id                      = tencentcloud_kubernetes_cluster.aether.id
  cluster_internet                = true
  cluster_internet_security_group = tencentcloud_security_group.kube_api_endpoint.id
  cluster_intranet                = false

  extensive_parameters = jsonencode({
    InternetAccessible = {
      InternetChargeType      = "TRAFFIC_POSTPAID_BY_HOUR"
      InternetMaxBandwidthOut = 1
    }
  })
}
