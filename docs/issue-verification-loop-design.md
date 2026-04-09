# Issue 双 Agent 验收闭环设计（MUL-12）

## 背景

当前 issue 的 agent 流程是「指派 -> 执行 -> agent 自行更新状态」，缺少独立验收环节。  
MUL-12 目标是在指定了验证 agent 的前提下，形成自动闭环：

1. 执行 agent 开始前，先由验证 agent 产出验收标准。
2. 执行 agent 完成后，自动触发验证 agent 按标准验收。
3. 若不通过，系统自动反馈并 @ 执行 agent 继续修复。
4. 重复直到验收通过，最终完成交付。

## 目标

- 支持在 issue 上配置一个「验证 agent」（可选）。
- 在「执行」之前强制产生结构化验收标准（当验证 agent 已配置时）。
- 验证失败时自动触发下一轮执行，形成无需人工搬运信息的循环。
- 验证结论可追踪（谁验收、哪一轮、失败点是什么）。
- 不配置验证 agent 时，保持当前行为不变。

## 非目标

- 本期不做复杂 SLA / 审批流（例如多级人工签字）。
- 本期不做多验证 agent 并行投票。
- 本期不改造通用任务队列为全新工作流引擎（基于现有 `agent_task_queue` 扩展）。

## 当前系统现状（与本设计相关）

- 任务编排核心在 `server/internal/service/task.go`。
- issue 指派到 agent 后会直接 enqueue 执行任务（`on_assign`）。
- `issue.acceptance_criteria` 字段已存在于数据库，但尚未进入 API / 编排主流程。
- `agent_task_queue.context` 字段已存在，可承载任务级元信息（当前几乎未使用）。
- agent 完成任务后的自动评论路径为 `TaskService.CompleteTask -> createAgentComment`。

## 核心设计

### 1) 数据模型

#### 1.1 `issue` 表新增字段

- `verifier_agent_id UUID NULL REFERENCES agent(id) ON DELETE SET NULL`
- `verification_phase TEXT NOT NULL DEFAULT 'none'`
  - 枚举建议：`none | criteria_pending | execution_pending | validating | rework_pending | passed | blocked`
- `verification_round INT NOT NULL DEFAULT 0`
- `last_verification_result JSONB NOT NULL DEFAULT '{}'`

说明：

- 继续复用现有 `acceptance_criteria JSONB` 存放最终验收标准。
- 不单独新建 session 表，先用 issue 级状态满足闭环；后续如需审计增强再拆分。

#### 1.2 `agent_task_queue.context` 用法标准化

为验证流任务写入结构化上下文（JSON）：

```json
{
  "flow": "verification_loop",
  "role": "criteria|executor|validator|rework",
  "round": 1,
  "executor_agent_id": "<uuid>",
  "verifier_agent_id": "<uuid>",
  "source_task_id": "<uuid>"
}
```

用于在任务完成时判断下一步编排动作，避免仅靠评论文本推断。

### 2) 状态机

`none`（未启用验证）走现有流程，不变。

启用验证时：

1. 配置 verifier 后进入 `criteria_pending`
2. 标准产出成功后进入 `execution_pending` 并触发执行任务
3. 执行任务完成后进入 `validating` 并触发验证任务
4. 验证通过 -> `passed`
5. 验证失败 -> `rework_pending`，自动触发 rework 执行任务，再回到 `validating`
6. 连续失败超过阈值（例如 5 轮）-> `blocked`

## 编排流程（详细）

### A. 指派/更新 issue 时

当满足以下条件：

- `assignee_type=agent`
- `assignee_id` 有效（执行 agent）
- `verifier_agent_id` 有效

则改为：

1. 取消该 issue 当前活跃任务（沿用现有取消逻辑）
2. 若 `acceptance_criteria` 为空：enqueue `role=criteria` 到 verifier，phase 设为 `criteria_pending`
3. 若 `acceptance_criteria` 非空：直接 enqueue `role=executor` 到 assignee，phase 设为 `execution_pending`

约束：

- verifier 不能等于 assignee（返回 400）。
- verifier 必须是可访问、未归档、有 runtime 的 agent。

### B. `role=criteria` 任务完成

1. 解析输出中的结构化 criteria（见下节输出协议）。
2. 写入 `issue.acceptance_criteria`。
3. 生成一条「验收标准已确认」评论（可由 verifier 身份发布）。
4. enqueue `role=executor` 给执行 agent，phase -> `execution_pending`。

