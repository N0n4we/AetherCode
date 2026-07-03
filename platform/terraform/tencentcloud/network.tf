resource "tencentcloud_vpc" "aether" {
  name       = "aether"
  cidr_block = "10.0.0.0/16"

  is_multicast                  = false
  enable_route_vpc_publish      = false
  enable_route_vpc_publish_ipv6 = false

  tags = {}
}

resource "tencentcloud_subnet" "aether_pods" {
  name              = "aether"
  vpc_id            = tencentcloud_vpc.aether.id
  cidr_block        = "10.0.0.0/24"
  availability_zone = "ap-shanghai-4"
  route_table_id    = tencentcloud_vpc.aether.default_route_table_id
  is_multicast      = false

  tags = {}
}

resource "tencentcloud_subnet" "aether_pods_az5" {
  name              = "aether-pods-az5"
  vpc_id            = tencentcloud_vpc.aether.id
  cidr_block        = "10.0.2.0/24"
  availability_zone = "ap-shanghai-5"
  route_table_id    = tencentcloud_vpc.aether.default_route_table_id
  is_multicast      = false

  tags = {}
}

resource "tencentcloud_subnet" "aether_nodes" {
  name              = "aether5"
  vpc_id            = tencentcloud_vpc.aether.id
  cidr_block        = "10.0.1.0/24"
  availability_zone = "ap-shanghai-5"
  route_table_id    = tencentcloud_vpc.aether.default_route_table_id
  is_multicast      = false

  tags = {}
}

resource "tencentcloud_security_group" "default" {
  name        = "default"
  description = "System default security group"
  project_id  = 0

  tags = {}
}

resource "tencentcloud_security_group_rule_set" "default" {
  security_group_id = tencentcloud_security_group.default.id

  egress {
    action      = "ACCEPT"
    cidr_block  = "0.0.0.0/0"
    port        = "ALL"
    protocol    = "ALL"
    description = "Default rule"
  }

  egress {
    action          = "ACCEPT"
    ipv6_cidr_block = "::/0"
    port            = "ALL"
    protocol        = "ALL"
    description     = "Default rule"
  }

  ingress {
    action      = "ACCEPT"
    cidr_block  = "0.0.0.0/0"
    port        = "ALL"
    protocol    = "ALL"
    description = "Default rule"
  }

  ingress {
    action          = "DROP"
    ipv6_cidr_block = "::/0"
    port            = "ALL"
    protocol        = "ALL"
    description     = "Default rule"
  }
}
