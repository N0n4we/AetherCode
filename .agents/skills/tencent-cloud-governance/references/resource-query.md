# 资源查询

查询本账号腾讯云各类资源。命令不可用或凭证过期见 `references/environment-setup.md`。

## 通用调用格式

```bash
tccli <service> <Action> --region ap-shanghai [--参数 值]
```

> 默认区域 **ap-shanghai**；tccli 默认 region 为 ap-guangzhou，查不到资源时务必加 `--region ap-shanghai`。
> 输出建议加 `2>&1 | tr -d '\r'`，避免回车符导致终端显示错位。
> 列表类接口分页参数 `--Limit`（首字母大写）；数组类参数传 JSON，如 `--PhoneNumberSet '["+8613711112222"]'`。

## 各服务查询命令

### 一、计算与容器
| 资源 | 命令 |
|------|------|
| CVM 云服务器 | `tccli cvm DescribeInstances --region ap-shanghai --Limit 100` |
| TKE 集群 | `tccli tke DescribeClusters --region ap-shanghai` |
| TKE Serverless / EKS | `tccli tke DescribeEKSClusters --region ap-shanghai` |
| TCR 容器镜像 | `tccli tcr DescribeInstances --region ap-shanghai` |

### 二、网络与内容分发
| 资源 | 命令 |
|------|------|
| CLB 负载均衡 | `tccli clb DescribeLoadBalancers --region ap-shanghai` |
| VPC 私有网络 | `tccli vpc DescribeVpcs --region ap-shanghai` |
| CDN 内容分发 | `tccli cdn DescribeDomains` （CDN 为全局服务，无需 region）|
| NAT 网关 | `tccli vpc DescribeNatGateways --region ap-shanghai` |
| EIP 公网 IP | `tccli vpc DescribeAddresses --region ap-shanghai` |
| Private DNS 私有域解析 | `tccli privatedns DescribePrivateZoneList` |

### 三、数据库、存储与备份
| 资源 | 命令 |
|------|------|
| MySQL | `tccli cdb DescribeDBInstances --region ap-shanghai` |
| MongoDB | `tccli mongodb DescribeDBInstances --region ap-shanghai` |
| Redis | `tccli redis DescribeInstances --region ap-shanghai` |
| CBS 云硬盘 | `tccli cbs DescribeDisks --region ap-shanghai` |
| 快照 Snapshot | `tccli cbs DescribeSnapshots --region ap-shanghai` |
| COS 对象存储 | 见下方「COS 访问」，需用 coscli |

### 四、大数据与中间件
| 资源 | 命令 |
|------|------|
| ES Elasticsearch | `tccli es DescribeInstances --region ap-shanghai --Limit 1` |
| EMR 弹性 MapReduce | `tccli emr DescribeInstancesList --region ap-shanghai --DisplayStrategy clusterList` |
| Oceanus 流计算 | `tccli oceanus DescribeClusters --region ap-shanghai` |
| CKafka 消息队列 | `tccli ckafka DescribeInstances --region ap-shanghai` |

### 五、安全、运维与管理
| 资源 | 命令 |
|------|------|
| Advisor 云顾问 | `tccli advisor DescribeStrategies` |
| CWP 主机安全 | `tccli cwp DescribeMachinesSimple --MachineType CVM --MachineRegion ap-shanghai` |
| CLS 日志服务 | `tccli cls DescribeLogsets --region ap-shanghai` |

### 六、其他服务
| 资源 | 命令 |
|------|------|
| SMS 短信 | `tccli sms DescribePhoneNumberInfo --PhoneNumberSet '["+8613711112222"]'` |
| ICP 备案 | `tccli ba DescribeGetAuthInfo` |

## COS 访问 (coscli)

tccli 不支持 COS，使用本技能自带的封装脚本 `scripts/coscli-auth.sh`。它自动从 tccli 读取临时凭证（含 sessionToken 与过期校验），通过命令行参数注入 coscli，默认区域 ap-shanghai。

```bash
# 列出所有桶
skills/tencent-cloud-governance/scripts/coscli-auth.sh ls

# 列出某个桶内对象
skills/tencent-cloud-governance/scripts/coscli-auth.sh ls cos://qibaotu-1313190940

# 上传 / 下载
skills/tencent-cloud-governance/scripts/coscli-auth.sh cp ./local.txt cos://bucket-1313190940/path/
skills/tencent-cloud-governance/scripts/coscli-auth.sh cp cos://bucket-1313190940/path/obj ./

# 切换区域
COS_REGION=ap-guangzhou skills/tencent-cloud-governance/scripts/coscli-auth.sh ls
```

环境变量：`COS_REGION`（默认 ap-shanghai）、`TCCLI_PROFILE`（默认 default）、`TCCLI_CRED`（凭证文件路径）。

## 资源查询排查

- 返回 `TotalCount: 0` 但确信有资源 → region 错了，加 `--region ap-shanghai`。
- 报 `UnknownOptions` 或 `usage:` → Action 名或参数名写错（参数大小写敏感，先 `tccli <service> <Action> help` 查看）。
- coscli 列桶返回 0 → endpoint 的 region 与桶所在区域不一致，桶都在 ap-shanghai。
- 鉴权失败（`AuthFailure` / token 失效 / coscli 鉴权报错）→ 临时凭证过期，见 `references/environment-setup.md`。
