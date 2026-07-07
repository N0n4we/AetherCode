# AetherCode 项目索引

## 项目概览

AetherCode 是 Go 实现的 OpenAI-compatible relay / router 与账号 API 服务，配套腾讯云 TKE 部署、Terraform 基础设施和 OpenSpec 规格管理。

主要能力：

- OpenAI-compatible relay：`/v1/chat/completions`、`/v1/completions`、模型元数据与未实现路由 shell。
- Provider channel 管理：共享数据库中的供应商、模型、优先级、权重、endpoint capability 配置。
- Account service：账号维度 relay API key 创建、查询、禁用、撤销。
- 腾讯云运行时：Terraform 管理 VPC/TKE/CLB/EIP；Kubernetes manifest 管理 relay、account、Postgres、Open WebUI。

## 顶层目录索引

| 路径 | 用途 |
| --- | --- |
| `README.md` | 项目总览、当前部署架构、入口地址、Terraform/Kubernetes owner 边界。 |
| `platform/RUNBOOK.md` | 腾讯云 TKE 部署、更新、巡检、回滚、烟测 runbook。 |
| `platform/RUNBOOK_cn.md` | 中文运维 runbook。 |
| `router/` | Go relay/router/account-service/migrate 源码、测试、Dockerfile。 |
| `platform/terraform/tencentcloud/` | 腾讯云 Terraform：VPC、子网、NAT、TKE、节点池、公网 K8s API endpoint、业务 CLB/EIP/listener/backend attachment。 |
| `platform/terraform/environments/gcp-dev/` | GCP dev Terraform 环境。 |
| `platform/k8s/base/` | Kubernetes base manifests。 |
| `platform/k8s/tke/` | TKE runtime manifests：relay、account、Postgres、Open WebUI、ConfigMap、Secret 示例、Jobs/CronJobs、NodePort Services。 |
| `openspec/specs/` | 当前主规格。 |
| `openspec/changes/archive/` | 已归档 OpenSpec 变更。 |
| `docs/` | 项目设计/待办补充文档。 |
| `.agents/skills/tencent-cloud-governance/` | 腾讯云治理技能资料和脚本。 |
| `.pi/skills/` | 本仓库内 OpenSpec workflow 技能。 |

## Router 代码索引

| 路径 | 用途 |
| --- | --- |
| `router/cmd/router/main.go` | relay/router 入口。 |
| `router/cmd/account-service/main.go` | account service 入口。 |
| `router/cmd/migrate/main.go` | 数据库迁移入口。 |
| `router/cmd/mock-provider/main.go` | 本地/测试 mock upstream provider。 |
| `router/internal/app/` | HTTP server、OpenAI-compatible relay pipeline、admin/provider-channel routes、route shell、models、usage、错误处理和相关测试。 |
| `router/internal/account/` | Account API key lifecycle service 和测试。 |
| `router/internal/store/` | Provider/platform 数据模型、缓存、数据库访问和测试。 |
| `router/internal/config/` | 环境变量配置解析。 |
| `router/internal/upstream/` | Upstream HTTP client。 |
| `router/scripts/k3d-test.sh` | k3d 集成测试脚本。 |
| `router/deploy/k3d-test.yaml` | k3d 测试部署配置。 |
| `router/Dockerfile` | 多入口镜像构建：`/router`、`/account-service`、`/migrate`。 |

## 常用命令

从仓库根目录执行：

```sh
cd router
go test ./...
go run ./cmd/router
go run ./cmd/account-service
go run ./cmd/migrate
```

Terraform 腾讯云目录：

```sh
cd platform/terraform/tencentcloud
TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" TENCENTCLOUD_PROFILE=default terraform init
TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" TENCENTCLOUD_PROFILE=default terraform plan -input=false -detailed-exitcode
```

Kubernetes TKE runtime：

```sh
kubectl apply -k platform/k8s/tke
```

镜像构建按 `platform/RUNBOOK.md`：Docker build context 需要从 `/Users/noname/Desktop/Projects` 执行，因为 `router/Dockerfile` 依赖 sibling checkout `gollm`。

## 配置与密钥边界

不要提交：

- `platform/k8s/tke/secrets.env`
- `platform/k8s/tke/openwebui-secrets.env`
- kubeconfig 文件
- OpenRouter/API provider key
- relay API key 明文
- TCR 登录凭证

可提交的示例文件：

- `platform/k8s/tke/secrets.env.example`
- `platform/k8s/tke/openwebui-secrets.env.example`

## 当前部署线索

| 项 | 值 |
| --- | --- |
| 腾讯云 Region | `ap-shanghai` |
| TKE cluster | `cls-26zqizrl` / `aether` |
| Relay public base URL | `https://relay.n0n4w3.cn/v1` |
| Open WebUI | `https://openwebui.n0n4w3.cn` |
| Account service | 默认集群内访问：`account.aether-relay.svc.cluster.local` |
| Relay NodePort | `31326` |
| Open WebUI NodePort | `31327` |
| Router image repo | `ccr.ccs.tencentyun.com/aethercode-100034871923/router` |

## 开发约定

- Go module：`aethercode-router`，Go `1.25`。
- 数据库 DSN 由 `SQL_DSN` 控制；空值或 `local` 使用本地 `router.db`。
- Router public API key：`ROUTER_API_KEY`。
- Provider admin API key：`ROUTER_ADMIN_KEY`。
- Account service auth：`ACCOUNT_SERVICE_KEY` + `X-Aether-Account-ID`。
- Account-issued relay keys：启用 `RELAY_ACCOUNT_KEY_AUTH=true`，并配置 `API_KEY_HASH_SECRET`。
- Provider cache 通过共享 DB version 轮询刷新，默认 `CONFIG_SYNC_INTERVAL=5s`。
- OpenSpec 相关需求变更先看 `openspec/specs/` 和 `openspec/changes/archive/`，再新增或修改规格。

## 变更入口建议

| 任务类型 | 优先查看 |
| --- | --- |
| Relay 路由/兼容性 | `router/internal/app/routes.go`、`router/internal/app/openai.go`、`router/internal/app/route_shell_test.go`、`router/README.md` |
| Provider 选择/缓存 | `router/internal/store/`、`router/internal/app/admin_channels.go`、`router/internal/app/models.go` |
| Account API key | `router/internal/account/`、`router/internal/store/platform.go` |
| 配置项 | `router/internal/config/config.go`、`router/README.md` |
| 容器/入口 | `router/Dockerfile`、`router/cmd/*/main.go` |
| TKE runtime | `platform/k8s/tke/README.md`、`platform/k8s/tke/kustomization.yaml`、相关 manifest |
| Terraform/Tencent Cloud | `platform/terraform/tencentcloud/README.md`、`platform/RUNBOOK.md` |
| OpenSpec | `openspec/specs/`、`openspec/changes/archive/*/{proposal.md,design.md,tasks.md,specs/}` |

## 验证习惯

- Go 代码改动：优先运行 `cd router && go test ./...`；若只改单包，可先跑目标包测试，再按风险决定是否全量。
- Kubernetes manifest 改动：优先用 `kubectl apply -k platform/k8s/tke --dry-run=server` 或按 runbook 的实际集群验证路径。
- Terraform 改动：运行对应环境 `terraform fmt` 和 `terraform plan -input=false -detailed-exitcode`，应用前必须人工审阅 plan。
- 文档/索引改动：重读目标文件确认结构、路径、命令无明显拼写错误。
