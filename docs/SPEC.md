# Multica 平台规范

Status: Draft v1（语言无关）

Purpose: 定义一个把 AI agent 当作一等团队成员的任务管理平台——人和 agent 在同一个 workspace 里围绕 issue 协作，agent 可被指派、评论、推进状态，并在本地 daemon 或云端 runtime 上执行真实工作。

## 规范用语

本文中的 `MUST`、`MUST NOT`、`REQUIRED`、`SHOULD`、`SHOULD NOT`、`RECOMMENDED`、`MAY`、`OPTIONAL` 按 RFC 2119 解释。

`Implementation-defined` 表示行为属于实现契约，但本规范不规定唯一策略；实现 MUST 明确记录所选行为。

为便于对照，保留以下英文术语不译：`workspace`、`issue`、`agent`、`runtime`、`daemon`、`task`、`skill`、`routine`、`trigger`、`assignee`、`subscriber`、`inbox`、`label`、`webhook`。

## 1. 问题陈述

Multica 是一个多租户任务管理平台（形似 Linear），但把 AI agent 提升为一等参与者。它解决四个问题：

- 让 agent 像人类队友一样被指派 `issue`、留下评论、推进状态，而不是靠手工粘贴 prompt。
- 把 agent 的执行编排成可靠的任务生命周期（排队 → 领取 → 运行 → 完成/失败），并实时回流进度。
- 把团队的可复用能力沉淀为 `skill`，把重复性工作沉淀为 `routine`（当某件事发生时自动创建 `issue`）。
- 用统一的 runtime 视图管理本地与云端算力，用 `inbox` 把变化告诉对的人。

重要边界：

- 服务端是编排者与事实来源（issue/task 状态、通知、活动日志）。
- 真正的代码工作由 agent 在其 runtime 内完成；服务端不内置"如何改代码"的业务逻辑。
- 任务成功不等于 issue 完成；issue 可停在工作流定义的交接状态（如 `in_review`）。

## 2. Goals 与 Non-Goals

### 2.1 Goals

- 以 `workspace` 为租户边界，对所有数据做强隔离。
- 用单一权威的服务端状态驱动 issue 与 task 的生命周期、状态同步与广播。
- 用拉取式（pull-based）派发把 task 交给本地 daemon，NAT 友好、无需反向连接。
- 用事件总线解耦副作用（订阅、通知、活动日志）。
- 支持 member 与 agent 的多态 `assignee`，二者共用同一套协作原语。
- 提供面向人的可观测性（实时 WebSocket、活动时间线、结构化日志）。
- 允许把重复工作自动化（`routine`）并接入外部事件（GitHub、`webhook`）。

### 2.2 Non-Goals

- 不做分布式作业调度器或通用工作流引擎。
- 不规定 agent 内部如何编辑代码、提交 PR 或运行命令。
- 不强制单一的审批/沙箱策略；执行侧安全由 agent CLI 与宿主机提供。
- 不提供跨 workspace 的共享或聚合视图。
- 不要求持久化 agent 的中间推理过程，只持久化任务结果与流式消息。

## 3. 系统总览

### 3.1 主要组件

1. `HTTP API`（Chi 路由）
   - 公共路由：认证、健康检查、WebSocket 升级。
   - 受保护路由：需 JWT/PAT，按 `workspace` 鉴权。
   - daemon 路由：独立认证模型（daemon token），不走用户 JWT。

2. `Task Service`
   - 编排 agent 工作：enqueue → claim → start → complete/fail。
   - 自动同步 issue 状态，并在每次状态转移广播事件。
   - 驱动 OPTIONAL 的验收验证闭环。

3. `Realtime Hub`
   - 管理 WebSocket 客户端，按 `workspace_id` 分房间广播。
   - 服务端向客户端单向广播；入站消息路由为实现保留项。

4. `Event Bus`
   - 服务端内部事件总线，解耦 handler 与副作用监听者（订阅、通知、活动日志）。

5. `Daemon Runtime`（本地）
   - 自动探测可用 CLI（claude、codex 等），注册为 `runtime`。
   - 轮询服务端领取 task，按 provider 路由执行，回报结果与流式消息。

6. `Agent SDK`
   - 统一的 `Backend` 接口，为每种 CLI 各起一个进程，经 `Session.Messages` 与 `Session.Result` 通道流式返回。

7. `Integrations`
   - GitHub App、`webhook` 接入，把外部事件转成 `routine` 触发与 issue 回链。

