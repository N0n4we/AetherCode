# Aether Code

Aether编程工具及其中转站

## Feature

- 使用go构建，低延迟高吞吐
- 自动选择渠道，动态计费
- 比一般中转站更高的缓存命中率

## 基建架构

当前部署分为两层 IaC：

- Terraform：`platform/terraform/tencentcloud`，管理腾讯云基础设施、TKE
  集群底座，以及一个 EIP-backed 业务公网 CLB。
- Kubernetes：`platform/k8s/tke`，管理 TKE 内的应用运行时。Relay 和
  Open WebUI 通过固定 `NodePort` 暴露给 Terraform 管理的业务 CLB。

完整部署 runbook 见 [platform/RUNBOOK.md](platform/RUNBOOK.md)。

### Terraform 管理层

```mermaid
flowchart TB
  User["Operator / kubectl"]
  Internet["Internet"]
  Browser["Browser / OpenAI-compatible client"]

  subgraph TF["Terraform: platform/terraform/tencentcloud"]
    VPC["VPC aether<br/>vpc-kdegvs6i<br/>10.0.0.0/16"]

    subgraph Network["VPC Network"]
      NodeSubnet["Node Subnet<br/>subnet-6siua6l3<br/>10.0.1.0/24<br/>ap-shanghai-5"]
      PodSubnet4["Pod ENI Subnet<br/>subnet-iwqt8au5<br/>10.0.0.0/24<br/>ap-shanghai-4"]
      PodSubnet5["Pod ENI Subnet<br/>subnet-5r7klon1<br/>10.0.2.0/24<br/>ap-shanghai-5"]
      DefaultRoute["Default Route<br/>0.0.0.0/0"]
      NAT["NAT Gateway<br/>nat-6hvu0buf"]
      NATEIP["NAT EIP<br/>eip-1q4hutfv"]
    end

    SGDefault["Default Security Group<br/>sg-bvlotzok"]
    SGAPI["Kube API Endpoint SG<br/>sg-jqzzj85e<br/>allowed CIDRs only"]

    TKE["TKE Managed Cluster<br/>aether / cls-26zqizrl<br/>VPC-CNI tke-route-eni"]
    NodePool["TKE Node Pool<br/>np-8ldph9uj<br/>min 0 / max 2"]
    KubeEndpoint["Public Kubernetes API Endpoint<br/>tencentcloud_kubernetes_cluster_endpoint"]
    APIEndpointCLB["Tencent CLB for kube-apiserver<br/>created by TKE endpoint resource"]

    AppEIP["Application EIP<br/>110.40.156.154"]
    AppCLB["Application CLB<br/>aether-app-public"]
    OWListener["TCP :80<br/>Open WebUI NodePort 31327"]
    RelayListener["TCP :8080<br/>Relay NodePort 31326"]

    ASRole["CAM Role AS_QCSRole<br/>Auto Scaling policies"]
    KeyPair["SSH Key Pair<br/>aether_tf_node_pool"]
  end

  VPC --> Network
  NodeSubnet --> NodePool
  PodSubnet4 --> TKE
  PodSubnet5 --> TKE
  DefaultRoute --> NAT --> NATEIP --> Internet
  SGDefault --> NodePool
  ASRole --> NodePool
  KeyPair --> NodePool
  NodePool --> TKE
  TKE --> KubeEndpoint --> APIEndpointCLB
  SGAPI --> APIEndpointCLB
  User --> APIEndpointCLB
  Browser --> AppEIP --> AppCLB
  AppCLB --> OWListener
  AppCLB --> RelayListener
```

说明：

- `tencentcloud_kubernetes_cluster_endpoint.aether_public` 会间接创建一个
  kube-apiserver 公网 CLB；它只用于 Kubernetes API，不承载业务流量。
- Terraform 直接管理业务公网入口 `aether-app-public`，避免腾讯云默认
  CLB 域名拦截。
- TCR Personal 镜像仓库
  `ccr.ccs.tencentyun.com/aethercode-100034871923/router` 目前由文档记录，
  不在 Terraform state 中。

### Kubernetes 运行层

