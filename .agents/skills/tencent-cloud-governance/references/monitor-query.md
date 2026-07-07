# 云监控查询

用 `tccli monitor GetMonitorData` 查询资源利用率，用于降配、扩容、容量复核。

## 通用原则

- 执行前先激活环境：`source ~/.tccli/venv/bin/activate`。
- 查询地域资源时显式加 `--region ap-shanghai`。不加 region 可能走默认广州，导致监控报「实例不存在或无权限」。
- 输出建议加 `2>&1 | tr -d '\r'`，避免回车符导致终端显示错位。
- 先用 `DescribeBaseMetrics` 看指标名、周期和维度，再用 `GetMonitorData` 拉数据。
- 日粒度 `--Period 86400` 常用于降配判断，通常看 7 天或 14 天峰值；5 分钟粒度 `--Period 300` 用于排查短时毛刺。
- `GetMonitorData --Instances` 接收 JSON 数组，格式为 `[{ "Dimensions": [{ "Name": "...", "Value": "..." }] }]`。

## 指标发现

```bash
# CVM 可用指标
tccli monitor DescribeBaseMetrics --Namespace QCE/CVM 2>&1 | tr -d '\r' \
  | jq -r '.MetricSet[]? | select(.MetricName|test("CPU|Mem|Disk";"i")) | [.MetricName,.MetricCName,.Unit,(.Period|join(","))] | @tsv'

# Redis 可用指标
tccli monitor DescribeBaseMetrics --Namespace QCE/REDIS 2>&1 | tr -d '\r' \
  | jq -r '.MetricSet[]? | select(.MetricName|test("Mem|Cpu|Conn|Key|Storage|Util";"i")) | [.MetricName,.MetricCName,.Unit,(.Period|join(","))] | @tsv'

# MySQL 可用指标
tccli monitor DescribeBaseMetrics --Namespace QCE/CDB 2>&1 | tr -d '\r' \
  | jq -r '.MetricSet[]? | select(.MetricName|test("Cpu|Mem|Disk|Storage|Util";"i")) | [.MetricName,.MetricCName,.Unit,(.Period|join(","))] | @tsv'

# 公网 CLB 可用指标
tccli monitor DescribeBaseMetrics --Namespace QCE/LB_PUBLIC 2>&1 | tr -d '\r' \
  | jq -r '.MetricSet[]? | select(.MetricName|test("Client.*traffic|Client.*Conn|AccOuttraffic";"i")) | [.MetricName,.MetricCName,.Unit,(.Period|join(","))] | @tsv'

# 内网 CLB 可用指标
tccli monitor DescribeBaseMetrics --Namespace QCE/LB_PRIVATE 2>&1 | tr -d '\r' \
  | jq -r '.MetricSet[]? | select(.MetricName|test("Client.*traffic|Client.*Conn|AccOuttraffic";"i")) | [.MetricName,.MetricCName,.Unit,(.Period|join(","))] | @tsv'
```

查看某个指标的维度定义：

```bash
tccli monitor DescribeBaseMetrics --Namespace QCE/CVM 2>&1 | tr -d '\r' \
  | jq '.MetricSet[]? | select(.MetricName=="CpuUsage") | {MetricName,Dimensions,Periods}'
```

## 告警策略

```bash
tccli monitor DescribeAlarmPolicies --region ap-shanghai --Module monitor 2>&1 | tr -d '\r'
```

## CVM 监控

实测可用维度：`InstanceId=ins-...`，并且必须显式 `--region ap-shanghai`。

常用指标：

| 用途 | Namespace | MetricName | 维度 |
|------|-----------|------------|------|
| CPU 利用率 | `QCE/CVM` | `CpuUsage` | `InstanceId` |
| 内存利用率 | `QCE/CVM` | `MemUsage` | `InstanceId` |
| 磁盘使用率 | `QCE/CVM` | `CvmDiskUsage` | `InstanceId` |

单实例查询：

```bash
tccli monitor GetMonitorData \
  --region ap-shanghai \
  --Namespace QCE/CVM \
  --MetricName CpuUsage \
  --Period 86400 \
  --StartTime "2026-06-04 00:00:00" \
  --EndTime "2026-06-11 00:00:00" \
  --Instances '[{"Dimensions":[{"Name":"InstanceId","Value":"ins-0npwh5xh"}]}]' \
  2>&1 | tr -d '\r' \
  | jq '.DataPoints[0] | {Dimensions, Values}'
```

批量查询所有运行中 CVM 的 7 日 CPU 峰值：

