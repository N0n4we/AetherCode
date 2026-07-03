# AetherCode 腾讯云运维手册

本运维手册说明如何在腾讯云 TKE 上重建、更新、验证和运维当前的
AetherCode 部署。

## 适用范围

基础设施即代码（IaC）位于：

- Terraform：`platform/terraform/tencentcloud`
- Kubernetes：`platform/k8s/tke`
- Router 应用源码及镜像构建输入：`router`

Terraform 负责管理腾讯云基础设施。Kubernetes 清单负责管理 TKE 内部的
运行时资源。应用数据、生成的 API 密钥以及原始 Secret 均有意不存储在
本仓库中。

当前公网端点：

- Relay：`http://lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com`
- OpenAI 兼容 Base URL：
  `http://lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com/v1`
- 账户服务：
  `http://lb-l5vk4kbl-i1jarzn7ckm3sf2o.clb.sh-tencentclb.com`

目前由 Terraform 管理的腾讯云资源包括 VPC、子网、NAT、EIP、默认路由、
TKE 集群、节点池、Kubernetes 公网访问端点、安全组、CAM 自动伸缩角色
关联以及节点池 SSH 密钥。

已知的不由 Terraform 管理的资源：

- TCR 个人版仓库
  `ccr.ccs.tencentyun.com/aethercode-100034871923/router`。腾讯云
  Terraform Provider 目前覆盖的是 TCR 企业版，因此这里只记录个人版
  仓库，但不使用 Terraform 管理它。
- `relay-public` 和 `account-public` 后面的 CLB。它们由 TKE 云控制器根据
  Kubernetes 的 `Service type=LoadBalancer` 自动创建。
- Kubernetes 工作负载、Service、Secret、ConfigMap、Job 和 CronJob。
  这些资源由 `platform/k8s/tke` 管理。

## 前置条件

本地工具：

- 已安装 `tenv`，并且能够使用 Terraform
- 已安装 `tccli`，并使用默认配置完成身份认证
- `kubectl`
- `docker`
- `jq`
- `python3`

腾讯云环境假设：

- 地域：`ap-shanghai`
- TKE 集群：`cls-26zqizrl`
- 用于故障兜底操作、支持 TAT 的节点：`ins-3k2t5zjf`
- 本地 Terraform 凭证来自 `~/.tccli`

不要提交以下内容：

- `platform/k8s/tke/secrets.env`
- 生成的 Relay API 密钥
- OpenRouter 密钥
- kubeconfig 文件
- 任何 TCR 登录凭证

## 1. 检查 Terraform 配置漂移

在仓库根目录执行：

```sh
cd /Users/noname/Desktop/Projects/AetherCode/platform/terraform/tencentcloud

TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform init

TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform plan -input=false -detailed-exitcode
```

退出码说明：

- 退出码 `0`：Terraform 管理的基础设施与腾讯云中的实际状态一致。
- 退出码 `2`：Terraform 检测到了变更。应用前必须仔细检查。
- 退出码 `1`：Terraform 或 Provider 出错。

列出受管理的资源：

```sh
terraform state list
```

当前预期的无变更结果为：

```text
No changes. Your infrastructure matches the configuration.
```

## 2. 应用 Terraform 基础设施配置

仅在检查完 Plan 后执行 Apply：

```sh
cd /Users/noname/Desktop/Projects/AetherCode/platform/terraform/tencentcloud

TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform apply -input=false
```

查看有用的输出：

```sh
terraform output
```

重要变量：

- `kube_api_allowed_cidrs`：允许访问 Kubernetes 公网 API 的 CIDR。
  当前默认值为 `188.253.117.150/32`。
- `node_pool_max_size`：TKE 节点池自动伸缩的最大节点数。当前默认值为
  `2`；仅在有意暂停环境容量时才将其设为 `0`。

## 3. 构建并推送 Router 镜像

Dockerfile 要求构建上下文同时包含 `AetherCode` 和同级的 `gollm`
代码仓库。请从 `/Users/noname/Desktop/Projects` 开始构建：

```sh
cd /Users/noname/Desktop/Projects

TAG="$(date +%Y%m%d-%H%M%S)"
IMAGE="ccr.ccs.tencentyun.com/aethercode-100034871923/router:${TAG}"

docker build -f AetherCode/router/Dockerfile -t "$IMAGE" .
docker push "$IMAGE"
```

随后更新 `platform/k8s/tke/kustomization.yaml`：

```yaml
images:
  - name: ghcr.io/aethercode/router
    newName: ccr.ccs.tencentyun.com/aethercode-100034871923/router
    newTag: <TAG>
```

当前部署的镜像：

```text
ccr.ccs.tencentyun.com/aethercode-100034871923/router:20260703-183944
```