8. `Web App`（Next.js）
   - 路由层薄壳 + 按领域组织的 feature 模块；Zustand 管理客户端状态。

### 3.2 抽象分层

1. `身份与租户层`：user、membership/role、workspace 隔离。
2. `协作层`：issue、comment、label、subscriber、activity——人与 agent 共用。
3. `执行层`：task 生命周期、verification 闭环、runtime/daemon。
4. `能力层`：skill（可复用）、agent 配置与 trigger。
5. `自动化与集成层`：routine、GitHub、webhook。
6. `通知与可观测层`：inbox、notification channel、WebSocket、结构化日志。

### 3.3 外部依赖

- PostgreSQL（含 `pgvector` 扩展）作为持久化存储；sqlc 生成查询代码。
- 编码 agent 的可执行 CLI（claude、codex 等），由 daemon 在宿主机启动。
- OPTIONAL：GitHub App、Telegram Bot、SMTP（验证码邮件）。
- 宿主机对 issue tracker / 代码仓库 / agent 的鉴权由部署环境提供。

## 4. 核心领域模型

### 4.1 实体

#### 4.1.1 Workspace

租户边界。所有业务数据按 `workspace_id` 隔离。

字段（逻辑）：
- `id` (uuid)
- `slug` (string，URL 友好，workspace 内唯一)
- `name` (string)
- `settings` (JSONB，含 provider config)
- `repos` (JSONB，关联的代码仓库)

#### 4.1.2 User 与 Member

- `User`：全局身份，邮箱登录，可属于多个 workspace。
- `Member`：`user` ↔ `workspace` 的绑定，带 `role`（`owner` / `admin` / `member`）。
- `Bot User`：代表 agent 的特殊用户身份，用于以 agent 名义产生评论/活动。

#### 4.1.3 Issue

最核心的工作单元。

字段（逻辑）：
- `id` (uuid)
- `number` (integer，workspace 内自增) + `prefix` (string，如 `ABC`)
- `title` (string)
- `description` (string 或 null)
- `status` (enum，见 §6.1)
- `priority` (enum：`no_priority` / `urgent` / `high` / `medium` / `low`)
- `assignee_type` (enum：`member` / `agent` 或 null) + `assignee_id` (uuid 或 null)
- `parent_id` (uuid 或 null，子 issue 指向父)
- `acceptance_criteria` (string 或 null，可触发 verification)
- `labels` (多对多)
- `created_at` / `updated_at`

#### 4.1.4 Comment

issue 下的线程化讨论。

字段（逻辑）：
- `id` (uuid)、`issue_id`、`workspace_id`
- `author_type` (enum：`member` / `agent`) + `author_id`
- `body` (string，非空)
- `parent_id` (uuid 或 null，回复指向父评论)
- `created_at` / `updated_at`

#### 4.1.5 Label

workspace 级标签，与 issue 多对多。

字段：`id`、`name`、`color`、`workspace_id`。

#### 4.1.6 Agent

可被指派 issue 的 AI 队友。多态 assignee 之一。

字段（逻辑）：
- `id`、`workspace_id`、`name`、`description`
- `provider` (枚举，背后 CLI) + `model`
- `instructions` (string，角色与约束)
- `runtime_ref` (绑定的 runtime)
- `skills` (多对多)
- `triggers` (`on_assign` / `on_comment` / `scheduled`)
- `max_concurrency` (integer，默认见 §8 cheat sheet)
- `enabled` (boolean)

#### 4.1.7 Skill

workspace 级可复用技能包。

字段：`id`、`workspace_id`、`name`、`content` (markdown)、`files` (OPTIONAL 附件)；与 agent 多对多。

#### 4.1.8 Daemon

运行在某台机器上的本地进程，一等 DB 实体。

字段（逻辑）：
- `id`、`workspace_id`
- `status` (`online` / `offline`)
- `cli_version`、`device_info`
- `last_heartbeat_at`

#### 4.1.9 Runtime

一个已注册的 agent CLI 实例，归属某个 daemon。

- 标识：`(workspace_id, daemon_id, provider)`。
- `daemon_ref` (FK → daemon)。
- Runtime Usage：按模型按天聚合的 token 计数。

#### 4.1.10 Task

派发给 agent、针对某个 issue 的一次执行。

字段（逻辑）：
- `id`、`workspace_id`、`issue_id`、`agent_id`
- `status` (enum，见 §9.1)
- `role` (OPTIONAL：`criteria` / `validator` / `rework`，用于验证闭环)
- `trigger` (OPTIONAL：`assign` / `mention` / `schedule` 等触发来源)
- `trigger_comment_id` (OPTIONAL，提及触发时记录来源评论)
- `session_id`、`work_dir`、`result`、`error`
- `dispatched_at` / `started_at` / `completed_at`

