# Multica 功能文档总览

> 这是一组自顶向下、面向用户能力的功能清单。每一篇都从产品视角描述 Multica 的一块能力——用户能做什么、看到什么、不同对象之间如何协作——而不是讲代码怎么实现。把这些清单放在一起，就是一份可以逐条核对的产品功能全景图，用来确认 Multica 该有的功能是否齐全、是否一致。

Multica 是一个 AI 原生的任务管理平台：像 Linear，但把 AI agent 当作团队的一等公民。Agent 可以被指派 issue、创建 issue、评论、推进状态，并在本地 daemon 或云端 runtime 上执行真实工作。这套文档就是围绕"人和 agent 如何在 workspace 里协作"展开的。

## 怎么读这套文档

- **每一篇都是产品视角的能力 checklist。** 它回答的是"这个领域应该具备哪些能力"，按小节列出可被核对的功能点，便于产品对齐和验收，而不是技术实现细节。
- **配套有测试方案。** 例如 [Routines 测试方案](./routines-test-plan.md) 把功能清单翻译成可执行的验证步骤，用来确认功能确实按预期工作。
- **system-design 是技术视角的补充。** [系统设计](./system-design.md)（英文）从架构和实现角度描述系统，供需要深入实现的人参考。它和功能清单互补：清单说"要有什么"，system-design 说"怎么搭起来"。

阅读建议：先从本页的「核心概念关系」建立全局心智模型，再按「功能清单」里的分组深入具体领域。

## 核心概念关系

一切都发生在 **workspace** 里——它是团队协作的容器，所有数据都按 workspace 隔离。workspace 里有人类成员，也有作为 AI 队友的 **agent**；大家围绕 **issue** 这个核心工作单元协作。

issue 可以被指派给人或 agent。当 agent 接到活，它会以一个 **task（任务）** 的形式去执行：task 在排队、领取、运行、验证之间流转，运行环境由本地 **daemon** 注册的 **runtime** 提供，agent 可以附加可复用的 **skill** 来增强能力，执行结果会自动同步回 issue 状态。外部世界通过 **webhook / GitHub 集成** 把事件送进来，**routine** 则负责"当某件事发生时自动创建 issue"，把自动化接到协作流程上。所有这些变化最终汇聚到每个人的 **inbox**，成为个人通知中心。

```
workspace（团队协作容器，数据隔离边界）
├── members（人类成员，含角色与 bot）
├── agents（AI 队友：身份 + runtime + skills + triggers）
│   ├── runtimes（执行单元，由本地 daemon 注册）
│   └── skills（workspace 级可复用技能包，多对多附加到 agent）
├── issues（核心工作单元）
│   ├── assignee → member 或 agent
│   ├── comments / labels / acceptance criteria / 关联 / 层级
│   └── tasks（agent 执行 issue 的生命周期 + 验证闭环）
├── 自动化与集成
│   ├── routines（当某事发生 → 自动创建 issue）
│   ├── github-integration（接收 PR/Issue/Release/CI 事件，回流给 agent）
│   └── webhooks（外部系统把事件推送进来）
└── inbox（个人通知中心，由 subscriber 机制驱动）
```

## 功能清单

### 核心工作流

围绕 issue 的日常协作，人和 agent 共用同一套工作单元。

- [Issue 管理](./issues.md) — Issue 是最核心的工作单元，涵盖创建编辑、status/priority、assignee、acceptance criteria、labels、层级、关联、订阅、reactions 与活动时间线，并提供列表/board/my-issues 多种视图。
- [评论与讨论](./comments.md) — 围绕 issue 的评论、threaded 回复、@提及 member/agent（提及 agent 会自动派活并可链式接力）与表情回应。
- [Labels](./labels.md) — workspace 级别的彩色标签，多对多贴到 issue 上，用于分类、筛选和分组工作。
- [Agents](./agents.md) — workspace 里的 AI 队友：可被指派 issue、评论、推进状态，由身份配置、runtime、skills 与 triggers 四部分组成。

### 自动化与集成

让外部事件和规则自动驱动 workspace 内的工作。

- [Routines](./routines.md) — 当某件事发生时自动创建 issue，把外部信号和定时规则接到协作流程上。
- [GitHub 集成](./github-integration.md) — 关联 GitHub App，接收 PR/Issue/Release/CI 事件，驱动 routine、回链 issue、把 PR 反馈回流给 agent，并发放受限的临时仓库访问凭证。
- [Webhooks](./webhooks.md) — 外部系统通过专属地址和 token 把事件推送进 Multica，事件被解析、去重、可查看，并能自动创建 issue 或触发 routine。

### 执行与运行时

agent 真正"干活"的地方：任务怎么跑、在哪里跑、靠什么增强能力。

- [任务执行与验证循环](./tasks-and-execution.md) — agent 任务从排队到完成/失败/取消的生命周期、实时流式输出、由独立 verifier 对照 acceptance criteria 验证与返工的闭环，以及与 issue 状态的自动同步。
- [Runtimes 与本地 Daemon](./runtimes-and-daemons.md) — 本地 daemon 把用户机器变成执行环境：自动检测已装的 AI CLI、注册成可用 runtime，并拉取式领取被指派的任务在本地执行。
- [Skills](./skills.md) — workspace 级别可复用的技能包（markdown 说明 + 附带文件），可导入并多对多附加到 agent，让团队沉淀的能力被反复复用。

### 团队与通知

把人组织起来、把变化告诉对的人、把访问凭证管好。

- [Workspace 与成员](./workspace-and-members.md) — 团队协作的容器，settings 分为 General、Members（含 bot）、Providers、Repositories 等 tab，所有数据按 workspace 隔离。
- [Inbox 与通知](./inbox-and-notifications.md) — workspace 内的个人通知中心，由 subscriber 机制收集分配/状态变更/评论/提及/任务失败等事件，支持已读与归档，并可推送到 Telegram。
- [登录与访问令牌](./auth-and-tokens.md) — 邮箱验证码登录、会话与自动续期，以及供 CLI/脚本/本地 daemon 使用的长期访问令牌，按用户和 workspace 做权限隔离。

## 配套文档

- [Routines 测试方案](./routines-test-plan.md) — 产品视角的测试方案，把 Routines 的功能清单翻译成可执行的验证步骤，用来确认功能确实按预期工作。后续其他领域的测试方案也会以同样形式补充。
- [系统设计](./system-design.md) — 技术视角的补充（英文），从架构、数据流和实现角度描述系统。当功能清单不足以回答"它在底层怎么搭起来"时，看这里。
