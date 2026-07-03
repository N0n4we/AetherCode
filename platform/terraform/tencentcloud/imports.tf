import {
  to = tencentcloud_vpc.aether
  id = "vpc-kdegvs6i"
}

import {
  to = tencentcloud_subnet.aether_pods
  id = "subnet-iwqt8au5"
}

import {
  to = tencentcloud_subnet.aether_nodes
  id = "subnet-6siua6l3"
}

import {
  to = tencentcloud_security_group.default
  id = "sg-bvlotzok"
}

import {
  to = tencentcloud_security_group_rule_set.default
  id = "sg-bvlotzok"
}

import {
  to = tencentcloud_kubernetes_cluster.aether
  id = "cls-26zqizrl"
}

import {
  to = tencentcloud_key_pair.aether_node_pool
  id = "skey-ji9fl1td"
}

import {
  to = tencentcloud_cam_role.auto_scaling
  id = "4611686018447633701"
}

import {
  to = tencentcloud_cam_role_policy_attachment_by_name.auto_scaling_access
  id = "AS_QCSRole#QcloudAccessForASRole"
}

import {
  to = tencentcloud_cam_role_policy_attachment_by_name.auto_scaling_notification
  id = "AS_QCSRole#QcloudAccessForASRoleInNotification"
}

import {
  to = tencentcloud_kubernetes_node_pool.aether_zero
  id = "cls-26zqizrl#np-8ldph9uj"
}
