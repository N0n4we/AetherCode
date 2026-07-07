# 2026-07-07 关闭 TKE 公网 API 与清理 CLB 裸 IP 入口

## 结论

1. 关闭 TKE 集群 `cls-26zqizrl` 的公网 Kubernetes API endpoint。
2. 删除业务 CLB `lb-r3fxnxal` 的裸 IP 入口 `TCP/80`、`TCP/8080`，只保留 `HTTPS/443` 域名访问。

## 变更对象

|对象|资源|处理|
|---|---|---|
|TKE 公网 API endpoint|`cls-26zqizrl`|关闭公网访问|
|TKE API 公网 CLB|`lb-4qsbs3wt`|已删除|
|TKE API endpoint 安全组|`sg-jqzzj85e`|从 Terraform 移除|
|业务 CLB|`lb-r3fxnxal`|保留|
|业务 CLB listener|`TCP/80 openwebui-http`|已删除|
|业务 CLB listener|`TCP/8080 relay-http`|已删除|
|业务 CLB listener|`HTTPS/443 aether-app-https`|保留|

## 代码变更

- 删除 `platform/terraform/tencentcloud/tke_endpoint.tf`。
- 删除 `variables.tf` 中的 `kube_api_allowed_cidrs`。
- 删除 `outputs.tf` 中的 `tke_public_endpoint`。
- 从 `openwebui_public.tf` 删除 `80` / `8080` TCP listener 和 attachment。
- 更新 `platform/terraform/tencentcloud/README.md`、`platform/k8s/tke/README.md` 的入口说明。
- `.gitignore` 增加 `.resource-control/`。

## 执行记录

### 关闭 TKE 公网 API

执行：

```sh
tccli tke DeleteClusterEndpoint \
  --region ap-shanghai \
  --ClusterId cls-26zqizrl \
  --IsExtranet true
```

随后用 Terraform 清理 endpoint 相关资源和 state：

```sh
terraform apply -input=false -auto-approve \
  -target=tencentcloud_kubernetes_cluster_endpoint.aether_public \
  -target=tencentcloud_security_group_rule_set.kube_api_endpoint \
  -target=tencentcloud_security_group.kube_api_endpoint

terraform apply -refresh-only -input=false -auto-approve
```

结果：

```text
Resources: 0 added, 0 changed, 3 destroyed.
```

### 删除裸 IP 入口

执行：

```sh
terraform apply -input=false -auto-approve
```

结果：

```text
Plan: 0 to add, 0 to change, 4 to destroy.
Resources: 0 added, 0 changed, 4 destroyed.
```

删除的 4 个资源：

- `tencentcloud_clb_attachment.openwebui_nodes[0]`
- `tencentcloud_clb_attachment.relay_nodes[0]`
- `tencentcloud_clb_listener.openwebui_http`
- `tencentcloud_clb_listener.relay_http`

本地执行快照：

- `~/.tccli/resource-control/20260707-193025/`
- `~/.tccli/resource-control/20260707-193928/`

## 验证

### TKE 公网 API 已关闭

```text
DescribeClusterSecurity:
ClusterExternalEndpoint=""
SecurityPolicy=null
```

```text
DescribeLoadBalancers lb-4qsbs3wt:
TotalCount=0
```

### 业务 CLB 只剩 HTTPS listener

```text
DescribeListeners lb-r3fxnxal:
TotalCount=1
lbl-hp2uxr3f HTTPS 443 aether-app-https
```

### HTTPS 业务入口正常

```text
https://relay.n0n4w3.cn/healthz    -> 200
https://openwebui.n0n4w3.cn/health -> 200
```

### 裸 IP 入口已下线

```text
http://110.40.156.154/health       -> 000 / Empty reply
http://110.40.156.154:8080/healthz -> 000 / Empty reply
```

### Terraform 已收敛

```text
terraform plan -input=false -detailed-exitcode
No changes. Your infrastructure matches the configuration.
```

## 剩余注意事项

- 公网 `kubectl` 不再可用；后续运维需要内网入口、VPN、跳板机或临时开启 endpoint。
- `tencentcloud_eip.app_public.applicable_for_clb` 仍有 provider deprecated warning，非本次变更引入。
- NAT、节点数、TKE 规格本次未调整。