## 4. 准备 Kubernetes Secret

创建本地 Secret 配置文件：

```sh
cd /Users/noname/Desktop/Projects/AetherCode
cp platform/k8s/tke/secrets.env.example platform/k8s/tke/secrets.env
```

填写以下值：

```text
sql-dsn=postgres://router:<url-encoded-postgres-password>@postgres.aether-relay.svc.cluster.local:5432/router?sslmode=disable
postgres-password=<raw-postgres-password>
router-api-key=<optional-static-public-relay-key>
router-admin-key=<admin-key-for-internal-provider-apis>
api-key-hash-secret=<random-hmac-secret-for-relay-api-keys>
account-service-key=<bearer-key-for-account-service>
openrouter-api-key=<openrouter-key>
```

`postgres-password` 应填写原始密码。`sql-dsn` 中的同一个密码则必须进行
URL 编码，尤其是密码包含 `/`、`+`、`=`、`%`、`@` 或其他保留字符时。

URL 编码辅助示例：

```sh
python3 - <<'PY'
import getpass
import urllib.parse

print(urllib.parse.quote(getpass.getpass("Postgres password: "), safe=""))
PY
```

需要时生成随机 Secret：

```sh
openssl rand -base64 32
```

## 5. 获取 Kubernetes 访问权限

正常方式：

```sh
tccli tke DescribeClusterKubeconfig \
  --region ap-shanghai \
  --ClusterId cls-26zqizrl \
  --IsExtranet true \
  --output json | jq -r '.Kubeconfig' > /tmp/aether-kubeconfig

export KUBECONFIG=/tmp/aether-kubeconfig
kubectl get nodes
```

如果从本地访问 Kubernetes API 时出现 TLS EOF 或网络问题，可通过
腾讯云 TAT 在集群节点上运行 `kubectl`。该节点已经具有
`/root/.kube/config`：

```sh
CONTENT="$(printf '%s' 'set -eu
export KUBECONFIG=/root/.kube/config
kubectl get nodes
kubectl -n aether-relay get all -o wide
' | base64 | tr -d '\n')"

tccli tat RunCommand \
  --cli-unfold-argument \
  --region ap-shanghai \
  --CommandType SHELL \
  --CommandName aether-kubectl-check \
  --Content "$CONTENT" \
  --InstanceIds ins-3k2t5zjf \
  --Timeout 120 \
  --Username root \
  --output json
```

轮询 TAT 执行结果：

```sh
tccli tat DescribeInvocationTasks \
  --cli-unfold-argument \
  --region ap-shanghai \
  --Offset 0 \
  --Limit 10 \
  --HideOutput False \
  --Filters.0.Name invocation-id \
  --Filters.0.Values <invocation-id> \
  --output json |
jq -r '.InvocationTaskSet[] |
  "status=" + .TaskStatus + " exit=" + ((.TaskResult.ExitCode // -999)|tostring) +
  "\n" + ((.TaskResult.Output // "") | @base64d)'
```

## 6. 部署 Kubernetes 运行时资源

应用 TKE Overlay：

```sh
cd /Users/noname/Desktop/Projects/AetherCode
kubectl apply -k platform/k8s/tke
```

等待工作负载就绪：

```sh
kubectl -n aether-relay rollout status statefulset/postgres --timeout=300s
kubectl -n aether-relay rollout status deployment/relay --timeout=300s
kubectl -n aether-relay rollout status deployment/account --timeout=300s
kubectl -n aether-relay wait \
  --for=condition=complete \
  job/sync-openrouter-provider-channels-once \
  --timeout=300s
```

检查 Service：

```sh
kubectl -n aether-relay get svc relay-public account-public
```

Overlay 创建的运行时资源：

- Namespace `aether-relay`
- Postgres StatefulSet 和 Service
- Relay Deployment 和 Service
- Account Deployment 和 Service
- 腾讯云公网 LoadBalancer Service
- 运行时 ConfigMap
- 根据 `secrets.env` 生成的运行时 Secret
- OpenRouter Provider Channel 同步 ConfigMap、一次性 Job 和 CronJob

## 7. 同步 OpenRouter Provider Channel 配置漂移

`platform/k8s/tke/openrouter-seed.yaml` 现在以声明式方式对 Provider
Channel 进行协调。

受管理的 Channel：

- `poolside/laguna-xs-2.1:free`
- `cohere/north-mini-code:free`

一次性 Job 用于初始化全新部署：

```text
sync-openrouter-provider-channels-once
```

CronJob 每 15 分钟修正一次配置漂移：

```text
sync-openrouter-provider-channels
```

