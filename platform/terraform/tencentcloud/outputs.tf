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

output "tke_cluster_id" {
  value = tencentcloud_kubernetes_cluster.aether.id
}

output "tke_node_pool_id" {
  value = tencentcloud_kubernetes_node_pool.aether_zero.id
}

output "node_pool_auto_scaling_group_id" {
  value = tencentcloud_kubernetes_node_pool.aether_zero.auto_scaling_group_id
}

output "app_public_clb_id" {
  value = tencentcloud_clb_instance.app_public.id
}

output "app_public_ip" {
  value = tencentcloud_eip.app_public.public_ip
}

output "openwebui_public_url" {
  value = "https://openwebui.n0n4w3.cn"
}

output "relay_public_base_url" {
  value = "https://relay.n0n4w3.cn/v1"
}
