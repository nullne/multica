# Workspace 与成员 功能清单（产品视角）

> Workspace 是团队协作的容器：所有 issue、agent、comment、routine 都归属于某个 workspace。
> Workspace settings 按主题分成几个 tab——**General（基础信息）**、**Members（成员与 bot）**、
> **Providers（code agent 的 API key）**、**Repositories（代码仓库与 GitHub 连接）**，
> 此外还有 Labels、Webhooks 等。

这是一份自顶向下、面向用户能力的功能清单，用来核对产品功能是否齐全。

## 1. Workspace 是什么

- [ ] 用户可以创建一个 workspace：填写 name，并据此自动生成 slug（也可手动改，仅限小写字母、数字、连字符）。
- [ ] 创建 workspace 的人自动成为 owner，并自动切换到这个新 workspace。
- [ ] 一个用户可以同时属于多个 workspace。
- [ ] 用户可以在多个 workspace 之间切换；当前 workspace 决定看到哪些 issue、agent、成员、label 等。
- [ ] 当前 workspace 会被记住，下次进入时仍停留在上次的 workspace；未指定时默认进入第一个。
- [ ] 用户可以离开（leave）一个 workspace；离开后失去访问权，需被重新邀请。
- [ ] owner 可以删除整个 workspace（连同 issue、agent 等全部数据一并永久清除，不可恢复）。

## 2. General：基础信息

- [ ] 用户可以修改 workspace 的 name。
- [ ] 用户可以填写 description（这个 workspace 关注什么）。
- [ ] 用户可以填写 context（给在此工作的 AI agent 的背景信息）。
- [ ] slug 在此页只读展示，仅能在创建时设定。
- [ ] **Telegram 群通知**：填入群聊 chat ID 后，可把 workspace 活动（新 issue、评论、状态变化、任务结果）推送到 Telegram 群；可随时开关或移除。

## 3. Members：成员与角色

- [ ] 用户可以查看 workspace 内的全部成员，包括姓名、邮箱、头像和角色。
- [ ] 用户可以通过邮箱邀请成员加入；被邀请者即使还没注册账号也能先被加进来。
- [ ] 邀请时为新成员选择角色，默认为 member。
- [ ] 成员有三种角色：
  - [ ] **owner** —— 完全权限，管理所有设置。
  - [ ] **admin** —— 管理成员与设置。
  - [ ] **member** —— 创建并处理 issue。
- [ ] 用户可以修改某个成员的角色。
- [ ] 用户可以把某个成员移出 workspace。
- [ ] workspace 必须始终保留至少一个 owner：移除 / 降级最后一个 owner、或最后一个 owner 离开都会被阻止。

## 4. Members：Bot 用户

- [ ] 用户可以创建 bot 用户：它作为一个普通 member 加入 workspace，但不能登录。
- [ ] bot 用于代表 webhook 发表评论（例如 GitHub App webhook 的"在关联 issue 上评论"动作）。
- [ ] bot 会像普通成员一样出现在 @mention 列表里。
- [ ] 列表会显示每个 bot 被多少个 webhook 使用。
- [ ] 用户可以删除 bot（删除前会提示有多少 webhook 受影响）。

## 5. Providers：Code Agent 的 API key

- [ ] 用户可以为 workspace 配置 code agent 的 provider：Claude Code、Codex、OpenCode、Cursor。
- [ ] 每个 provider 可以单独启用 / 停用。
- [ ] 启用后可填写该 provider 的 API key。
  - [ ] API key 可切换显示 / 隐藏。
  - [ ] 留空则回退到 per-user 登录认证；填入后环境自动使用，无需每个用户单独登录。
  - [ ] 重新打开页面时 API key 以掩码形式展示，不回显明文。
- [ ] 用户可以点击 Validate 校验填入的 API key 是否有效，并看到校验结果（有效 / 被拒绝 / 暂不可用 / 不支持校验）。
- [ ] 每个 provider 展示一个只读的"已测试版本"（由仓库统一管理，用户不可改）。

## 6. Repositories：代码仓库与 GitHub 连接

- [ ] 用户可以为 workspace 配置一个或多个代码仓库（仓库 URL + 描述），供 agent 克隆并在其中工作。
- [ ] 用户可以新增 / 编辑 / 删除仓库条目并保存。
- [ ] **GitHub App 连接**：用户可以连接 / 断开 workspace 的 GitHub App。
  - [ ] 页面显示当前是否已连接。
  - [ ] 连接后 agent 会自动拿到受限范围的 GitHub token；未连接时 agent 依赖宿主机的 git 凭据。

## 7. 权限与 workspace 隔离

- [ ] 所有数据（issue、agent、comment、成员、label、provider 配置、repos 等）都归属于某个 workspace，并按 workspace 严格隔离，互不可见。
- [ ] 用户只能访问自己所属 workspace 的内容；请求会根据当前 workspace 路由。
- [ ] 修改 General 设置、管理成员 / bot、配置 provider 与 repositories、连接 / 断开 GitHub：仅 owner / admin。
- [ ] 邀请或设置 owner 角色、删除 workspace：仅 owner。
- [ ] 普通 member 只能查看上述设置，不能修改。
- [ ] provider 的 API key 加密存储，界面只以掩码展示，不回显明文。

## 相关文档

- [Agents](./agents.md) — Agents（AI 队友）与触发方式
- [GitHub 集成](./github-integration.md) — GitHub 集成
- [Inbox 与通知](./inbox-and-notifications.md) — Inbox、通知渠道与 Telegram
- [登录与 Token](./auth-and-tokens.md) — 登录、PAT 与 daemon token
