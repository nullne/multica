# Issue 管理 功能清单（产品视角）

> Issue 是 Multica 里最核心的工作单元——团队（人和 agent）围绕一个个 issue 协作。
> 一个 issue 由三部分组成——**它描述什么**（标题、描述、acceptance criteria、labels）、
> **它的状态与归属**（status、priority、assignee）、**它如何与其他 issue 和成员连接**
> （parent/子 issue、关联 link、subscribers、reactions、activity timeline）。

这是一份自顶向下、面向用户能力的功能清单，用来核对产品功能是否齐全。

## 1. Issue 是什么

- [ ] 用户可以创建一个 issue：至少填写标题，可附带详细描述。
- [ ] 每个 issue 在创建时自动获得一个 workspace 内可读的编号（带 prefix 的 issue number，如 `ENG-12`），用于在团队里互相引用。
- [ ] issue 编号在 workspace 内唯一且稳定，不随删改而变化。
- [ ] 用户可以打开任意 issue 查看其完整详情。
- [ ] 用户可以编辑 issue 的标题与描述。
- [ ] 用户可以删除不再需要的 issue。

## 2. 状态与优先级

- [ ] 每个 issue 有一个 status，反映它在工作流中的阶段（如待办、进行中、已完成、已取消等）。
- [ ] 用户可以随时切换 issue 的 status。
- [ ] 每个 issue 有一个 priority（如无、低、中、高、紧急）。
- [ ] 用户可以调整 issue 的 priority。
- [ ] 当 agent 在某个 issue 上执行任务时，issue 的 status 会随任务生命周期自动同步（开始执行 / 完成 / 失败）。

## 3. Assignee（指派给人或 agent）

- [ ] 用户可以把 issue 指派给一个 assignee。
- [ ] assignee 可以是 workspace 的一个 member（人）。
- [ ] assignee 也可以是一个 agent（AI 队友）——agent 是一等公民，可以像成员一样被指派工作。
- [ ] 界面会区分人和 agent 两类 assignee（agent 用独特样式标识）。
- [ ] 用户可以更换或清空 issue 的 assignee。
- [ ] 把 issue 指派给 agent，是触发 agent 开始工作的方式之一。

## 4. 内容字段：描述、acceptance criteria、labels

- [ ] issue 支持一段富文本/markdown 形式的描述，说明要做什么。
- [ ] issue 支持 acceptance criteria（验收标准），用来明确「做到什么程度算完成」。
  - [ ] 验收标准可以编辑、增删条目。
  - [ ] 验收标准有审批状态：当 agent 提出标准后，需由人 approve 才算确认。
- [ ] 用户可以为 issue 添加一个或多个 label，用于分类和筛选。
- [ ] 用户可以从 issue 上移除 label。

## 5. 层级：parent 与子 issue

- [ ] 用户可以为一个 issue 指定 parent issue，从而组织出父子层级。
- [ ] 用户可以在一个 issue 下查看它的子 issue 列表。
- [ ] 用户可以把已有 issue 挂到某个 parent 下，或从 parent 下摘除。

## 6. Issue 之间的关联（link）

- [ ] 用户可以在两个 issue 之间建立关联（link），表达它们之间的关系（如相关、重复、阻塞等）。
- [ ] 用户可以在 issue 详情里查看与它关联的其他 issue。
- [ ] 用户可以移除一条已有的关联。

## 7. Subscribers（订阅与关注）

- [ ] 用户可以订阅一个 issue，从而在它有更新时收到通知。
- [ ] 用户可以取消订阅。
- [ ] 与 issue 有交互的成员（如创建者、assignee、参与评论者）会被纳入订阅，便于持续跟进。
- [ ] 用户可以查看一个 issue 的 subscribers 列表。

## 8. Reactions（表情回应）

- [ ] 用户可以对 issue 添加 emoji 表情回应。
- [ ] 用户可以撤回自己的表情回应。
- [ ] 界面会聚合显示某个表情被哪些人/agent 回应过及其数量。

## 9. Activity timeline（动态时间线）

- [ ] 每个 issue 有一条按时间排列的动态时间线，记录它发生过的变化。
- [ ] 时间线会记录关键变更，如 status 变化、priority 变化、assignee 变化、label 变化、层级与关联变化等。
- [ ] 评论也会出现在同一条时间线里，与活动记录交织呈现完整脉络。
- [ ] 时间线会标明每条记录由谁（人或 agent）在何时触发。

## 10. 视图：列表、board 与 my-issues

- [ ] 用户可以在 issue 列表里浏览 workspace 内的所有 issue。
- [ ] 用户可以在 board（看板）视图里按 status 分列查看 issue。
- [ ] 用户可以在 board 上通过拖拽改变 issue 的 status。
- [ ] 用户可以在 my-issues 视图里集中查看指派给自己的 issue。
- [ ] 各视图支持按 status、priority、assignee、label 等维度筛选与组织。

## 11. 实时协作

- [ ] issue 的变更会实时同步给正在查看的其他成员，无需手动刷新。
- [ ] 当 agent 在 issue 上推进工作时，相关状态与动态会实时反映在界面上。

## 12. 权限与 workspace 隔离

- [ ] 所有 issue 都归属于某个 workspace，用户只能看到并操作自己所属 workspace 内的 issue。
- [ ] 访问任何 issue 前都会校验用户是否为该 workspace 的成员。
- [ ] 跨 workspace 无法读取或修改对方的 issue、label、关联和动态。
- [ ] assignee、subscriber、关联等只能在同一 workspace 范围内选择。

## 相关文档

- [comments.md](./comments.md) — 评论、@提及、回复与表情回应
- [labels.md](./labels.md) — Labels（标签）
- [agents.md](./agents.md) — Agents（AI 队友）与触发方式
- [routines.md](./routines.md) — Routines（自动创建 issue）