#### 4.1.11 Task Message

运行中 task 的流式输出。

字段：`task_id`、`kind`（`text` / `tool_use` / `tool_result` / `error`）、`payload`、`created_at`。

#### 4.1.12 Routine

把重复工作自动化的规则：当 trigger 命中时按模板创建（或评论）issue。

- `id`、`workspace_id`、`enabled`
- 一个 routine 可配置多个 `trigger`（见 §12.2）。
- action 配置：issue 模板（title/description/priority/assignee/due/labels/subscribers/dispatch）。

#### 4.1.13 Inbox Item

路由给某个 user 的通知。

字段：`id`、`workspace_id`、`recipient_id`、`type`（`issue_assigned` / `status_changed` / `new_comment` / `mention` / `task_failed` 等）、`severity`（`info` / `action_required`）、`read_at`、`details` (JSONB)。

#### 4.1.14 Subscriber

关注某个 issue 的 user/agent。在创建、指派、评论时自动订阅，驱动 inbox 通知。

#### 4.1.15 Activity Log

issue 的审计条目（created、status_changed、assigned 等），与 comment 合并成统一时间线。

#### 4.1.16 凭证

- `Verification Code`：邮箱 OTP，登录时若用户不存在则即时创建。
- `Personal Access Token (PAT)`：`mul_` 前缀的长期令牌，user 级，供 CLI/API 使用。
- `Daemon Token`：`mdt_` 前缀，workspace 级，供 daemon ↔ 服务端认证。

### 4.2 标识与规范化规则

- `Issue Identifier`：以 `<prefix>-<number>` 组成人类可读键（如 `ABC-123`）。
- `Runtime Identity`：以 `(workspace_id, daemon_id, provider)` 唯一确定一个 runtime。
- `Session ID`：由 agent 后端返回，关联一次 agent 线程，用于断点续跑。
- `Assignee`：`assignee_type` + `assignee_id` 组成多态引用；agent 与 member 在 UI 上必须可区分。
- 多租户：所有查询 MUST 按 `workspace_id` 过滤；访问前 MUST 做 membership 检查。

### 4.3 关系

```
Workspace
├── Members ← User（role: owner/admin/member）
├── Issues
│   ├── Comments（线程化，author = member 或 agent）
│   ├── Subscribers（自动管理）
│   ├── Labels（多对多）
│   ├── Reactions
│   ├── Activity Log
│   └── Tasks
│       └── Task Messages（流式输出）
├── Agents
│   ├── → Runtime（绑定其一）
│   ├── → Skills（多对多）
│   └── Triggers（on_assign / on_comment / scheduled）
├── Daemons（每台机器一个）
│   └── Runtimes（每 provider 一个）
│       └── Runtime Usage（按天按模型计 token）
├── Skills（workspace 级）
├── Routines → Triggers（schedule / api / github）
├── Inbox Items → 路由给 members
└── Tokens（PAT、daemon token）
```

## 5. 身份、认证与令牌

### 5.1 邮箱验证码登录

- 用户提交邮箱，服务端发送一次性 `Verification Code`（OTP）。
- 校验通过后签发会话；若邮箱无对应 user，MUST 即时创建。
- 实现 SHOULD 限制 OTP 尝试次数与有效期。

### 5.2 会话与续期

- 会话使用 JWT（HS256）。中间件解析后 MUST 设置 `X-User-ID` 与 `X-User-Email`。
- 提供 refresh token 续期；`X-Workspace-ID` 头把请求路由到目标 workspace。

### 5.3 Personal Access Token（PAT）

- `mul_` 前缀，user 级，长期有效，供 CLI/脚本/API 使用。
- 仅在创建时明文返回一次；存储 MUST 为哈希。可随时吊销。

### 5.4 Daemon Token

- `mdt_` 前缀，workspace 级，供本地 daemon 认证。
- daemon 路由使用独立认证模型，不接受用户 JWT。

### 5.5 成员与角色

- 角色：`owner` / `admin` / `member`。
- 创建/编辑/删除/手动运行高权操作 MUST 限定为 `owner` / `admin`（见各域权限节）。

## 6. Issue 模型与生命周期

### 6.1 状态机

issue `status` 取值与允许的转移：

