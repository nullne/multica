# Agents 功能清单（产品视角）

> Agent 是 workspace 里的 AI 队友：像成员一样能被指派 issue、留下评论、推进状态，
> 但它的工作由 AI 自动完成。一个 agent 由四部分组成——**身份与配置**（名字、模型、说明）、
> **运行它的 runtime**、**它会用到的 skills**，以及**什么时候自动触发它**（trigger）。

这是一份自顶向下、面向用户能力的功能清单，用来核对产品功能是否齐全。

## 1. Agent 是什么

- [ ] 用户可以在 workspace 里创建一个 agent，把它当作 AI 队友使用。
- [ ] 一个 agent 拥有自己的身份：名字、可选的描述、用于在界面上识别它的头像与颜色。
- [ ] agent 在界面上与人类成员区分显示（独特的样式与机器人图标），让用户一眼看出这是 AI。
- [ ] 用户可以随时编辑 agent 的配置，也可以删除不再需要的 agent。
- [ ] 用户可以浏览 workspace 内所有 agent 的列表。

## 2. Agent 的配置

- [ ] 用户可以为 agent 选择背后的 AI 能力（如 Claude Code、Codex 等提供方）以及具体模型。
- [ ] 用户可以给 agent 写一段 instructions，定义它的角色、工作方式和约束，agent 在执行任务时会遵循这段说明。
- [ ] 用户可以设置 agent 的并发度：限制它最多同时处理多少个任务，避免一次接太多活。
- [ ] 用户可以启用或停用一个 agent；停用后它不再接收新任务，但配置仍然保留。

## 3. 绑定 Runtime

- [ ] 用户可以把 agent 绑定到一个 runtime，决定它实际在哪里运行（如本地 daemon 或云端运行时）。
- [ ] agent 只有在拥有可用 runtime 时才能真正执行任务；没有可用 runtime 时不会接活。
- [ ] 同一个 workspace 可以有多个 agent，分别绑定到不同的 runtime。

## 4. 附加 Skills

- [ ] 用户可以给 agent 附加一个或多个 skills，扩展它在执行任务时可调用的可复用能力。
- [ ] 用户可以调整 agent 已附加的 skills 组合，按需增减。
- [ ] 不同 agent 可以拥有各自不同的 skills 组合，从而擅长不同类型的工作。

## 5. 触发方式（Triggers）

- [ ] 用户可以为 agent 配置一个或多个 trigger，定义它在什么情况下被自动唤起去工作。
- [ ] **on_assign**：当一个 issue 被指派给该 agent 时，自动开始处理。
- [ ] **on_comment**：当 issue 上出现新的评论（如有人 @ 它或追加需求）时，自动响应。
- [ ] **scheduled**：按设定的时间计划自动运行，无需人工触发。
- [ ] 没有配置任何 trigger 的 agent 不会被自动唤起，可作为仅手动使用的队友。

## 6. 作为 Assignee 出现在协作中

- [ ] agent 可以像成员一样被指派为 issue 的 assignee，在 board 与列表中正常显示。
- [ ] 被指派的 agent 在 issue 上以可识别的 AI 样式呈现，与人类 assignee 区分。
- [ ] agent 在完成工作时可以推进 issue 的状态、留下评论，整个过程对团队可见。
- [ ] 用户可以把一个 issue 在人类成员与 agent 之间重新指派。

## 7. Agent 个人页

- [ ] 每个 agent 有自己的个人页，集中展示它的身份、配置、绑定的 runtime、skills 与 triggers。
- [ ] 用户可以从个人页直接查看并修改该 agent 的设置。
- [ ] 个人页帮助用户了解这个 AI 队友"是谁、会做什么、什么时候会动手"。

## 8. 权限与 workspace 隔离

- [ ] agent 归属于某个 workspace，只有该 workspace 的成员能看到和管理它。
- [ ] agent 的所有活动（被指派、评论、推进状态）都限定在它所属的 workspace 内，不会跨 workspace。
- [ ] 创建、配置、删除 agent 需要相应的 workspace 成员权限。
- [ ] agent 只能访问和操作其所属 workspace 内的 issue 与数据。

## 相关文档

- [issues.md](./issues.md) — Issue 管理（agent 处理的核心工作单元）
- [skills.md](./skills.md) — Skills（可附加给 agent 的可复用技能）
- [runtimes-and-daemons.md](./runtimes-and-daemons.md) — Runtimes 与本地 Daemon（agent 的运行环境）
- [tasks-and-execution.md](./tasks-and-execution.md) — 任务执行生命周期与验证循环
