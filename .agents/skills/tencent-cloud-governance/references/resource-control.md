# 资源控制与变更执行

用于在用户明确要求后，对腾讯云资源执行启停、释放、改配、绑定、解绑、策略调整等会改变线上状态的动作。这个模块不是固定流程；它给出执行边界、核验清单和常见命令入口。

## 执行边界

- 只读查询可以主动执行；写操作必须等用户明确确认资源、动作和时间窗口。
- 高风险动作包括释放、删除、销毁、降配、改公网入口、改安全组、切换路由、清空对象、缩短日志/备份保留期。
- 执行前必须保存当前状态快照，执行后必须复查目标状态，并把命令、时间、资源 ID、结果记录到答复中。
- 命令参数可能随产品更新；执行写操作前先运行 `tccli <service> <Action> help` 核对参数名和必填项。
- 默认区域为 `ap-shanghai`。全局服务按产品要求省略 region；COS 使用 `scripts/coscli-auth.sh`。

## 变更前确认模板

向用户确认时至少说明：

- 对象：产品、资源 ID、资源名、地域。
- 动作：要执行的具体 Action，例如停机、释放、解绑、改保留期。
- 影响：预期中断、数据保留、计费变化、回滚难度。
- 窗口：立即执行还是指定低峰时间。
- 回滚：能否回滚、回滚命令或恢复条件。

示例：

```text
请确认是否在 ap-shanghai 对 ins-xxxx 执行 StopInstances。影响：实例将停止提供服务，按量实例停止后计算费用变化以腾讯云计费为准；可通过 StartInstances 启动恢复。
```

## 执行记录目录

建议把一次变更的输入和输出保存到本地目录，便于复盘：

```bash
mkdir -p .resource-control/YYYYMMDD-HHMMSS
tccli cvm DescribeInstances --region ap-shanghai --InstanceIds '["ins-xxxx"]' 2>&1 | tr -d '\r' > .resource-control/YYYYMMDD-HHMMSS/before.json
# 执行变更命令后：
tccli cvm DescribeInstances --region ap-shanghai --InstanceIds '["ins-xxxx"]' 2>&1 | tr -d '\r' > .resource-control/YYYYMMDD-HHMMSS/after.json
```

## 通用执行步骤

1. 查询资源当前状态，确认资源存在且 region 正确。
2. 查询账单或监控证据，确认动作符合用户目标。
3. 运行 `help` 核对写操作参数。
4. 让用户确认对象、动作、影响和窗口。
5. 保存 before 快照。
6. 执行变更。
7. 保存 after 快照并检查状态。
8. 汇报结果、残余风险和后续观察点。

## 常见资源控制命令

### CVM 云服务器

```bash
# 查询
tccli cvm DescribeInstances --region ap-shanghai --InstanceIds '["ins-xxxx"]'

# 停机 / 开机 / 重启
tccli cvm StopInstances --region ap-shanghai --InstanceIds '["ins-xxxx"]'
tccli cvm StartInstances --region ap-shanghai --InstanceIds '["ins-xxxx"]'
tccli cvm RebootInstances --region ap-shanghai --InstanceIds '["ins-xxxx"]'

# 释放按量计费实例，高风险，必须二次确认
tccli cvm TerminateInstances --region ap-shanghai --InstanceIds '["ins-xxxx"]'
```

注意：包年包月、关机不收费、数据盘保留、实例销毁保护等规则以实例当前配置和腾讯云返回为准，执行前要查实例详情。

### CBS 云硬盘与快照

```bash
# 查询云硬盘 / 快照
tccli cbs DescribeDisks --region ap-shanghai --DiskIds '["disk-xxxx"]'
tccli cbs DescribeSnapshots --region ap-shanghai --SnapshotIds '["snap-xxxx"]'

# 创建快照
tccli cbs CreateSnapshot --region ap-shanghai --DiskId disk-xxxx --SnapshotName before-change-YYYYMMDD

# 卸载 / 退还云硬盘，高风险
tccli cbs DetachDisks --region ap-shanghai --DiskIds '["disk-xxxx"]'
tccli cbs TerminateDisks --region ap-shanghai --DiskIds '["disk-xxxx"]'

# 删除快照，高风险
tccli cbs DeleteSnapshots --region ap-shanghai --SnapshotIds '["snap-xxxx"]'
```

