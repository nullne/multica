# Routines 测试方案（产品视角）

> 这份测试方案用来确认 routines 功能是否按产品清单成立：用户能配置 routine，
> trigger 命中后能按模板自动创建 issue，并且权限、历史、隔离和异常状态都可被人核对。

测试结果要让人快速判断“能不能用”：关键 UI 流程用录屏或截图；自动化测试输出用 checklist
汇总到本文件对应条目，不要求人阅读测试代码或原始日志。

## 1. 最终验收结论

- [ ] Owner / admin 可以完成 routine 的创建、编辑、enable / disable、删除和手动运行。
- [ ] Schedule、API、GitHub 三类 trigger 都能在命中条件时创建正确的 issue。
- [ ] 生成的 issue 正确带上 title、description、priority、assignee、due date、labels、subscribers、
      dispatch 配置和来源回链。
- [ ] Run history 能解释每次运行结果：`processed` / `filtered` / `deduped` / `error`。
- [ ] 权限与 workspace 隔离正确：普通 member 只能查看，不能修改或运行；不同 workspace 互不可见。

## 2. 给人看的测试产出

- [ ] 一段完整录屏：从 sidebar 进入 Routines，新建 routine，配置 trigger 与 issue 模板，手动运行，
      查看创建出的 issue 和 run history。
- [ ] 一组关键截图：empty state、routine 列表、编辑表单、详情页、run history、权限拦截状态。
- [ ] 一份自动化结果摘要：按本方案的一级分组列出通过 / 失败 / 未覆盖，不展示底层日志。
- [ ] 失败项必须说明用户可见影响，例如“API trigger token regenerate 后旧 token 仍能触发 issue”。

## 3. 端到端主流程

- [ ] **创建 routine**
  - [ ] Owner / admin 从 Routines 入口进入列表页。
  - [ ] Empty state 能引导创建第一个 routine。
  - [ ] 新建表单能同时配置 trigger 和 issue 模板。
  - [ ] 创建成功后进入详情页，展示 enable 状态、trigger 概要、issue 模板概要和 run history。
- [ ] **手动运行 routine**
  - [ ] 在详情页点击 trigger now。
  - [ ] 系统立即创建一个 issue。
  - [ ] 新 issue 的字段与模板一致。
  - [ ] Run history 新增一条 `processed` 记录，并能点进创建出的 issue。
- [ ] **编辑与停用**
  - [ ] 编辑 routine 后，下一次运行使用新模板。
  - [ ] Disable 后，自动 trigger 不再创建 issue。
  - [ ] Enable 后，自动 trigger 恢复工作。
  - [ ] 删除 routine 后，列表与详情入口不再可见。

## 4. Trigger 覆盖

- [ ] **Schedule trigger**
  - [ ] Cron 周期运行能按指定时间创建 issue。
  - [ ] Run at 一次性运行只执行一次。
  - [ ] Timezone 会影响实际触发时间。
  - [ ] Max runs 达到上限后自动停止。
- [ ] **API trigger**
  - [ ] 每个 API trigger 有专属接入地址和 token。
  - [ ] 使用正确 token 调用时创建 issue。
  - [ ] 缺少 token 或 token 错误时不创建 issue，并返回可理解错误。
  - [ ] Regenerate token 后，新 token 生效，旧 token 立即失效。
- [ ] **GitHub trigger**
  - [ ] 关联 GitHub App installation 后，可以选择 PR、Issue、Release 事件。
  - [ ] `pull_request.opened`、`pull_request.merged`、`issues.opened`、`release.published`
        等事件能按配置触发。
  - [ ] Filter 满足时创建 issue。
  - [ ] Filter 不满足时不创建 issue，并在 run history 中记录为 `filtered`。

## 5. Issue 模板覆盖

- [ ] Title 与 description 支持模板变量，并能填入 trigger 事件信息。
- [ ] Priority 与模板一致。
- [ ] Assignee 可以是 member。
- [ ] Assignee 可以是 agent。
- [ ] Due date 能按 trigger 时刻的相对偏移计算。
- [ ] Labels 正确写入 issue。
- [ ] Subscribers 会在 issue 创建后收到通知。
- [ ] Dispatch 配置会指定正确的 agent runtime / daemon。
- [ ] GitHub / API 事件触发的 issue 会回链到来源。

## 6. 运行历史与异常

- [ ] 每次 trigger 尝试都会产生 run history。
- [ ] 成功创建 issue 时，状态为 `processed`。
- [ ] Filter 未命中时，状态为 `filtered`，且不创建 issue。
- [ ] 同一事件短时间重复到达时，状态为 `deduped`，且只创建一个 issue。
- [ ] 创建 issue 失败时，状态为 `error`，并显示人能理解的错误信息。
- [ ] Run history 中的 issue 链接能跳转到对应 issue。

## 7. 权限与隔离

- [ ] Owner / admin 可以创建、编辑、删除、手动运行 routine。
- [ ] 普通 member 可以查看 routine 列表、详情和 run history。
- [ ] 普通 member 看不到创建、编辑、删除、手动运行入口。
- [ ] 普通 member 直接访问受限接口时会被拒绝。
- [ ] Workspace A 的 routine、trigger、run history 和创建出的 issue 不会出现在 Workspace B。

## 8. 最小自动化分层

- [ ] **E2E** 覆盖用户主流程：创建 routine、手动运行、查看 issue、查看 run history、权限只读。
- [ ] **Backend integration** 覆盖 trigger 执行：schedule、API、GitHub、filter、dedup、error。
- [ ] **API contract** 覆盖 routine CRUD、token regenerate、manual run、run history 查询和权限拒绝。
- [ ] **Unit** 覆盖模板变量渲染、due date 计算、filter 判断、dedup key 生成。

## 9. 发布前检查顺序

- [ ] 先跑自动化测试，得到按本方案分组的结果摘要。
- [ ] 再做一次人工 UI 录屏，确认真实入口、文案、跳转、空状态、权限状态都可理解。
- [ ] 最后按“最终验收结论”逐项勾选；任何未勾选项都要说明是未实现、未测试，还是产品范围变化。