它会更新 Provider 名称、公开模型 ID、上游 Base URL、上游模型、
Secret 引用、认证 Header/前缀、优先级、权重、状态、端点能力、
Channel 类型、Relay 格式和计费类别等受管理字段。

它有意不会删除不受其管理的 Provider Channel。

立即执行一次手动同步：

```sh
kubectl -n aether-relay create job \
  --from=cronjob/sync-openrouter-provider-channels \
  "sync-openrouter-provider-channels-manual-$(date +%Y%m%d%H%M%S)"
```

检查同步日志：

```sh
kubectl -n aether-relay logs job/sync-openrouter-provider-channels-once
kubectl -n aether-relay get cronjob sync-openrouter-provider-channels
```

## 8. 验证服务健康状态

从 Kubernetes 获取当前公网端点：

```sh
kubectl -n aether-relay get svc relay-public account-public
```

健康检查：

```sh
RELAY_URL="http://lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com"
ACCOUNT_URL="http://lb-l5vk4kbl-i1jarzn7ckm3sf2o.clb.sh-tencentclb.com"

curl --noproxy '*' -fsS "$RELAY_URL/healthz"
curl --noproxy '*' -fsS "$ACCOUNT_URL/healthz"
```

如果本地 DNS 返回了伪造 IP，请显式指定 CLB 域名的解析结果：

```sh
curl --noproxy '*' \
  --resolve "lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com:80:1.15.197.38" \
  -fsS "$RELAY_URL/healthz"
```

检查 Relay 内部状态：

```sh
kubectl -n aether-relay port-forward svc/relay 18080:80

curl -fsS \
  -H "Authorization: Bearer $ROUTER_ADMIN_KEY" \
  http://127.0.0.1:18080/internal/status | jq .
```

预期结果：

- `in_sync: true`
- Provider 数量为 `2`
- 模型数量为 `2`

## 9. 生成 Relay API 密钥

账户服务负责创建限定到账户范围的 Relay API 密钥。调用时需要
`ACCOUNT_SERVICE_KEY` 和账户身份 Header。

```sh
ACCOUNT_URL="http://lb-l5vk4kbl-i1jarzn7ckm3sf2o.clb.sh-tencentclb.com"

curl --noproxy '*' -fsS \
  -X POST "$ACCOUNT_URL/account/api-keys" \
  -H "Authorization: Bearer $ACCOUNT_SERVICE_KEY" \
  -H "X-Aether-Account-ID: default" \
  -H "Content-Type: application/json" \
  -d '{"name":"smoke-test"}' | jq .
```

响应中的 `secret` 只会返回一次。测试时可将它保存到本地 Shell 变量中，
但不要提交到仓库：

```sh
RELAY_API_KEY="<secret-from-response>"
```

密钥不再需要时，应将其禁用或吊销：

```sh
curl --noproxy '*' -fsS \
  -X POST "$ACCOUNT_URL/account/api-keys/<id>/disable" \
  -H "Authorization: Bearer $ACCOUNT_SERVICE_KEY" \
  -H "X-Aether-Account-ID: default"

curl --noproxy '*' -fsS \
  -X POST "$ACCOUNT_URL/account/api-keys/<id>/revoke" \
  -H "Authorization: Bearer $ACCOUNT_SERVICE_KEY" \
  -H "X-Aether-Account-ID: default"
```

## 10. OpenAI 兼容接口冒烟测试

使用 Relay 公网端点：

```sh
RELAY_URL="http://lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com"

curl --noproxy '*' -fsS \
  "$RELAY_URL/v1/chat/completions" \
  -H "Authorization: Bearer $RELAY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "cohere/north-mini-code:free",
    "messages": [
      {"role": "user", "content": "Output only this word: pong"}
    ],
    "max_tokens": 100
  }' | jq '{id, model, provider, finish_reason: .choices[0].finish_reason, message: .choices[0].message}'
```

预期结果：

```json
{
  "finish_reason": "stop",
  "message": {
    "role": "assistant",
    "content": "pong"
  }
}
```

Provider 元数据 Header 应包括：

- `X-Aether-Provider-Id`
- `X-Aether-Provider-Name`
- `X-Aether-Provider-Version`
- `X-Aether-Router-Instance`

## 11. 故障排查

### Terraform Plan 试图替换 NAT

不要重新为 `tencentcloud_nat_gateway.aether` 添加 `subnet_id`。当前 State
中的 `subnet_id = null`；添加该字段会强制替换 NAT。

### TKE CNI 无法分配 Pod IP

如果 Pod 失败并出现类似信息：

```text
zone ap-shanghai-5 does not exist in allocator
```

确认 `ap-shanghai-5` 的 Pod ENI 子网已纳入管理，并包含在集群配置中：

```sh
terraform state show tencentcloud_subnet.aether_pods_az5
terraform state show tencentcloud_kubernetes_cluster.aether
```