```bash
ids=$(tccli cvm DescribeInstances --region ap-shanghai --Limit 100 2>&1 | tr -d '\r' \
  | jq -c '[.InstanceSet[] | select(.InstanceState=="RUNNING") | {Dimensions:[{Name:"InstanceId",Value:.InstanceId}]}]')

tccli monitor GetMonitorData \
  --region ap-shanghai \
  --Namespace QCE/CVM \
  --MetricName CpuUsage \
  --Period 86400 \
  --StartTime "2026-06-04 00:00:00" \
  --EndTime "2026-06-11 00:00:00" \
  --Instances "$ids" \
  2>&1 | tr -d '\r' \
  | jq -r '.DataPoints[] | [(.Dimensions[]|select(.Name=="InstanceId").Value), (([.Values[]?]|max)//0)] | @tsv' \
  | sort -k2 -n
```

批量查询内存或磁盘时，只需替换 `--MetricName`：

```bash
--MetricName MemUsage
--MetricName CvmDiskUsage
```

合并 CVM 规格、CPU、内存，输出降配候选表：

```bash
tmpdir=$(mktemp -d)

tccli cvm DescribeInstances --region ap-shanghai --Limit 100 2>&1 | tr -d '\r' \
  | jq -r '.InstanceSet[] | [.InstanceId,.InstanceName,.InstanceType,.CPU,.Memory,.InstanceState] | @tsv' \
  | sort > "$tmpdir/cvm.tsv"

ids=$(jq -R -s -c 'split("\n")[:-1] | map(split("\t")) | map(select(.[5]=="RUNNING") | {Dimensions:[{Name:"InstanceId",Value:.[0]}]})' "$tmpdir/cvm.tsv")

tccli monitor GetMonitorData --region ap-shanghai --Namespace QCE/CVM --MetricName CpuUsage \
  --Period 86400 --StartTime "2026-06-04 00:00:00" --EndTime "2026-06-11 00:00:00" \
  --Instances "$ids" 2>&1 | tr -d '\r' \
  | jq -r '.DataPoints[] | [(.Dimensions[]|select(.Name=="InstanceId").Value), (([.Values[]?]|max)//0)] | @tsv' \
  | sort > "$tmpdir/cpu.tsv"

tccli monitor GetMonitorData --region ap-shanghai --Namespace QCE/CVM --MetricName MemUsage \
  --Period 86400 --StartTime "2026-06-04 00:00:00" --EndTime "2026-06-11 00:00:00" \
  --Instances "$ids" 2>&1 | tr -d '\r' \
  | jq -r '.DataPoints[] | [(.Dimensions[]|select(.Name=="InstanceId").Value), (([.Values[]?]|max)//0)] | @tsv' \
  | sort > "$tmpdir/mem.tsv"

join -t $'\t' -a1 -e 0 -o '1.1 1.2 1.3 1.4 1.5 1.6 2.2' "$tmpdir/cvm.tsv" "$tmpdir/cpu.tsv" \
  | sort -t $'\t' -k1,1 > "$tmpdir/cvm_cpu.tsv"

join -t $'\t' -a1 -e 0 -o '1.1 1.2 1.3 1.4 1.5 1.6 1.7 2.2' "$tmpdir/cvm_cpu.tsv" "$tmpdir/mem.tsv" \
  | sort -t $'\t' -k7,7n -k8,8n \
  | awk 'BEGIN{OFS="\t"; print "实例ID","名称","规格","CPU核","内存GB","状态","7日CPU峰值%","7日内存峰值%"} {print}'
```

## MySQL 监控

实测可用维度：`InstanceId=cdb-...`。

常用指标：

| 用途 | Namespace | MetricName | 维度 |
|------|-----------|------------|------|
| CPU 利用率 | `QCE/CDB` | `CpuUseRate` | `InstanceId` |
| 内存利用率 | `QCE/CDB` | `MemoryUseRate` | `InstanceId` |
| 磁盘剩余量 | `QCE/CDB` | `DiskRemaining` | `InstanceId` |

示例：

```bash
for metric in CpuUseRate MemoryUseRate DiskRemaining; do
  echo "=== $metric ==="
  tccli monitor GetMonitorData \
    --region ap-shanghai \
    --Namespace QCE/CDB \
    --MetricName "$metric" \
    --Period 86400 \
    --StartTime "2026-06-04 00:00:00" \
    --EndTime "2026-06-11 00:00:00" \
    --Instances '[{"Dimensions":[{"Name":"InstanceId","Value":"cdb-jwxj711m"}]},{"Dimensions":[{"Name":"InstanceId","Value":"cdb-qxpwmb5y"}]}]' \
    2>&1 | tr -d '\r' \
    | jq -r 'if .DataPoints then (.DataPoints[] | [(.Dimensions[]|select(.Name=="InstanceId").Value), (([.Values[]?]|max)//"-")] | @tsv) else . end'
done
```

## CLB 监控

公网 CLB 常用命名空间为 `QCE/LB_PUBLIC`，内网 CLB 常用命名空间为 `QCE/LB_PRIVATE`。

