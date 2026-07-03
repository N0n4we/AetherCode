resource "tencentcloud_key_pair" "aether_node_pool" {
  key_name   = "aether_tf_node_pool"
  public_key = file("${path.module}/aether_tf_node_pool.pub")
}

resource "tencentcloud_cam_role" "auto_scaling" {
  name        = "AS_QCSRole"
  description = "The current role is the AS service role, which will access your other service resources within the scope of the permissions of the associated policy."

  document = jsonencode({
    version = "2.0"
    statement = [
      {
        effect = "allow"
        action = "sts:AssumeRole"
        principal = {
          service = "as.cloud.tencent.com"
        }
      }
    ]
  })
}

resource "tencentcloud_cam_role_policy_attachment_by_name" "auto_scaling_access" {
  role_name   = tencentcloud_cam_role.auto_scaling.name
  policy_name = "QcloudAccessForASRole"
}

resource "tencentcloud_cam_role_policy_attachment_by_name" "auto_scaling_notification" {
  role_name   = tencentcloud_cam_role.auto_scaling.name
  policy_name = "QcloudAccessForASRoleInNotification"
}

resource "tencentcloud_kubernetes_node_pool" "aether_zero" {
  name              = "aether-tf-zero"
  cluster_id        = tencentcloud_kubernetes_cluster.aether.id
  vpc_id            = tencentcloud_vpc.aether.id
  subnet_ids        = [tencentcloud_subnet.aether_nodes.id]
  min_size          = 0
  max_size          = var.node_pool_max_size
  enable_auto_scale = true

  delete_keep_instance = true
  node_os              = "tlinux4_x86_64_public"
  node_os_type         = "GENERAL"
  termination_policies = ["OLDEST_INSTANCE"]

  labels = {
    managed_by = "terraform"
  }

  auto_scaling_config {
    backup_instance_types      = []
    instance_type              = var.node_pool_instance_type
    instance_charge_type       = "POSTPAID_BY_HOUR"
    public_ip_assigned         = false
    internet_max_bandwidth_out = 0
    system_disk_type           = "CLOUD_PREMIUM"
    system_disk_size           = 50
    enhanced_monitor_service   = true
    enhanced_security_service  = true
    key_ids                    = [tencentcloud_key_pair.aether_node_pool.id]
    orderly_security_group_ids = [tencentcloud_security_group.default.id]
  }

  node_config {
    desired_pod_num   = 64
    docker_graph_path = "/var/lib/docker"
    extra_args        = []
    is_schedule       = true
  }

  depends_on = [
    tencentcloud_cam_role_policy_attachment_by_name.auto_scaling_access,
    tencentcloud_cam_role_policy_attachment_by_name.auto_scaling_notification,
    tencentcloud_security_group_rule_set.default,
  ]
}