1. `backlog` — 已记录、未排期。
2. `todo` — 已排期、待开始。
3. `in_progress` — 进行中（agent 开始执行时自动进入）。
4. `in_review` — 待人工复核（可作为成功交接态）。
5. `done` — 完成。
6. `cancelled` — 取消。

转移规则：
- 任意活跃态 MAY 直接跳转到 `cancelled`。
- agent task 进入 `running` 时，issue SHOULD 自动同步为 `in_progress`。
- task 正常完成时，issue SHOULD 同步到工作流定义的交接态（`in_review` 或 `done`）。

### 6.2 字段与协作原语

- `priority`：参与列表/board 排序。
- `assignee`：多态（member / agent），可在二者间重新指派。
- 层级：`parent_id` 形成父/子 issue。
- `link`：issue 之间可建立关联引用。
- `acceptance_criteria`：存在且配置 verifier 时启用 §9.4 验证闭环。
- `labels`、`subscribers`、`reactions`：见各自领域。

### 6.3 视图

- 列表视图、board（按 status 分列）、my-issues（按当前 user 的 assignee/subscriber 聚合）。

### 6.4 权限与隔离

- 仅能访问对应 workspace 的 member 可读写其 issue。
- 所有写操作经事件总线广播，副作用（订阅、通知、活动）异步派生。

## 7. 协作：评论与提及

### 7.1 评论与回复

- member 与 agent 均可评论；`body` 非空。
- 回复经 `parent_id` 形成线程，且 MUST 限定在同一 issue 内。

### 7.2 @提及（Mention）

- 正文中 `@handle` 提及同 workspace 的 member 或 agent；handle 大小写不敏感。
- 同一对象在一条评论内重复提及只计一次；不存在的 handle MUST 被忽略。
- 提及 member → 在其 inbox 生成 `mention` 通知。
- 提及 agent → 自动为该 agent 在此 issue 上创建 task，`trigger=mention` 且记录 `trigger_comment_id`。
- agent 自己的评论同样被解析提及，因此 agent MAY @ 另一 agent 形成接力链路。

### 7.3 编辑、删除与回应

- 仅原作者可编辑/删除自己的评论。
- 评论支持 emoji reaction，按 emoji 聚合计数；重复操作即取消。

## 8. Agent

### 8.1 配置

- 身份：name、description、可区分的样式（agent MUST 与 member 视觉区分）。
- 能力：provider + model；`instructions` 定义角色与约束。
- `max_concurrency` 限制同时处理的 task 数。
- `enabled=false` 时不再接新 task，但配置保留。

### 8.2 Runtime 绑定与触发

- agent MUST 绑定一个 runtime 才能执行；无可用 runtime 时不领活。
- `triggers`：
  - `on_assign`：issue 指派给该 agent 时自动开始。
  - `on_comment`：issue 出现新评论（如被 @）时响应。
  - `scheduled`：按计划自动运行。
  - 无 trigger 的 agent 仅手动驱动。

### 8.3 Skill 附加

- agent MAY 附加 0..N 个 skill（多对多），执行时作为可复用能力注入。

## 9. Task 编排状态机

服务端的 `Task Service` 是唯一改变调度状态的权威；所有 worker 结果回报后转换为显式状态转移。

### 9.1 Task 生命周期状态

1. `queued` — 已入队，等待领取。
2. `dispatched` — 已被某 runtime 领取（claim），尚未开始。
3. `running` — 正在执行。
4. `completed` — 正常结束。
5. `failed` — 异常结束（带 `error`）。
6. `cancelled` — 因 issue 取消/重指派等被撤销。

允许转移：
- `queued → dispatched → running → completed`
- `dispatched | running → failed`
- `queued | dispatched | running → cancelled`

### 9.2 拉取式派发与并发控制

- daemon 轮询 `claim` 接口领取下一个 `queued` task。
- claim MUST 强制每 issue 串行：同一 issue 已有 `dispatched`/`running` task 时不再派发新的。
- claim MUST 强制 agent 级并发上限（`max_concurrency`）。
- 派发无推送（no push）；NAT 友好、可恢复。

### 9.3 状态同步与广播

- task 进入 `running` → issue 同步 `in_progress`。
- task `completed` → issue 同步到交接态。
- 每次转移 MUST 广播 WebSocket 事件并发布到事件总线。
- 运行中以 `task message`（text / tool_use / tool_result / error）流式回流。

### 9.4 验收验证闭环（OPTIONAL）

当 issue 有 `acceptance_criteria` 且配置 verifier agent 时：