```mermaid
flowchart LR
  Client["Client / OpenAI-compatible SDK"]
  Admin["Operator / Account API caller"]
  Browser["Browser / Operator UI"]
  OpenRouter["OpenRouter API<br/>https://openrouter.ai/api/v1"]
  TCR["TCR Personal<br/>router:20260703-183944"]
  AppCLB["Terraform app CLB<br/>110.40.156.154"]

  subgraph TKE["TKE Cluster: aether"]
    subgraph NS["Namespace: aether-relay"]
      Secret["Secret<br/>relay-runtime-secrets<br/>generated from secrets.env"]
      RuntimeConfig["ConfigMap<br/>relay-runtime-config"]

      PostgresSvc["Service<br/>postgres"]
      Postgres["StatefulSet<br/>postgres<br/>postgres:16-alpine"]

      RelaySvc["Service<br/>relay<br/>ClusterIP"]
      Relay["Deployment<br/>relay x2<br/>/router<br/>init: /migrate"]
      RelayPublic["Service<br/>relay-public<br/>NodePort 31326"]

      AccountSvc["Service<br/>account<br/>ClusterIP"]
      Account["Deployment<br/>account x2<br/>/account-service"]

      ProviderSyncConfig["ConfigMap<br/>openrouter-provider-channel-sync"]
      ProviderSyncOnce["Job<br/>sync-openrouter-provider-channels-once"]
      ProviderSyncCron["CronJob<br/>sync-openrouter-provider-channels<br/>every 15 minutes"]
    end
    subgraph OWNS["Namespace: openwebui"]
      OpenWebUIConfig["ConfigMap<br/>openwebui-config<br/>ENABLE_PERSISTENT_CONFIG=False"]
      OpenWebUISecret["Secret<br/>openwebui-secrets<br/>from openwebui-secrets.env"]
      OpenWebUIData["PVC<br/>openwebui-data<br/>/app/backend/data"]
      OpenWebUISvc["Service<br/>openwebui<br/>ClusterIP"]
      OpenWebUI["Deployment<br/>openwebui x1<br/>open-webui:v0.10.2"]
      OpenWebUIPublic["Service<br/>openwebui-public<br/>NodePort 31327"]
    end
  end

  TCR --> Relay
  TCR --> Account
  Secret --> Relay
  Secret --> Account
  Secret --> Postgres
  RuntimeConfig --> Relay
  RuntimeConfig --> Account

  PostgresSvc --> Postgres
  Relay --> PostgresSvc
  Account --> PostgresSvc

  Client --> AppCLB --> RelayPublic --> RelaySvc --> Relay
  Admin --> AccountSvc --> Account

  Relay --> OpenRouter
  ProviderSyncConfig --> ProviderSyncOnce
  ProviderSyncConfig --> ProviderSyncCron
  ProviderSyncOnce --> RelaySvc
  ProviderSyncCron --> RelaySvc
  OpenWebUIConfig --> OpenWebUI
  OpenWebUISecret --> OpenWebUI
  OpenWebUIData --> OpenWebUI
  Browser --> AppCLB --> OpenWebUIPublic --> OpenWebUISvc --> OpenWebUI
  OpenWebUI --> AppCLB --> RelayPublic --> RelaySvc --> Relay
```

业务入口：

- Open WebUI: `https://openwebui.n0n4w3.cn`
- Relay OpenAI-compatible base URL: `https://relay.n0n4w3.cn/v1`
- Account service: 默认不公开，通过集群内 `account.aether-relay.svc.cluster.local` 访问。

Owner 边界：

- `relay-public`、`openwebui`、`openwebui-public` 是 K8s manifest 管理的
  `NodePort` Service。
- 业务公网 EIP/CLB/listener/后端 attachment 由 Terraform 管理。
- kube-apiserver 公网 CLB 是 TKE 控制面入口，不承载 relay/Open WebUI。
- Open WebUI 当前通过公网 relay listener 调用 relay；设置域名后可把
  `OPENAI_API_BASE_URL` 改为域名形式。
- OpenRouter provider channel 配置由
  `sync-openrouter-provider-channels` CronJob 每 15 分钟按声明式配置纠偏。
