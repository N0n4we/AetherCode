# Node egress EIPs.
#
# Each worker node gets a traffic-billed EIP so Pods can reach the public
# internet (upstream LLM APIs) without a NAT gateway. In VPC-CNI
# (tke-route-eni) mode, ip-masq-agent SNATs Pod egress to the node's primary
# private IP, which egresses via the bound EIP. Traffic billing means cost is
# driven by actual outbound bytes (near zero for relay API calls), not by the
# bandwidth cap, so the cap is set for throughput headroom only.
#
# These EIPs bind to the instances currently returned by the node-pool ASG
# data source. If the node pool replaces an instance (ASG recreation), run
# `terraform apply` so the data source refreshes and the EIP re-associates to
# the new instance; until then a freshly created node has no egress EIP.
resource "tencentcloud_eip" "node_egress" {
  for_each = local.aether_node_instance_ids

  name                       = "aether-node-${replace(each.key, "ins-", "")}"
  type                       = "EIP"
  internet_charge_type       = "TRAFFIC_POSTPAID_BY_HOUR"
  internet_max_bandwidth_out = 10

  tags = {
    service    = "aether-node-egress"
    managed_by = "terraform"
  }
}

resource "tencentcloud_eip_association" "node_egress" {
  for_each = local.aether_node_instance_ids

  eip_id      = tencentcloud_eip.node_egress[each.key].id
  instance_id = each.key
}