若解析失败或任务失败：

- phase -> `blocked`
- 自动评论说明失败原因并 @ verifier（便于人工干预）。

### C. `role=executor|rework` 任务完成

如果 issue 仍配置 verifier：

1. enqueue `role=validator` 给 verifier（携带 round 与 source_task_id）。
2. phase -> `validating`。

如果未配置 verifier：保持现有完成行为（兼容旧流程）。

### D. `role=validator` 任务完成

1. 解析结构化验收结果（`pass|fail` + failed checks + summary）。
2. 写入 `last_verification_result`。
3. 若 `pass`：
   - phase -> `passed`
   - 自动评论「验收通过」
   - 建议自动置状态：
     - 若当前是 `in_review`，转为 `done`
     - 其他状态不强制覆盖
4. 若 `fail`：
   - `verification_round += 1`
   - 自动评论验收未通过，并 `@执行 agent`
   - 自动 enqueue `role=rework` 给执行 agent
   - phase -> `rework_pending`
   - 若超过最大轮次，phase -> `blocked` 并停止自动重试

## Prompt 与输出协议

为避免解析自然语言歧义，对 `criteria` 与 `validator` 任务引入「机器可解析块」：

### 1) Criteria 任务输出约定

输出中必须包含：

```text
<!--multica:criteria
{"criteria":[{"id":"AC-1","title":"...","check":"...","severity":"must"}]}
-->
```

### 2) Validator 任务输出约定

输出中必须包含：

```text
<!--multica:verification
{"decision":"pass|fail","summary":"...","failed_checks":[{"id":"AC-1","reason":"..."}]}
-->
```

说明：

- 注释块之外仍允许自然语言总结，供人类阅读。
- 服务端只解析注释块 JSON，解析失败按「验证任务失败」处理。

## API 与 CLI 变更

### API

- `GET /api/issues/:id`、`GET /api/issues` 返回新增字段：
  - `verifier_agent_id`
  - `verification_phase`
  - `verification_round`
  - `acceptance_criteria`
  - `last_verification_result`
- `POST/PUT /api/issues` 支持写入 `verifier_agent_id`（可置空）。

### CLI

- `multica issue get` 增加上述字段展示（json 原样透出）。
- `multica issue update <id>` 新增：
  - `--verifier <agent-name>`
  - `--clear-verifier`

## 与现有行为的兼容性

- `verifier_agent_id` 为空时：完全走现有逻辑。
- 已存在 issue 数据默认 `verification_phase='none'`，无迁移风险。
- comment trigger / mention trigger 保持原语义，但验证闭环不依赖 mention 才能推进（避免自动评论路径遗漏 trigger）。

## 失败与幂等策略

- 所有「任务完成 -> 下一步 enqueue」在事务中执行，并对 issue 行加锁（`SELECT ... FOR UPDATE`）。
- 对同一 issue + 同一 round + 同一 role 做去重检查（避免重复 enqueue）。
- 验证失败自动重试有上限（建议 5）；超限后置 `blocked` 并停止自动循环。

## 实现拆分建议

### Phase 1（后端最小闭环）

- DB migration + sqlc + handler/CLI 字段打通
- `TaskService` 中按 `context.role` 编排
- prompt 注入与结构化输出解析

### Phase 2（可视化与易用性）

- Web issue 详情页展示 verifier、criteria、phase、round
- 在创建/编辑 issue 的 UI 增加 verifier 选择器

### Phase 3（增强）

- 失败轮次指标、告警、审计视图
- 可配置最大轮次/自动状态策略

## 测试计划

- 单测：
  - verifier 校验（不存在、归档、与 assignee 相同）
  - 输出解析（有效 JSON、缺失块、坏 JSON）
  - 状态机跳转正确性
- 集成测试：
  - criteria -> execute -> validate(pass)
  - criteria -> execute -> validate(fail) -> rework -> validate(pass)
  - 连续 fail 超阈值进入 blocked
- 回归测试：
  - 未配置 verifier 的 issue 行为与当前一致
  - comment trigger / mention trigger 行为不回归

## 待确认决策

1. 验收通过后是否统一自动改为 `done`（当前建议仅在 `in_review` 时自动转 `done`）。
2. 默认最大自动修复轮次是否固定为 5，还是做 workspace 级配置。
3. 验证失败评论的作者身份使用 verifier agent 还是 system（建议 verifier，更符合语义）。