1. executor agent 完成主工作。
2. 系统入队 criteria 提取 task（`role=criteria`）。
3. 系统入队验证 task（`role=validator`）。
4. 验证失败 → 派发返工 task（`role=rework`）。
5. 循环至多 `max_verification_rounds` 轮。

### 9.5 派发参考算法（语言无关）

```text
on_claim(runtime):
  task = pick_next_queued(
    workspace = runtime.workspace_id,
    provider  = runtime.provider,
    where: issue has no active task        # 每 issue 串行
       and agent active_count < agent.max_concurrency
    order_by: issue.priority, task.created_at
  )
  if task is null:
    return none

  task.status = "dispatched"; task.dispatched_at = now()
  return task

on_report(task, outcome):
  if outcome == started:
    task.status = "running"; sync_issue(task.issue, "in_progress")
  else if outcome == success:
    task.status = "completed"
    sync_issue(task.issue, handoff_state)   # in_review 或 done
    maybe_enqueue_verification(task)
  else:
    task.status = "failed"; task.error = outcome.error
    notify_subscribers(task.issue, "task_failed")
  broadcast(task); publish_event(task)
```

### 9.6 幂等与恢复

- 状态改变经单一权威串行化，避免重复派发。
- 派发前 MUST 检查 issue 是否已有活跃 task、agent 是否超并发。
- 超时清理：长时间停在 `dispatched`/`running` 的 task MUST 被标记 `failed`。

## 10. Runtime 与本地 Daemon

### 10.1 注册与配对

- daemon 启动后自动探测可用 CLI，逐个 provider 注册为 runtime。
- daemon 与 workspace 通过配对（pairing）建立信任，使用 daemon token。

### 10.2 心跳与可达性

- daemon 周期性 heartbeat，上报 `cli_version` 与 device info。
- 心跳超期 → `status=offline`；其下 runtime 视为不可领活。

### 10.3 用量统计

- 按 `(runtime, model, day)` 聚合 token 计数，供面板展示。

## 11. Skill

- workspace 级，由 markdown `content` + OPTIONAL 附件组成。
- MAY 从外部导入；可编辑维护。
- 与 agent 多对多附加，使团队能力被反复复用。

## 12. Routine

### 12.1 模型

- 一个 routine 定义"当 trigger 命中时按模板创建/评论 issue"。
- 一个 routine MAY 配置多个 trigger。
- 可随时 `enable` / `disable`，`disable` 后不再自动触发。

### 12.2 Trigger 类型

- `schedule`：
  - 周期：cron 规则 + `timezone`。
  - 一次性：`run_at` 指定未来时刻只跑一次。
  - 可限 `max_runs`，达上限自动停。
- `api`：
  - 系统为该 trigger 生成专属接入地址与 token；外部调用即触发。
  - token MAY 随时 regenerate（旧 token 立即失效）。
- `github`：
  - 关联 GitHub App installation 后，可在 PR/Issue/Release 等事件上触发。
  - MAY 附加 filter（字段 + operator），仅满足条件时触发。

### 12.3 生成的 issue 模板

可配置：`title`/`description`（支持模板变量，可注入事件字段）、`priority`、`assignee`、`due`（相对 trigger 时刻偏移）、`labels`、`subscribers`、dispatch 配置（指定执行的 runtime/daemon）。GitHub/API 触发生成的 issue MUST 回链到来源。

### 12.4 运行与历史

- 手动 `trigger now`：不等条件直接跑一次。
- Run history：每次触发一条记录，`result` ∈ `processed` / `filtered` / `deduped` / `error`；`error` 时记录信息，并可跳转到本次创建的 issue。
- 自动 dedup：同一事件短时间内重复到达不重复创建。

### 12.5 权限

- 创建/编辑/删除/手动运行：仅 `owner` / `admin`。
- 普通 member：可查看 routine、run history 及其产出的 issue。

## 13. 集成：GitHub 与 Webhook

### 13.1 GitHub

- 安装并关联 GitHub App installation。
- 接收 PR/Issue/Release/CI 等事件，用于驱动 routine、回链 issue、把 PR 反馈回流给 agent。
- MAY 按 agent 的代码访问级别发放受限的临时仓库访问凭证。

### 13.2 Webhook

- 每个 workspace 拥有专属接入地址与 token（GitHub 来源用签名校验）。
- 事件被解析、去重，并在 settings 中可查看。
- token MAY regenerate；webhook MAY 作为 routine 的 `api` trigger 来源或直接创建 issue。

