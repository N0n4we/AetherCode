# 基础模块：费用查询

查询账户余额与账单。`billing` 为全局服务，**无需 `--region`**。命令不可用或凭证过期见 `references/environment-setup.md`。

> **单位**：`DescribeAccountBalance` 金额字段单位是**分**（÷100 得元）；账单接口的 `RealTotalCost` 单位是**元**，可直接用。
> **账单延迟**：账单数据约 **T+1** 才结算，查「当天/当月至今」可能返回空或不完整，分析趋势请用已结算的整月。

## 账户余额

```bash
tccli billing DescribeAccountBalance 2>&1 | tr -d '\r'
# Balance 字段单位为分，例如 2577773 = ¥25777.73
```

## 月度账单汇总

按产品、付费方式、地域三种维度汇总，参数都是 `--BeginTime` + `--EndTime`（格式 `"YYYY-MM-DD HH:MM:SS"`）：

```bash
# 按产品（最常用，看钱花在哪些云服务上）
tccli billing DescribeBillSummaryByProduct \
  --BeginTime "2026-05-01 00:00:00" --EndTime "2026-05-31 23:59:59" 2>&1 | tr -d '\r' \
  | jq -r '"合计: " + (([.SummaryOverview[]?.RealTotalCost|tonumber]|add)|tostring) + " 元", (.SummaryOverview[]? | "\(.BusinessCodeName) | \(.RealTotalCost) 元")'

# 按付费方式（看包年包月 vs 按量计费占比）
tccli billing DescribeBillSummaryByPayMode \
  --BeginTime "2026-05-01 00:00:00" --EndTime "2026-05-31 23:59:59" 2>&1 | tr -d '\r' \
  | jq -r '.SummaryOverview[]? | "\(.PayModeName) | \(.RealTotalCost) 元"'

# 按地域（确认资源/费用集中在哪个 region）
tccli billing DescribeBillSummaryByRegion \
  --BeginTime "2026-05-01 00:00:00" --EndTime "2026-05-31 23:59:59" 2>&1 | tr -d '\r' \
  | jq -r '.SummaryOverview[]? | "\(.RegionName) | \(.RealTotalCost) 元"'
```

## 资源级账单（定位到具体实例）

`DescribeBillResourceSummary` 用 `--Month "YYYY-MM"` + 分页 `--Limit`/`--Offset`，可定位到单个资源的花费——降本分析时用来找「最贵的那几个资源」：

```bash
tccli billing DescribeBillResourceSummary --Month "2026-05" --Limit 100 --Offset 0 2>&1 | tr -d '\r' \
  | jq -r '.ResourceSummarySet[]? | "\(.RealTotalCost)\t\(.BusinessCodeName)\t\(.ResourceName // .ResourceId)"' \
  | sort -t$'\t' -k1 -rn | head -30
```

资源较多时必须分页，否则只看前 100 条可能漏掉目标产品：

```bash
month="2026-05"
tmpfile=$(mktemp)

for offset in $(seq 0 100 5000); do
  page=$(tccli billing DescribeBillResourceSummary --Month "$month" --Limit 100 --Offset "$offset" 2>&1 | tr -d '\r')
  count=$(printf '%s\n' "$page" | jq '.ResourceSummarySet | length')

  printf '%s\n' "$page" \
    | jq -c '.ResourceSummarySet[]?' >> "$tmpfile"

  [ "$count" -lt 100 ] && break
done

jq -sr '.[] | [.RealTotalCost, .BusinessCodeName, (.ResourceName // .ResourceId), .ResourceId] | @tsv' "$tmpfile" \
  | sort -t$'\t' -k1 -rn \
  | head -30
```

按产品过滤资源级账单：

```bash
jq -sr '.[] | select(((.BusinessCodeName // "") + (.ProductCodeName // "") + (.ResourceId // "") + (.ResourceName // "")) | test("负载均衡|CLB|lb-"; "i")) | [.RealTotalCost, .BusinessCodeName, .ProductCodeName, .ResourceId, .ResourceName] | @tsv' "$tmpfile" \
  | sort -t$'\t' -k1 -rn
```

> 提示：某些产品（如 Oceanus 流计算）的资源管理 API 可能因权限/工作空间维度返回空，但账单接口仍能查到其费用与资源名，可作为定位手段。

## 其他常用账单 Action

| 用途 | Action |
|------|--------|
| 按项目汇总 | `tccli billing DescribeBillSummaryByProject --BeginTime ... --EndTime ...` |
| 按标签汇总 | `tccli billing DescribeBillSummaryByTag --BeginTime ... --EndTime ...` |
| 账单明细（最细粒度，含用量） | `tccli billing DescribeBillDetail --Month "2026-05" --Limit 100 --Offset 0` |
| 生成账单下载链接 | `tccli billing DescribeBillDownloadUrl` |
| 节省计划资源信息 | `tccli billing DescribeSavingPlanResourceInfo` |
| 代金券信息 | `tccli billing DescribeVoucherInfo` |

## 费用查询排查

- 金额对不上 → 检查单位：余额是「分」，账单是「元」。
- 鉴权失败 → 临时凭证过期，见 `references/environment-setup.md`。
