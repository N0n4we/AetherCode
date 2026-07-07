---
name: tencent-cloud-governance
description: 使用 tccli/coscli 查询本账号腾讯云资源、COS、账单和成本线索；用于资源盘点、巡检、费用分析和降本建议。
---

# 腾讯云治理 (tccli + coscli)

用 `tccli` / `coscli` 查询本账号腾讯云资源、账单与成本线索。顶层只保留边界和入口；命令细节见对应模块。

## 运行环境

- 工具环境固定在 `~/.tccli/venv`。
- 如果环境不存在，必须先创建环境，再处理用户请求。
- `tccli` 安装在该虚拟环境里，`coscli` 放在 `~/.tccli/venv/bin/coscli`，`jq` 放在 `~/.tccli/venv/bin/jq`。
- 执行 `tccli` / `coscli` / `scripts/coscli-auth.sh` 前，先运行 `source ~/.tccli/venv/bin/activate`。
- 登录与故障处理见 `references/environment-setup.md`。

## 使用原则

- 先理解用户真正要的结果：盘点、定位某个资源、查账单、解释费用、给降本建议，或执行前核验。
- 优先使用真实账单和资源 API；已有本地快照可以复用，但要说明快照日期。
- 只读查询可以主动执行；释放、删除、降配、改计费方式、改网络入口等会影响资源的动作，只能给建议或在用户明确确认后执行。
- 不要被下面的参考文档绑死。它们是命令手册和检查清单，不是固定流程。

## 常用入口

- 环境部署：`references/environment-setup.md`
- 资源查询命令：`references/resource-query.md`
- 云监控查询命令：`references/monitor-query.md`
- 资源控制与变更执行：`references/resource-control.md`
- 账单查询命令：`references/cost-query.md`
- 成本分析提示：`references/cost-analysis.md`
- 优化建议提示：`references/cost-optimization.md`
- 查询离线归档：位于仓库里的 `docs/resource-audit/`
