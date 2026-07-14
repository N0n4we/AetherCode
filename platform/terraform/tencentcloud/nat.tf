# NAT gateway 已移除。
#
# 历史上 VPC 默认路由 0.0.0.0/0 指向 NAT 网关（nat-6hvu0buf）作为集群
# 出公网通道。为降低固定实例费（小型 NAT ¥0.5/小时 ≈ ¥360/月，与带宽无关），
# 改为给每个 worker 节点绑定流量计费 EIP（见 node_egress.tf）。VPC-CNI
# (tke-route-eni) 下 ip-masq-agent 将 Pod 出公网流量 SNAT 到节点主网卡 IP，
# 再经节点 EIP 出公网。实际出公网流量极小（relay 调 LLM API 的 payload），
# 按流量计费几乎为零。
#
# 回滚方式：恢复本文件中的 tencentcloud_eip.aether_nat、
# tencentcloud_nat_gateway.aether、tencentcloud_route_table_entry.default_internet_via_nat
# 三个资源定义，重新 `terraform apply`。