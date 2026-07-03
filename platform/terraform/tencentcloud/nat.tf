resource "tencentcloud_eip" "aether_nat" {
  name                       = "aether-nat"
  type                       = "EIP"
  internet_charge_type       = "TRAFFIC_POSTPAID_BY_HOUR"
  internet_max_bandwidth_out = 20

  tags = {}
}

resource "tencentcloud_nat_gateway" "aether" {
  name             = "aether-nat"
  vpc_id           = tencentcloud_vpc.aether.id
  assigned_eip_set = [tencentcloud_eip.aether_nat.public_ip]
  bandwidth        = 20
  max_concurrent   = 1000000

  tags = {}
}

resource "tencentcloud_route_table_entry" "default_internet_via_nat" {
  route_table_id         = tencentcloud_vpc.aether.default_route_table_id
  destination_cidr_block = "0.0.0.0/0"
  next_type              = "NAT"
  next_hub               = tencentcloud_nat_gateway.aether.id
  description            = "Default internet egress for private TKE nodes."
}