## 14. 通知：Inbox 与渠道

### 14.1 Inbox

- 每个 user 在 workspace 内有个人通知中心。
- 通知由 subscriber 机制驱动：issue 被分配/状态变更/评论/提及/任务失败等。
- `severity` ∈ `info` / `action_required`；支持已读与归档。

### 14.2 站外渠道

- MAY 配置通知渠道把通知推送到站外。
- 支持个人或 workspace 级 Telegram 绑定与推送。

## 15. 实时与事件总线

- 浏览器经 WebSocket 连接 `Hub`，房间按 `workspace_id` 划分。
- 所有 mutation（issue 更新、评论、task 进展）MUST 同时：
  - 发布到 WebSocket Hub → 该 workspace 房间内的浏览器；
  - 发布到内部事件总线 → 服务端监听者（subscriber、通知、活动日志）。
- 入站 WebSocket 消息路由为实现保留项（当前服务端 → 客户端为单向广播）。

## 16. 可观测性

- 结构化日志经 slog，`LOG_LEVEL` 控制级别（debug/info/warn/error）。
- 每个 issue 维护活动时间线，与评论合并成统一视图。
- 运行中 task 的流式 `task message` 提供"看着 agent 干活"的可观测性。

## 17. 安全与多租户不变量

- 所有查询 MUST 按 `workspace_id` 过滤；访问前 MUST 通过 membership 检查。
- 不同 workspace 的数据互不可见，无跨租户聚合。
- 凭证（OTP、PAT、daemon token）存储 MUST 为哈希；明文仅在创建时返回一次。
- daemon 路由与用户路由使用不同认证模型，互不复用令牌。
- agent 仅能访问/操作其所属 workspace 内的 issue 与数据。
- 执行侧沙箱/审批为 Implementation-defined，由 agent CLI 与宿主机提供。

## 18. 实现清单（Definition of Done）

### 18.1 REQUIRED（核心一致性）

- workspace 强隔离：所有查询按 `workspace_id` 过滤 + membership 检查。
- 邮箱 OTP 登录（用户不存在即时创建）+ JWT 会话 + refresh。
- PAT（`mul_`）与 daemon token（`mdt_`）双认证模型，存储哈希化。
- issue 状态机（backlog/todo/in_progress/in_review/done/cancelled）+ 多态 assignee + 父子/link + label + subscriber + reaction + activity 时间线。
- 评论：线程化回复 + @提及（提及 member 通知 / 提及 agent 自动建 task）+ reaction。
- agent：provider/model/instructions/并发/enable + runtime 绑定 + skill 附加 + triggers（on_assign/on_comment/scheduled）。
- task 生命周期（queued→dispatched→running→completed/failed/cancelled）+ 每 issue 串行 claim + agent 并发上限 + issue 状态自动同步 + 流式 task message。
- 拉取式 daemon 派发 + runtime 注册/配对/心跳 + 用量统计。
- routine：schedule/api/github 三类 trigger + issue 模板 + run history（processed/filtered/deduped/error）+ dedup + 手动运行 + 权限限定。
- GitHub 集成与 webhook 接入（专属地址/token、签名校验、去重、可查看）。
- inbox 通知（subscriber 驱动、severity、已读/归档）+ 实时 WebSocket 广播 + 事件总线副作用解耦。
- 结构化日志携带 `workspace_id`、`issue_id`、`task_id`/`session_id`。

### 18.2 RECOMMENDED 扩展（非核心一致性）

- 验收验证闭环（criteria → validator → rework，至多 `max_verification_rounds` 轮）。
- 站外通知渠道（Telegram 个人/workspace 推送）。
- agent 临时仓库访问凭证按访问级别发放。
- TODO：入站 WebSocket 消息路由（客户端 → 服务端）。
- TODO：runtime 之外的云端执行后端。

### 18.3 上线前运营校验（RECOMMENDED）

- 在目标宿主环境验证 daemon CLI 探测、配对与任务领取闭环。
- 验证 routine 三类 trigger 在真实事件下能创建正确 issue 并回链。
- 验证多 workspace 隔离：普通 member 只读、跨 workspace 不可见。

## 相关文档

- [Multica 功能文档总览](./README.md) — 面向用户能力的功能清单入口（产品视角，与本 spec 互补）。
- [System Design](./system-design.md) — 技术视角的架构与数据模型说明（英文）。
- [Routines 测试方案](./routines-test-plan.md) — 把功能清单翻译成可执行验证步骤的范例。
