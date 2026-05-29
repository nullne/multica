# Skills 功能清单（产品视角）

> Skill 是团队可复用的"技能包"：一段 markdown 写的能力说明，外加任意数量的附带文件。
> 把 skill 附加到 agent 上，agent 在工作时就能用上这份沉淀下来的知识与工具。
> 一个 skill 由三部分组成——**它是什么（说明内容）**、**它带了哪些文件**、**它被哪些 agent 使用**。

这是一份自顶向下、面向用户能力的功能清单，用来核对产品功能是否齐全。

## 1. Skill 是什么

- [ ] 用户可以创建一个 skill：给它一个名字、一段简短描述，以及一份 markdown 正文来说明这项技能。
- [ ] Skill 的核心内容是 markdown 文本，用来描述"这项技能能做什么、该怎么用"。
- [ ] Skill 可以附带额外配置（config），用于记录该技能的结构化设置。
- [ ] 同一个 workspace 内 skill 的名字唯一，重名会被拒绝。
- [ ] 系统会记录每个 skill 由谁创建。

## 2. 附带文件

- [ ] 一个 skill 除了 markdown 正文，还可以附带任意多个文件（例如脚本、模板、参考资料）。
- [ ] 每个文件有自己的相对路径和文本内容，路径会被校验以保证安全（不允许绝对路径或向上跳目录）。
- [ ] 文件可以单独新增 / 覆盖 / 删除。
- [ ] 编辑 skill 时可以整体替换它的文件集合。
- [ ] 查看 skill 时能看到它包含的所有文件，并以文件树的形式浏览、逐个查看内容。

## 3. 从外部导入

- [ ] 用户可以粘贴一个链接，把现成的 skill 一键导入到自己的 workspace。
- [ ] 支持从 ClawHub 导入：自动抓取技能信息、最新版本以及随附文件。
- [ ] 支持从 skills.sh（基于 GitHub 仓库）导入：自动找到 `SKILL.md`、读取其名称与描述，并递归收集随附文件。
- [ ] 导入时会自动跳过无关文件（如 LICENSE），并对文件路径做安全校验。

## 4. 编辑与维护

- [ ] 用户可以修改 skill 的名字、描述、正文内容和配置。
- [ ] 用户可以删除 skill。
- [ ] Skill 的任何创建 / 修改 / 删除都会实时同步给 workspace 内的其他人，无需刷新。

## 5. 附加到 agent（复用的关键）

- [ ] Skill 与 agent 是多对多关系：一个 skill 可被多个 agent 使用，一个 agent 也可以挂多个 skill。
- [ ] 用户可以为某个 agent 设置它启用的 skill 集合（一次性指定整组）。
- [ ] 用户可以查看某个 agent 当前挂载了哪些 skill。
- [ ] 调整 agent 的 skill 集合后，变更会实时同步出去。
- [ ] Agent 干活时即可使用这些 skill，让团队沉淀的能力被反复复用。

## 6. 团队沉淀与共享

- [ ] Skill 属于整个 workspace，而不是某个人，团队成员共享同一份技能库。
- [ ] 一处创建、处处可用：任意 agent 都能从 workspace 的技能库中挑选并启用。
- [ ] 通过"导入 + 附加到 agent"，团队可以把外部最佳实践沉淀进自己的技能库并长期复用。

## 7. 界面与导航

- [ ] Sidebar 有顶层 "Skills" 入口。
- [ ] Skills 列表 / 详情页：浏览 workspace 内的所有 skill，查看其正文与附带文件。
- [ ] 文件树 + 文件查看：以树状结构浏览 skill 的附带文件并查看单个文件内容。

## 8. 权限与 workspace 隔离

- [ ] 创建、编辑、删除 skill，以及增删其附带文件：仅限 workspace 的 owner / admin。
- [ ] 修改某个 agent 的 skill 集合：仅限有权管理该 agent 的人。
- [ ] 普通 member：可浏览 workspace 内的 skill 及其内容。
- [ ] 所有 skill 与附带文件都按 workspace 隔离，跨 workspace 互不可见、互不可访问。

## 相关文档

- [agents.md](./agents.md) — Agents（AI 队友）与触发方式
- [workspace-and-members.md](./workspace-and-members.md) — Workspace、成员与角色
- [runtimes-and-daemons.md](./runtimes-and-daemons.md) — Runtimes 与本地 Daemon
- [tasks-and-execution.md](./tasks-and-execution.md) — 任务执行生命周期与验证循环