常用指标：

| 用途 | Namespace | MetricName | 维度 |
|------|-----------|------------|------|
| 入带宽峰值 | `QCE/LB_PUBLIC` / `QCE/LB_PRIVATE` | `ClientIntraffic` | `vip`, `loadBalancerPort`, `protocol`，内网通常还要 `vpcId` |
| 出带宽峰值 | `QCE/LB_PUBLIC` / `QCE/LB_PRIVATE` | `ClientOuttraffic` | `vip`, `loadBalancerPort`, `protocol`，内网通常还要 `vpcId` |
| 累计出流量 | `QCE/LB_PUBLIC` / `QCE/LB_PRIVATE` | `ClientAccOuttraffic` | `vip`, `loadBalancerPort`, `protocol`，内网通常还要 `vpcId` |
| 并发连接数 | `QCE/LB_PUBLIC` / `QCE/LB_PRIVATE` | `ClientConcurConn` | `vip`, `loadBalancerPort`, `protocol`，内网通常还要 `vpcId` |
| 新建连接数 | `QCE/LB_PUBLIC` / `QCE/LB_PRIVATE` | `ClientNewConn` | `vip`, `loadBalancerPort`, `protocol`，内网通常还要 `vpcId` |

公网单监听器查询：

```bash
tccli monitor GetMonitorData \
  --region ap-shanghai \
  --Namespace QCE/LB_PUBLIC \
  --MetricName ClientOuttraffic \
  --Period 86400 \
  --StartTime "2026-06-04 00:00:00" \
  --EndTime "2026-06-11 00:00:00" \
  --Instances '[{"Dimensions":[{"Name":"vip","Value":"42.192.177.73"},{"Name":"loadBalancerPort","Value":"8080"},{"Name":"protocol","Value":"TCP"}]}]' \
  2>&1 | tr -d '\r' \
  | jq '.DataPoints[0] | {Dimensions, Values, Max: (([.Values[]?]|max)//null)}'
```

内网单监听器查询：

```bash
tccli monitor GetMonitorData \
  --region ap-shanghai \
  --Namespace QCE/LB_PRIVATE \
  --MetricName ClientOuttraffic \
  --Period 86400 \
  --StartTime "2026-06-04 00:00:00" \
  --EndTime "2026-06-11 00:00:00" \
  --Instances '[{"Dimensions":[{"Name":"vip","Value":"10.11.2.8"},{"Name":"loadBalancerPort","Value":"3000"},{"Name":"protocol","Value":"TCP"},{"Name":"vpcId","Value":"8774419"}]}]' \
  2>&1 | tr -d '\r' \
  | jq '.DataPoints[0] | {Dimensions, Values, Max: (([.Values[]?]|max)//null)}'
```

注意：CLB 的 `protocol` 值建议直接使用 `DescribeListeners` 返回的 `Protocol`。按流量计费的公网 CLB 中，带宽上限通常是限速上限；降本判断要优先结合实例是否闲置、是否可合并、账单实例费，而不是只看带宽配置。

## Redis 监控

指标元数据中 `StorageUs` 的维度为 `redis_uuid`，实测可被接口接受：

```bash
tccli monitor GetMonitorData \
  --region ap-shanghai \
  --Namespace QCE/REDIS \
  --MetricName StorageUs \
  --Period 300 \
  --StartTime "2026-06-10 00:00:00" \
  --EndTime "2026-06-10 01:00:00" \
  --Instances '[{"Dimensions":[{"Name":"redis_uuid","Value":"crs-fzpfbkja"}]}]' \
  2>&1 | tr -d '\r'
```

注意：接口可能返回空 `Timestamps`/`Values`，这不等于零占用。遇到 Redis 指标为空时，先换时间范围、周期或指标名复核；仍为空则不要把它作为降配依据。

常试指标：

```bash
StorageUs
StorageUsMin
Storage
CpuUs
CpuUsMin
ConnectionsUs
MemsizeDatasetMin
```

## 常见排查

- `Unknown options: --Region` → tccli 公共参数用小写：`--region ap-shanghai`。
- `unauthorized operation or the instance has been destroyed`，但资源列表确认实例存在 → 优先检查 `--region` 是否显式指定为资源所在地域，再检查维度名和值。
- CVM `DescribeBaseMetrics` 可能显示 `vm_uuid`，但 `GetMonitorData` 实测用 `InstanceId=ins-...` 才可查。
- CLB 监控为空时，先核对公网/内网命名空间、监听器端口、协议大小写和内网 `vpcId` 维度。
- `Values: []` → 指标无上报或该时段无数据，不要当成 0；换指标、周期、时间范围或去控制台核对。
- `DiskUsage` 返回全 0 时，尝试 `CvmDiskUsage`。
