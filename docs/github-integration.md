# GitHub 集成 功能清单（产品视角）

> GitHub 集成让 workspace 与 GitHub 仓库打通：**把 GitHub 上发生的事接进来**（PR、Issue、Release、CI 等事件），
> 把这些事件**用来驱动自动化、回链 issue、甚至把 PR 上的反馈自动喂回给 agent**，
> 同时**让 agent 能带着合适的权限去操作仓库**（读代码、开 PR）。
> 它由四部分组成——**把 GitHub App 装到组织/账号并关联到 workspace**、**接收并消费 GitHub 事件**、
> **用事件驱动 routine 与 PR 反馈自动回流**、**按 agent 的代码访问级别发放受限的仓库访问权限**。

这是一份自顶向下、面向用户能力的功能清单，用来核对产品功能是否齐全。

## 1. 连接 GitHub App

- [ ] 用户可以从 workspace 设置里发起连接，跳转到 GitHub 去安装 / 配置 GitHub App。
- [ ] 安装界面可以选择把 App 装到哪个组织或账号、授权哪些仓库。
- [ ] 安装完成后回到 Multica，系统自动把这次 installation 关联到当前 workspace，连接即生效。
- [ ] 重复跑一遍安装流程不会产生重复连接，结果保持一致（幂等）。
- [ ] 连接成功后，系统会自动为这个 installation 准备好接收事件所需的通道，用户无需手动配置。

## 2. 查看与断开连接

- [ ] 用户可以查看当前 workspace 的 GitHub 连接状态：
  - [ ] 是否已连接。
  - [ ] 当前连接的是哪一个 installation。
  - [ ] 平台是否已经具备 GitHub App 能力（未配置时连接入口不可用，并给出明确提示）。
- [ ] 用户可以随时断开连接：断开后该 workspace 不再接收任何 GitHub 事件，相关接入通道一并清除。

## 3. 接收 GitHub 事件

- [ ] 连接后，GitHub 上的事件会被自动接收进来，覆盖常见的工作流事件：
  - [ ] **Pull Request**：开启、关闭、合并、重开、改为草稿 / 可评审、打标签等（合并会被识别为单独的 "merged" 动作）。
  - [ ] **Issue**：开启、关闭、重开、编辑、打标签等。
  - [ ] **评论与评审**：PR / Issue 评论、PR 行内评审评论、PR 评审提交。
  - [ ] **Release**：发布、预发布、编辑、删除等。
  - [ ] **CI 状态**：check run / check suite / commit status / workflow run 完成（含成功 / 失败 / 取消等结论）。
  - [ ] **Push**：推送到分支（含提交列表摘要）。
  - [ ] 对噪声较大的动作（如指派变动）做了过滤，不会让无关事件打扰用户。
- [ ] 每一条进来的事件都会先做来源校验（验证确实来自 GitHub），校验不通过的请求被拒绝。
- [ ] GitHub 的连通性检查（ping）会被正确响应，方便用户确认通道是否打通。
- [ ] 进来的事件按 installation 自动归属到对应的 workspace，不会串到别的 workspace。
- [ ] 已接收的 GitHub 事件可以在设置里查看，便于用户确认事件是否真的送达。
- [ ] 同一条事件在短时间窗口内重复到达时会被去重；评论 / 评审 / 打标签等按其各自的唯一来源去重，避免重复处理。
- [ ] 如果某个 installation 在本侧已经没有任何用途（未完成连接、或已断开），事件会被安静地忽略，且不会让 GitHub 反复重试。

## 4. 用 GitHub 事件驱动自动化

- [ ] **驱动 routine**：GitHub 事件可作为 routine 的 trigger，当指定的 PR / Issue / Release / CI 等事件发生时，自动按模板创建 issue。
- [ ] **回链来源**：由 GitHub 事件创建出来的 issue 会回链到来源（对应的 PR / Issue / Release），方便从 issue 一键跳回 GitHub。
  - [ ] PR 正文里用关闭关键字（如 `fixes #123`）或直接贴出的 issue 链接，都会被识别为来源候选，用于反向匹配。
- [ ] **PR 反馈自动回流（auto-fix）**：当一个 issue 开启了 auto-fix，且它关联的 PR 收到以下情况时，系统会自动在该 issue 上以机器人身份留下评论，把反馈带回来让负责的 agent 继续跟进：
  - [ ] PR 收到新评论或评审评论。
  - [ ] PR 评审被提交。
  - [ ] PR 相关的 CI 检查 / workflow 失败（成功的检查不打扰）。
  - [ ] 回流的机器人评论同样会去重，同一条 PR 反馈不会被重复带回。
- [ ] 同一条 GitHub 事件可以同时服务多个用途（既驱动 routine，又触发 auto-fix 回流，并走统一的事件处理）。

## 5. 给 agent 发放仓库访问权限

- [ ] 当 agent 需要访问代码时，系统会基于 workspace 的 GitHub 连接，为它临时发放一份受限的仓库访问凭证。
- [ ] 凭证按 agent 配置的**代码访问级别**授予不同权限：
  - [ ] **read**：可读取代码、读取 PR；可在 issue 上写。
  - [ ] **write**：在 read 之上，可写代码、可创建 / 更新 PR。
  - [ ] **admin**：在 write 之上，额外允许合并 PR（合并仅限 admin 级别）。
- [ ] 凭证只覆盖该 agent 实际需要的仓库范围，而不是整个账号的全部仓库。
- [ ] 凭证是短期有效的临时令牌，不会长期暴露，也不会回传给前端。
- [ ] 如果 workspace 没有连接 GitHub，agent 不会拿到任何仓库访问权限（能力优雅缺失，不报错阻断）。

## 6. 权限与 workspace 隔离

- [ ] 发起连接、断开连接、管理 GitHub 设置：由 workspace 内有相应权限的成员操作。
- [ ] 一个 GitHub installation 只关联到发起连接的那个 workspace；事件与凭证都严格按 installation → workspace 归属。
- [ ] 不同 workspace 的 GitHub 连接、收到的事件、发放的仓库权限彼此隔离，互不可见。
- [ ] 平台是否启用 GitHub App 能力由部署方统一配置；未启用时，所有 GitHub 相关入口都明确不可用。

## 相关文档

- [routines.md](./routines.md) — Routines（自动创建 issue），可用 GitHub 事件作为 trigger
- [webhooks.md](./webhooks.md) — Webhook 事件接收与统一处理管线
- [agents.md](./agents.md) — Agents（AI 队友）与其代码访问级别
- [comments.md](./comments.md) — 评论与 @提及（auto-fix 通过机器人评论回流反馈）