预期子网：

```text
subnet-5r7klon1
10.0.2.0/24
ap-shanghai-5
```

### Postgres 登录失败

检查 `platform/k8s/tke/secrets.env`。

最常见的原因是 `sql-dsn` 中的密码未进行转义。`postgres-password`
应保留原始密码，但 `sql-dsn` 中的密码部分必须进行 URL 编码。

### OpenRouter 返回 401：缺少 Authentication Header

检查 Provider Channel 配置：

```sh
kubectl -n aether-relay port-forward svc/relay 18080:80

curl -fsS \
  -H "Authorization: Bearer $ROUTER_ADMIN_KEY" \
  http://127.0.0.1:18080/internal/provider-channels | jq .
```

预期字段：

```json
{
  "upstream_api_key_secret_ref": "env:OPENROUTER_API_KEY",
  "auth_header": "Authorization",
  "auth_prefix": ""
}
```

`auth_prefix` 为空是有意设计。Router 代码会为 `Authorization` Header
自动补充默认的 `Bearer ` 前缀。

还应在不打印密钥内容的情况下，确认 Kubernetes Secret 中存在该键：

```sh
kubectl -n aether-relay get secret relay-runtime-secrets \
  -o jsonpath='{.data.openrouter-api-key}' | wc -c
```

### 本地 kubectl 失败，但集群仍在运行

使用“获取 Kubernetes 访问权限”一节中的 TAT 兜底方式，并使用以下环境
变量运行命令：

```sh
export KUBECONFIG=/root/.kube/config
```

### 需要查看日志

```sh
kubectl -n aether-relay logs deployment/relay --tail=100
kubectl -n aether-relay logs deployment/account --tail=100
kubectl -n aether-relay logs statefulset/postgres --tail=100
kubectl -n aether-relay describe pod <pod-name>
```

## 12. 发布与回滚

更新 `platform/k8s/tke/kustomization.yaml` 中的镜像 Tag，然后执行：

```sh
kubectl apply -k platform/k8s/tke
kubectl -n aether-relay rollout status deployment/relay --timeout=300s
kubectl -n aether-relay rollout status deployment/account --timeout=300s
```

回滚到上一个 Deployment Revision：

```sh
kubectl -n aether-relay rollout undo deployment/relay
kubectl -n aether-relay rollout undo deployment/account
```

也可以恢复 `platform/k8s/tke/kustomization.yaml` 中之前的 `newTag`，
然后重新应用 Overlay。

## 13. 轮换 Secret

轮换 OpenRouter 密钥：

1. 更新本地 `platform/k8s/tke/secrets.env` 中的 `openrouter-api-key`。
2. 执行 `kubectl apply -k platform/k8s/tke`。
3. 重启 Relay Pod，使环境变量重新加载：

```sh
kubectl -n aether-relay rollout restart deployment/relay
kubectl -n aether-relay rollout status deployment/relay --timeout=300s
```

轮换 `ACCOUNT_SERVICE_KEY`、`ROUTER_ADMIN_KEY` 或
`API_KEY_HASH_SECRET` 时必须谨慎：

- `ACCOUNT_SERVICE_KEY` 会影响账户 API 的访问。
- `ROUTER_ADMIN_KEY` 会影响内部 Provider 管理 API 和同步 Job。
- `API_KEY_HASH_SECRET` 会影响已生成 Relay API 密钥的校验。轮换它会使
  现有 API 密钥哈希失效，除非重新签发密钥。

## 14. 暂停或恢复容量

暂停 TKE 节点池容量：

```sh
cd /Users/noname/Desktop/Projects/AetherCode/platform/terraform/tencentcloud

TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform apply -input=false -var='node_pool_max_size=0'
```

恢复容量：

```sh
TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform apply -input=false -var='node_pool_max_size=2'
```

验证：

```sh
kubectl get nodes
kubectl -n aether-relay get pods -o wide
```

## 15. 最终验证清单

在确认部署健康之前，请检查：

- `terraform plan -detailed-exitcode` 返回 `0`。
- `kubectl -n aether-relay get pods` 显示 Postgres、Relay 和 Account Pod
  均处于运行状态。
- `relay-public` 和 `account-public` 已获得公网 CLB 主机名。
- Relay 和 Account 的公网 URL 均能成功响应 `GET /healthz`。
- `sync-openrouter-provider-channels-once` 已执行完成。
- `sync-openrouter-provider-channels` CronJob 已存在，且未被挂起。
- `/internal/status` 报告 `in_sync: true`。
- 使用生成的 Relay API 密钥能够调用 `/v1/chat/completions`。
- 临时冒烟测试 API 密钥在使用后已被禁用或吊销。
