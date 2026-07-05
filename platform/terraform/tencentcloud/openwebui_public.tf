locals {
  relay_node_port     = 31326
  openwebui_node_port = 31327

  openwebui_certificate_id = "YzBrwfL6"
  relay_certificate_id     = "YzBrtM79"

  aether_node_instance_ids = toset([
    for instance in data.tencentcloud_as_instances.aether_node_pool.instance_list : instance.instance_id
    if contains(["IN_SERVICE", "InService", "InService"], instance.life_cycle_state)
  ])
}

data "tencentcloud_as_instances" "aether_node_pool" {
  filters {
    name   = "auto-scaling-group-id"
    values = [tencentcloud_kubernetes_node_pool.aether_zero.auto_scaling_group_id]
  }
}

resource "tencentcloud_eip" "app_public" {
  name                       = "aether-app-public"
  type                       = "EIP"
  applicable_for_clb         = true
  internet_charge_type       = "TRAFFIC_POSTPAID_BY_HOUR"
  internet_max_bandwidth_out = 1

  tags = {
    service    = "aether-apps"
    managed_by = "terraform"
  }
}

resource "tencentcloud_clb_instance" "app_public" {
  clb_name       = "aether-app-public"
  network_type   = "INTERNAL"
  vpc_id         = tencentcloud_vpc.aether.id
  subnet_id      = tencentcloud_subnet.aether_nodes.id
  eip_address_id = tencentcloud_eip.app_public.id

  tags = {
    service    = "aether-apps"
    managed_by = "terraform"
  }
}

resource "tencentcloud_clb_listener" "openwebui_http" {
  clb_id        = tencentcloud_clb_instance.app_public.id
  listener_name = "openwebui-http"
  protocol      = "TCP"
  port          = 80
  target_type   = "NODE"
  scheduler     = "WRR"

  health_check_switch        = true
  health_check_type          = "TCP"
  health_check_port          = local.openwebui_node_port
  health_check_time_out      = 2
  health_check_interval_time = 5
  health_check_health_num    = 3
  health_check_unhealth_num  = 3
}

resource "tencentcloud_clb_attachment" "openwebui_nodes" {
  count = length(local.aether_node_instance_ids) > 0 ? 1 : 0

  clb_id      = tencentcloud_clb_instance.app_public.id
  listener_id = tencentcloud_clb_listener.openwebui_http.listener_id

  dynamic "targets" {
    for_each = local.aether_node_instance_ids

    content {
      instance_id = targets.value
      port        = local.openwebui_node_port
      weight      = 10
    }
  }
}

resource "tencentcloud_clb_listener" "relay_http" {
  clb_id        = tencentcloud_clb_instance.app_public.id
  listener_name = "relay-http"
  protocol      = "TCP"
  port          = 8080
  target_type   = "NODE"
  scheduler     = "WRR"

  health_check_switch        = true
  health_check_type          = "TCP"
  health_check_port          = local.relay_node_port
  health_check_time_out      = 2
  health_check_interval_time = 5
  health_check_health_num    = 3
  health_check_unhealth_num  = 3
}

resource "tencentcloud_clb_attachment" "relay_nodes" {
  count = length(local.aether_node_instance_ids) > 0 ? 1 : 0

  clb_id      = tencentcloud_clb_instance.app_public.id
  listener_id = tencentcloud_clb_listener.relay_http.listener_id

  dynamic "targets" {
    for_each = local.aether_node_instance_ids

    content {
      instance_id = targets.value
      port        = local.relay_node_port
      weight      = 10
    }
  }
}

resource "tencentcloud_clb_listener" "app_https" {
  clb_id               = tencentcloud_clb_instance.app_public.id
  listener_name        = "aether-app-https"
  protocol             = "HTTPS"
  port                 = 443
  certificate_ssl_mode = "UNIDIRECTIONAL"
  certificate_id       = local.openwebui_certificate_id
  sni_switch           = true
}

resource "tencentcloud_clb_listener_rule" "openwebui_https" {
  clb_id               = tencentcloud_clb_instance.app_public.id
  listener_id          = tencentcloud_clb_listener.app_https.listener_id
  domain               = "openwebui.n0n4w3.cn"
  url                  = "/"
  target_type          = "NODE"
  scheduler            = "WRR"
  certificate_ssl_mode = "UNIDIRECTIONAL"
  certificate_id       = local.openwebui_certificate_id

  health_check_switch        = true
  health_check_type          = "HTTP"
  health_check_http_method   = "GET"
  health_check_http_path     = "/health"
  health_check_http_code     = 31
  health_check_interval_time = 5
  health_check_time_out      = 2
  health_check_health_num    = 3
  health_check_unhealth_num  = 3
}

resource "tencentcloud_clb_attachment" "openwebui_https_nodes" {
  count = length(local.aether_node_instance_ids) > 0 ? 1 : 0

  clb_id      = tencentcloud_clb_instance.app_public.id
  listener_id = tencentcloud_clb_listener.app_https.listener_id
  rule_id     = tencentcloud_clb_listener_rule.openwebui_https.rule_id

  dynamic "targets" {
    for_each = local.aether_node_instance_ids

    content {
      instance_id = targets.value
      port        = local.openwebui_node_port
      weight      = 10
    }
  }
}

resource "tencentcloud_clb_listener_rule" "relay_https" {
  clb_id               = tencentcloud_clb_instance.app_public.id
  listener_id          = tencentcloud_clb_listener.app_https.listener_id
  domain               = "relay.n0n4w3.cn"
  url                  = "/"
  target_type          = "NODE"
  scheduler            = "WRR"
  certificate_ssl_mode = "UNIDIRECTIONAL"
  certificate_id       = local.relay_certificate_id

  health_check_switch        = true
  health_check_type          = "HTTP"
  health_check_http_method   = "GET"
  health_check_http_path     = "/healthz"
  health_check_http_code     = 31
  health_check_interval_time = 5
  health_check_time_out      = 2
  health_check_health_num    = 3
  health_check_unhealth_num  = 3
}

resource "tencentcloud_clb_attachment" "relay_https_nodes" {
  count = length(local.aether_node_instance_ids) > 0 ? 1 : 0

  clb_id      = tencentcloud_clb_instance.app_public.id
  listener_id = tencentcloud_clb_listener.app_https.listener_id
  rule_id     = tencentcloud_clb_listener_rule.relay_https.rule_id

  dynamic "targets" {
    for_each = local.aether_node_instance_ids

    content {
      instance_id = targets.value
      port        = local.relay_node_port
      weight      = 10
    }
  }
}
