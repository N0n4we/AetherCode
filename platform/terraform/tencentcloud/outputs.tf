output "vpc_id" {
  value = tencentcloud_vpc.aether.id
}

output "pod_subnet_id" {
  value = tencentcloud_subnet.aether_pods.id
}

output "pod_subnet_az5_id" {
  value = tencentcloud_subnet.aether_pods_az5.id
}

output "node_subnet_id" {
  value = tencentcloud_subnet.aether_nodes.id
}

output "node_security_group_id" {
  value = tencentcloud_security_group.default.id
}

output "nat_gateway_id" {
  value = tencentcloud_nat_gateway.aether.id
}

output "tke_cluster_id" {
  value = tencentcloud_kubernetes_cluster.aether.id
}

output "tke_node_pool_id" {
  value = tencentcloud_kubernetes_node_pool.aether_zero.id
}

output "tke_public_endpoint" {
  value = tencentcloud_kubernetes_cluster_endpoint.aether_public.cluster_external_endpoint
}

output "node_pool_auto_scaling_group_id" {
  value = tencentcloud_kubernetes_node_pool.aether_zero.auto_scaling_group_id
}