注意：释放云硬盘前确认是否随实例退还、是否有文件系统挂载、是否已有可恢复快照。

### EIP 公网 IP

```bash
# 查询
tccli vpc DescribeAddresses --region ap-shanghai --AddressIds '["eip-xxxx"]'

# 解绑 / 释放，高风险
tccli vpc DisassociateAddress --region ap-shanghai --AddressId eip-xxxx
tccli vpc ReleaseAddresses --region ap-shanghai --AddressIds '["eip-xxxx"]'
```

注意：公网入口变更会直接影响访问路径。释放前确认没有 DNS、白名单、第三方回调或业务配置依赖该 IP。

### CLB 负载均衡

```bash
# 查询
tccli clb DescribeLoadBalancers --region ap-shanghai --LoadBalancerIds '["lb-xxxx"]'

# 删除 CLB，高风险
tccli clb DeleteLoadBalancer --region ap-shanghai --LoadBalancerIds '["lb-xxxx"]'
```

注意：删除前要查询监听器、后端绑定、域名解析和监控流量，避免误删仍在承载流量的入口。

### NAT 网关

```bash
# 查询
tccli vpc DescribeNatGateways --region ap-shanghai --NatGatewayIds '["nat-xxxx"]'

# 删除 NAT 网关，高风险
tccli vpc DeleteNatGateway --region ap-shanghai --NatGatewayId nat-xxxx
```

注意：删除前确认路由表、出网依赖、固定出口 IP、第三方白名单和容器/云服务器出网路径。

### Redis

```bash
# 查询
tccli redis DescribeInstances --region ap-shanghai --InstanceIds '["crs-xxxx"]'

# 清退实例，高风险
tccli redis DestroyPrepaidInstance --region ap-shanghai --InstanceId crs-xxxx
```

注意：Redis 删除/清退前要确认备份、主从/集群关系、业务连接串和缓存击穿风险。不同计费模式 Action 可能不同，必须先看 `help`。

### CLS 日志服务

```bash
# 查询日志集
tccli cls DescribeLogsets --region ap-shanghai

# 查询主题
tccli cls DescribeTopics --region ap-shanghai --Filters '[{"Key":"logsetId","Values":["logset-xxxx"]}]'

# 修改日志主题配置，如保留天数
tccli cls ModifyTopic --region ap-shanghai --TopicId topic-xxxx --Period 7
```

注意：缩短保留期可能导致历史日志不可恢复；执行前确认审计、排障、等保和业务留存要求。

### COS 对象存储

COS 使用本 skill 的凭证封装脚本：

```bash
# 查询桶和对象
scripts/coscli-auth.sh ls
scripts/coscli-auth.sh ls cos://bucket-1313190940/path/

# 删除单个对象，高风险
scripts/coscli-auth.sh rm cos://bucket-1313190940/path/object.txt

# 删除前建议先 dry-run 思路：列出目标前缀并抽样核对，不要直接递归删除
scripts/coscli-auth.sh ls cos://bucket-1313190940/path/
```

注意：递归删除、生命周期策略、跨区域复制、静态网站桶和业务上传路径都可能影响大量对象。批量删除前必须让用户确认前缀、数量级和是否有版本控制/备份。

## 变更后复查

复查至少覆盖：

- 资源状态是否达到预期。
- 账单或计费状态是否符合预期；当日费用可能延迟。
- 监控告警是否新增异常。
- 业务入口是否仍可用，或是否按预期下线。
- 是否需要补充标签、备注、工单或后续观察时间点。

## 回答格式建议

执行完成后简洁报告：

```text
已执行：StopInstances
资源：ap-shanghai / ins-xxxx / qibaotu-api-1
时间：2026-06-10 15:04:22 CST
结果：实例状态从 RUNNING 变为 STOPPED
证据：before/after 快照保存在 .resource-control/20260610-150422/
后续：建议观察 30 分钟告警；账单变化通常有延迟。
```
