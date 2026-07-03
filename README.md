# Aether Code

Aether编程工具及其中转站

## Feature

- 使用go构建，低延迟高吞吐
- 自动选择渠道，动态计费
- 比一般中转站更高的缓存命中率

## 基建架构

当前部署分为两层 IaC：

- Terraform：`platform/terraform/tencentcloud`，管理腾讯云基础设施和 TKE
  集群底座。
- Kubernetes：`platform/k8s/tke`，管理 TKE 内的应用运行时，并通过
  `Service type=LoadBalancer` 让 TKE 控制器创建业务 CLB。

完整部署 runbook 见 [platform/RUNBOOK.md](platform/RUNBOOK.md)。

### Terraform 管理层

```mermaid
flowchart TB
  User["Operator / kubectl"]
  Internet["Internet"]

  subgraph TF["Terraform: platform/terraform/tencentcloud"]
    VPC["VPC aether<br/>vpc-kdegvs6i<br/>10.0.0.0/16"]

    subgraph Network["VPC Network"]
      NodeSubnet["Node Subnet<br/>subnet-6siua6l3<br/>10.0.1.0/24<br/>ap-shanghai-5"]
      PodSubnet4["Pod ENI Subnet<br/>subnet-iwqt8au5<br/>10.0.0.0/24<br/>ap-shanghai-4"]
      PodSubnet5["Pod ENI Subnet<br/>subnet-5r7klon1<br/>10.0.2.0/24<br/>ap-shanghai-5"]
      DefaultRoute["Default Route<br/>0.0.0.0/0"]
      NAT["NAT Gateway<br/>nat-6hvu0buf"]
      EIP["NAT EIP<br/>eip-1q4hutfv"]
    end

    SGDefault["Default Security Group<br/>sg-bvlotzok"]
    SGAPI["Kube API Endpoint SG<br/>sg-jqzzj85e<br/>allowed CIDRs only"]

    TKE["TKE Managed Cluster<br/>aether / cls-26zqizrl<br/>VPC-CNI tke-route-eni"]
    NodePool["TKE Node Pool<br/>np-8ldph9uj<br/>min 0 / max 2"]
    KubeEndpoint["Public Kubernetes API Endpoint<br/>tencentcloud_kubernetes_cluster_endpoint"]
    APIEndpointCLB["Tencent CLB for kube-apiserver<br/>created by TKE endpoint resource"]

    ASRole["CAM Role AS_QCSRole<br/>Auto Scaling policies"]
    KeyPair["SSH Key Pair<br/>aether_tf_node_pool"]
  end

  VPC --> Network
  NodeSubnet --> NodePool
  PodSubnet4 --> TKE
  PodSubnet5 --> TKE
  DefaultRoute --> NAT --> EIP --> Internet
  SGDefault --> NodePool
  ASRole --> NodePool
  KeyPair --> NodePool
  NodePool --> TKE
  TKE --> KubeEndpoint --> APIEndpointCLB
  SGAPI --> APIEndpointCLB
  User --> APIEndpointCLB
```

说明：

- Terraform 不直接定义业务 CLB。
- `tencentcloud_kubernetes_cluster_endpoint.aether_public` 会间接创建一个
  kube-apiserver 公网 CLB。
- TCR Personal 镜像仓库
  `ccr.ccs.tencentyun.com/aethercode-100034871923/router` 目前由文档记录，
  不在 Terraform state 中。

### Kubernetes 运行层

```mermaid
flowchart LR
  Client["Client / OpenAI-compatible SDK"]
  Admin["Operator / Account API caller"]
  OpenRouter["OpenRouter API<br/>https://openrouter.ai/api/v1"]
  TCR["TCR Personal<br/>router:20260703-183944"]

  subgraph TKE["TKE Cluster: aether"]
    subgraph NS["Namespace: aether-relay"]
      Secret["Secret<br/>relay-runtime-secrets<br/>generated from secrets.env"]
      RuntimeConfig["ConfigMap<br/>relay-runtime-config"]

      PostgresSvc["Service<br/>postgres"]
      Postgres["StatefulSet<br/>postgres<br/>postgres:16-alpine"]

      RelaySvc["Service<br/>relay<br/>ClusterIP"]
      Relay["Deployment<br/>relay x2<br/>/router<br/>init: /migrate"]
      RelayPublic["Service<br/>relay-public<br/>type=LoadBalancer"]
      RelayCLB["Tencent CLB<br/>lb-74kbv10l-...<br/>created by TKE controller"]

      AccountSvc["Service<br/>account<br/>ClusterIP"]
      Account["Deployment<br/>account x2<br/>/account-service"]
      AccountPublic["Service<br/>account-public<br/>type=LoadBalancer"]
      AccountCLB["Tencent CLB<br/>lb-l5vk4kbl-...<br/>created by TKE controller"]

      ProviderSyncConfig["ConfigMap<br/>openrouter-provider-channel-sync"]
      ProviderSyncOnce["Job<br/>sync-openrouter-provider-channels-once"]
      ProviderSyncCron["CronJob<br/>sync-openrouter-provider-channels<br/>every 15 minutes"]
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

  Client --> RelayCLB --> RelayPublic --> RelaySvc --> Relay
  Admin --> AccountCLB --> AccountPublic --> AccountSvc --> Account

  Relay --> OpenRouter
  ProviderSyncConfig --> ProviderSyncOnce
  ProviderSyncConfig --> ProviderSyncCron
  ProviderSyncOnce --> RelaySvc
  ProviderSyncCron --> RelaySvc
```

业务入口：

- Relay:
  `http://lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com/v1`
- Account:
  `http://lb-l5vk4kbl-i1jarzn7ckm3sf2o.clb.sh-tencentclb.com`

Owner 边界：

- `relay-public` 和 `account-public` 是 K8s manifest 管理的 Service。
- 对应两个业务 CLB 由 TKE cloud controller 根据 Service 自动创建、更新
  和回收，不应再用 Terraform 单独接管。
- OpenRouter provider channel 配置由
  `sync-openrouter-provider-channels` CronJob 每 15 分钟按声明式配置纠偏。
