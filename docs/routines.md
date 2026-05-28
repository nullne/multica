# Routines 功能清单（产品视角）

> Routine 让团队把重复性的工作自动化：**当某件事发生时，自动创建一个 issue**。
> 一个 routine 由三部分组成——**什么时候 trigger**、**生成的 issue 长什么样**、
> **运行后由谁来跟进**。

这是一份自顶向下、面向用户能力的功能清单，用来核对产品功能是否齐全。

## 1. Routine 是什么

- [ ] 用户可以创建一个 routine：定义一个 issue 模板，并指定它在什么条件下自动执行。
- [ ] 一个 routine 可以配置多个 trigger。
- [ ] 每次 trigger 命中后，routine 会按模板**创建一个 issue**。
- [ ] Routine 可以随时 enable / disable，disable 后不再自动 trigger。

## 2. Trigger：什么时候自动运行

- [ ] **Schedule（按时间）**
  - [ ] 周期性运行：用 cron 规则设定，例如"每周一早上 9 点"。
  - [ ] 一次性运行：指定未来某个时间点只跑一次（run at）。
  - [ ] 可指定 timezone。
  - [ ] 可限制最大运行次数（max runs），达到上限后自动停止。
- [ ] **API（收到外部调用时）**
  - [ ] 系统为该 trigger 生成专属的接入地址和 token，外部系统调用即可触发。
  - [ ] token 可随时 regenerate（旧 token 立即失效）。
- [ ] **GitHub（GitHub 事件发生时）**
  - [ ] 关联 GitHub App installation 后，可在 PR、Issue、Release 的各类事件上触发
        （如 `pull_request.opened`、`pull_request.merged`、`issues.opened`、`release.published` 等）。
  - [ ] 可对事件附加 filter（按字段 + operator），只在满足条件时才触发。

## 3. 生成的 issue 长什么样：模板内容

- [ ] Title 与 description（支持模板变量，可把 trigger 事件里的信息如 PR title、author 填进去）。
- [ ] Priority。
- [ ] Assignee（可指定 member 或 agent）。
- [ ] Due date（相对 trigger 时刻的偏移，如"trigger 后 24 小时"）。
- [ ] Labels。
- [ ] Subscribers（issue 创建后会通知这些人）。
- [ ] Dispatch 配置：指定由哪个 agent runtime / daemon 来执行。
- [ ] 由 GitHub / API 事件触发时，生成的 issue 会回链到来源（如对应的 PR）。

## 4. 运行与历史

- [ ] **手动立即运行（trigger now）**：不等条件满足，直接点一下就执行一次。
- [ ] **Run history**：每次 trigger 都有记录，可查看：
  - [ ] 运行结果状态：`processed` / `filtered` / `deduped` / `error`。
  - [ ] `error` 时显示错误信息。
  - [ ] 本次运行创建的 issue，可直接点进去。
- [ ] **自动 dedup**：同一事件在短时间内重复到达时不会重复创建，避免刷屏。

## 5. 界面与导航

- [ ] Sidebar 有顶层 "Routines" 入口。
- [ ] Routine 列表页：展示所有 routine；没有时显示引导用户创建的 empty state。
- [ ] 新建 / 编辑表单：在一个页面里配置 trigger 和 issue 模板。
- [ ] Routine 详情页：查看概要、手动运行、enable/disable、编辑、删除、查看 run history。

## 6. 权限

- [ ] 创建、编辑、删除、手动运行 routine：仅 workspace 的 owner / admin。
- [ ] 普通 member：可查看 routine、run history，以及 routine 创建出来的 issue。
- [ ] 所有数据按 workspace 隔离，互不可见。
