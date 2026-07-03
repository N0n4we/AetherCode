resource "tencentcloud_kubernetes_cluster" "aether" {
  cluster_name        = "aether"
  cluster_desc        = ""
  cluster_deploy_type = "MANAGED_CLUSTER"
  cluster_version     = "1.34.1"
  cluster_os          = "tlinux4_x86_64_public"
  cluster_os_type     = "GENERAL"

  container_runtime = "containerd"
  runtime_version   = "1.7.28"

  cluster_level              = "L5"
  auto_upgrade_cluster_level = false
  deletion_protection        = true
  project_id                 = 0

  vpc_id       = tencentcloud_vpc.aether.id
  network_type = "VPC-CNI"
  vpc_cni_type = "tke-route-eni"
  eni_subnet_ids = [
    tencentcloud_subnet.aether_pods.id,
    tencentcloud_subnet.aether_pods_az5.id,
  ]
  service_cidr            = "192.168.0.0/17"
  cluster_ipvs            = false
  data_plane_v2           = false
  is_dual_stack           = false
  node_name_type          = "lan-ip"
  kube_proxy_mode         = ""
  cluster_max_pod_num     = 64
  cluster_max_service_num = 32768

  ignore_cluster_cidr_conflict = false
  ignore_service_cidr_conflict = false
  is_non_static_ip_mode        = true

  tags = {}
}
