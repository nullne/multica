# Runtimes 与本地 Daemon 功能清单（产品视角）

> 本地 daemon 把你自己的机器变成 agent 的执行环境：它检测机器上装好的 AI CLI，
> 把它们注册成 workspace 里可用的 **runtime**，并主动领取被 assign 给 agent 的任务在本地跑起来。
> 这个领域由三部分组成——**机器上的 daemon（一台设备）**、**daemon 注册出来的 runtime（每个 AI CLI 一个）**、
> 以及 **workspace 里看到的运行状态、用量与管理操作**。

这是一份自顶向下、面向用户能力的功能清单，用来核对产品功能是否齐全。

## 1. 在本机运行 daemon

- [ ] 用户可以在自己的机器上启动一个 daemon，把这台设备接入 Multica。
- [ ] daemon 可以在后台常驻运行，也可以前台运行以便排查问题。
- [ ] 用户可以随时停止 daemon；停止后它在各 workspace 里的 runtime 会被标记为离线。
- [ ] 用户可以查看 daemon 状态：是否在线、运行时长、检测到的 agent、正在跟进的 workspace。
- [ ] 用户可以查看 daemon 日志，支持只看最近若干行或实时跟随输出。
- [ ] 同一台机器上可以用 profile 同时跑多个互不干扰的 daemon（例如生产与测试各一个），各自独立的配置、状态与工作目录。
- [ ] daemon 可连接官方云端，也可连接用户自建的 Multica 服务。

## 2. 自动检测可用的 AI CLI

- [ ] daemon 启动时自动检测机器上已安装的 AI 编码 CLI，无需用户手动声明。
- [ ] 支持检测 Claude、Codex、opencode、Cursor 等多种 agent CLI。
- [ ] 检测时会读取每个 CLI 的版本号。
- [ ] 检测时会判断每个 CLI 是否已完成登录授权（已就绪 / 未授权 / 未安装）。
- [ ] 用户可以通过设置覆盖某个 CLI 的可执行文件路径与使用的模型。
- [ ] 至少需要装好一个受支持的 CLI，daemon 才有可用的 runtime。

## 3. Runtime（每个 CLI 一个执行单元）

- [ ] daemon 把检测到的每个 CLI 注册为对应 workspace 里的一个 runtime。
- [ ] 每个 runtime 标明它属于哪个 provider（claude / codex / opencode / cursor）。
- [ ] runtime 会带上所在设备信息和 CLI 版本，便于区分来自哪台机器。
- [ ] runtime 有清晰的运行状态：在线 / 离线。
- [ ] runtime 有授权状态：已就绪 / 未授权 / 未安装。
  - [ ] 即使本机 CLI 未登录，只要所在 workspace 配置了该 provider 的 API key，runtime 也会显示为就绪（key 在任务下发时注入）。
- [ ] workspace 里可以查看本 workspace 全部可用的 runtime 列表。
- [ ] 除本地 runtime 外，列表里也能展示独立的云端 runtime。

## 4. 接入 workspace 与配对

- [ ] 用户首次启动 daemon 时，系统自动把它接入用户所属的全部 workspace，开箱即用。
- [ ] 之后用户可以按 workspace 单独开启或关闭这台 daemon——只在指定 workspace 里跟进任务。
- [ ] daemon 只处理已开启的 workspace 的任务。
- [ ] 在某个 workspace 关闭后，该 daemon 及其 runtime 对该 workspace 的其他成员不再可见。
- [ ] 用户可以查看一台 daemon 当前在哪些 workspace 开启或关闭。

## 5. 心跳、设备信息与可达性

- [ ] daemon 周期性发送心跳，让服务端知道它仍然存活。
- [ ] 每次心跳会刷新 daemon 及其所有 runtime 的最近在线时间。
- [ ] daemon 会定期重新检查各 CLI 的授权状态并上报，授权变化能及时反映到 runtime 上。
- [ ] daemon 记录设备名称与设备信息；用户可以重命名设备，方便在多台机器间辨认。
- [ ] daemon 记录所用 CLI 的版本。
- [ ] 用户可以给 daemon 发一次诊断 ping，确认它当前可达并看到往返延迟。
- [ ] 用户可以查看 daemon 注册时上报的环境变量信息。

## 6. 拉取式任务领取与执行

- [ ] daemon 主动按固定间隔轮询、领取被 assign 给 agent 的待办任务（拉取式，无需对外开放端口）。
- [ ] 任务被原子领取，避免多个 runtime 抢到同一个任务。
- [ ] 领取任务时会带上 agent 的名称、说明与 skills，让本地执行使用正确的人设与能力。
- [ ] 任务携带所在 workspace 的代码仓库信息，daemon 在本地为其准备独立的工作目录 / worktree。
- [ ] 对配置了代码访问的 agent，任务会附带限定范围的 GitHub 访问令牌。
- [ ] 同一个 agent 在同一个 issue 上的后续任务能够续接上一次的会话上下文与工作目录。
- [ ] 执行过程中的消息（工具调用、思考、输出、错误）会实时回传，并在 issue 上可见。
- [ ] daemon 上报任务进度，以及任务的开始、完成、失败等状态变化。
- [ ] daemon 执行中会检查任务是否已被取消，以便及时停止。
- [ ] 可配置任务执行的轮询间隔、心跳间隔、单任务超时与最大并发任务数。

## 7. 用量与活跃度统计

- [ ] runtime 上报按天、按 provider、按模型的 token 用量（输入、输出、缓存读取、缓存写入）。
- [ ] 用户可以查看某个 runtime 的用量趋势图，并选择回看的天数范围。
- [ ] 用户可以查看某个 runtime 按小时分布的任务活跃度。

## 8. Daemon 与 Runtime 管理面板

- [ ] workspace 里有一个面板，按设备 / daemon 分组展示 runtime，并列出独立的云端 runtime。
- [ ] 打开某台 daemon 可以查看设备信息、CLI 版本、最近在线时间，以及它检测到的 runtime。
- [ ] 在 daemon 详情里可以逐个 workspace 开关、发起 ping 看延迟、查看环境变量。
- [ ] 打开某个 runtime 可以查看 provider、状态、授权状态、设备信息、用量图与任务活跃度。
- [ ] 用户可以请求 daemon 自更新 CLI，或把某个 agent runtime 更新到目标版本；daemon 在下次心跳时执行。
- [ ] 用户可以归档不再使用的 daemon，并在需要时恢复。

## 9. 权限与 workspace 隔离

- [ ] daemon 归属于把它启动起来的用户（机器所有者）。
- [ ] 重命名设备、增减 workspace 接入、触发更新、归档 / 恢复等管理操作，只有机器所有者本人能做——别人不能管理你的机器。
- [ ] runtime 与 daemon 状态按 workspace 隔离：成员只能看到在本 workspace 已开启、且其所有者仍是本 workspace 成员的 daemon 与 runtime。
- [ ] 在某 workspace 被关闭或所有者已退出该 workspace 的 daemon / runtime，对该 workspace 的其他成员一律不可见。
- [ ] 用量与任务活跃度数据只在有权访问对应 runtime 的前提下可见。
- [ ] 任务只会下发给所在 workspace 已开启的 runtime。

## 相关文档

- [Agents（AI 队友）](./agents.md)
- [任务执行生命周期与验证循环](./tasks-and-execution.md)
- [登录、PAT 与 daemon token](./auth-and-tokens.md)
- [Workspace、成员与角色](./workspace-and-members.md)
