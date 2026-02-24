---
description: 从 plan/*.md 生成可维护的 issues CSV 快照（含开发/Review/Git 状态、验收边界、MCP 指定）
argument-hint: "<plan 文件路径（可选，默认取 plan/ 最新）>"

---

你现在处于「Plan → Issues CSV 模式」。

目标：把当前项目的 `plan/*.md`（由 `/prompts:plan` 生成的执行计划）转换为可落盘、可协作维护的 **唯一命名 issues CSV 快照**（`issues/<timestamp>-<slug>.csv`），并确保该 CSV 可以作为代码的一部分提交到仓库中，用于长期追踪任务边界与状态。

> 核心原则：ISSUES CSV 是"会议落盘的任务边界合同"，不是 AI 自嗨文档。
> CSV 要能防止任务跑偏：每条必须明确 **做什么、怎么验收、怎么 review、用什么测试工具/MCP**。

## 一、输入与默认行为

1. `$ARGUMENTS` 允许为空：
   - 若为空：默认选择当前项目 `plan/` 目录下**最新**的 `*.md` 作为输入。
   - 若不为空：视为 `plan` 文件路径（相对/绝对均可）。
2. 你必须读取该 `plan` 文件内容，必要时可根据 `📎 参考` 中的文件路径进一步读取少量上下文（只读、最小必要）。
3. 若找不到 `plan` 文件或内容不足以拆分任务：用 1–2 句话说明原因，并给出你需要的最小补充信息（不要长篇追问）。

## 二、总体行为约定（必须遵守）

1. 你是"任务拆分与落盘助手"，目标是生成**可维护**的 CSV，而不是输出大量散文。
2. 禁止使用百分比进度；所有进度必须使用状态枚举（见「四、状态字段」）。
3. 每条任务必须包含：
   - `acceptance_criteria`：可验证、可测试的验收口径（尽量量化）。
   - `review_initial_requirements`：边开发边 Review 的要求。
   - `review_regression_requirements`：全量完成后的回归/复测要求。
   - `test_mcp`：明确该任务默认用哪个测试执行器/MCP（后端/前端/端到端）。
4. 详细背景与推理不应堆进 CSV：尽量通过 `refs` 指向 `plan/*.md` / 其他审计文档（例如 PERF 审计）来承载细节。
5. 生成后必须将 CSV 写入项目的 `issues/` 目录：
   - 生成一个**唯一命名**的快照文件（便于审计/回溯）。
   - **禁止**创建/更新 `issues/issues.csv`、`issues.csv` 或任何其它固定文件名的"汇总版"（避免误导后续 `/prompts:issues_csv_execute` 的唯一状态源选择）。

## 三、拆分规则（从 plan 到 issues）

将 `plan` 中的 Phase/步骤转换为 issues 行，遵循：

1. 默认粒度：**一条 Phase 对应一条 issues**。
2. 允许拆分：若某个 Phase 同时包含明显独立的多项工作（例如前后端两条链路，或多个接口/模块），可拆分为多行，但要避免把 CSV 拆成"几十条细颗粒 TODO"。
3. 建议规模：一般 5–30 行最易维护；超过 30 行时，优先合并同类项并通过 `notes/refs` 指向细节文档。

## 四、CSV Schema（固定表头）

你必须使用以下表头（字段顺序固定）：

```
id,priority,phase,area,title,description,acceptance_criteria,test_mcp,review_initial_requirements,review_regression_requirements,dev_state,review_initial_state,review_regression_state,git_state,owner,refs,notes
```

字段含义与填写要求：

- `id`：任务唯一标识（建议：`<PREFIX>-000`、`<PREFIX>-010`... 以 10 递增，方便插入）。
- `priority`：建议值 `P0|P1|P2`（如计划有更多级别可扩展，但保持一致）。
- `phase`：来源 Phase 序号（例如 `0`、`1`、`2`；如拆分可用 `2.1`、`2.2`）。
- `area`：建议值 `backend|frontend|both`（可按项目需要扩展，但保持可枚举）。
- `title`：一句话标题（短、可读、可会议讨论）。
- `description`：1–2 句说明"做什么"，强调边界，不写实现细节。
- `acceptance_criteria`：可测试的验收标准（可含指标/阈值/复现步骤）。
- `test_mcp`：该任务默认测试执行器/MCP（见「五、测试执行器/MCP」）。
- `review_initial_requirements`：开发过程中的 Review 要点（例如兼容性、降级、日志、性能）。
- `review_regression_requirements`：最终回归/复测要点（例如故障注入、并发、边界场景）。
- `dev_state`：开发状态（见「六、状态字段」）。
- `review_initial_state`：初次 Review 状态（见「六、状态字段」）。
- `review_regression_state`：回归 Review 状态（见「六、状态字段」）。
- `git_state`：是否已提交到 git（见「六、状态字段」）。
- `owner`：负责人（默认留空，由会议分配后填写）。
- `refs`：引用与跳转（强制要求，使用 `path:line`，多个用 `;` 分隔）。
- `notes`：自由备注（默认留空，可用于 PR/commit/风险记录）。

## 五、测试执行器 / MCP 指定（可按项目调整）

默认约定（如项目另有规范，以项目规范为准）：

- `AUTOSERVER`：服务端/后端测试（接口测试、单测、压测、故障注入等）。
- `AUTOFRONTEND`：前端测试（组件/交互/性能 profile、虚拟化、渲染开销等）。
- `AUTOE2E`：端到端/联调测试（前后端联动、真实链路验证、回归对比等）。

每条任务必须明确填一个 `test_mcp`，以避免"做了但没测"的漂移。

## 六、状态字段（枚举，禁止百分比）

- `dev_state`：`未开始|进行中|已完成`
- `review_initial_state`：`未开始|进行中|已完成`
- `review_regression_state`：`未开始|进行中|已完成`
- `git_state`：`未提交|已提交`

默认值：

- 生成时全部填 `未开始`，`git_state` 填 `未提交`。

## 七、文件命名与编码（必须满足 Excel 与 AI）

1. 目录：确保项目根目录下存在 `issues/`（不存在则创建）。
2. 唯一命名快照（必须创建）：
   - 文件名建议：`issues/YYYY-MM-DD_HH-mm-ss-<slug>.csv`
   - 时间戳优先使用**当前时间**；`<slug>` 可从 plan 文件名或 plan task 提取并归一化。
3. 禁止生成"汇总入口"CSV：
   - 不要创建/更新 `issues/issues.csv` 或 `issues.csv`。
4. 编码：必须写为 **UTF-8 with BOM**（Excel 友好，避免中文乱码）。
   - Windows PowerShell 推荐：使用 `.NET UTF8Encoding($true)` 写文件。
5. 如果文件被 Excel/WPS 打开导致写入失败：提示用户关闭占用进程后重试。

## 八、CSV 输出规范（避免格式坑）

1. 必须输出合法 CSV：
   - 表头一行；
   - 每行字段数与表头一致；
   - 字段内出现逗号/换行/双引号时必须正确转义。
2. 推荐策略（更稳）：**所有字段统一使用双引号包裹**，内部 `"` 用 `""` 转义。
3. `refs` 中的路径必须尽量精确到 `file:line`，便于人类与 AI 直接跳转。

## 九、执行步骤（你需要实际落盘到文件）

你应按以下步骤执行，并在最后给出清晰交付物：

1. 定位并读取输入 `plan` 文件（根据 `$ARGUMENTS` 或默认选择最新）。
2. 从 plan 的 Phase/步骤拆出 issues 行，补齐每行的验收/Review/MCP/refs。
3. 在 `issues/` 下写入唯一命名快照 CSV。
4. 校验：
   - 用 `Import-Csv`（PowerShell）或等价工具验证可解析；
   - 检查状态字段是否只使用枚举值；
   - 检查 `refs` 是否存在且非空。

## 十、对话内输出格式（简洁交接）

完成后，在对话中只输出关键信息：

- 生成的快照路径：`issues/YYYY-MM-DD_HH-mm-ss-<slug>.csv`
- 行数统计（多少条 issues）
- 如有风险/注意事项（例如 BOM、Excel 锁文件）
- 下一步建议命令：`/prompts:issues_csv_execute <上面生成的快照路径>`

若因策略/错误无法写文件：在回答中输出完整 CSV（使用代码块），并说明应写入的目标路径与编码要求（UTF-8 BOM）。
