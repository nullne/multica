# Webhook 事件功能清单（产品视角）

> Webhook 让外部系统把"发生了什么"推送进 Multica：每个 workspace 拥有**专属接入地址和一把 token**，
> 外部服务带着 token 把事件 POST 进来，Multica 接收、解析、去重并记录这些事件，
> 再把它们转化为 workspace 内的工作（如自动创建 issue），也可作为 routine 的触发来源。
> 这个领域由三部分组成——**怎么把事件送进来**、**收到的事件如何查看与去重**、**事件如何驱动自动化**。

这是一份自顶向下、面向用户能力的功能清单，用来核对产品功能是否齐全。

## 1. 接收外部 Webhook

- [ ] 每个 workspace 都能接收来自外部系统的 webhook 请求，无需用户登录界面即可投递。
- [ ] 支持多种来源类型，用户接入时按来源选择对应格式：
  - [ ] **Standard**：直接用 Multica 的标准 JSON 结构投递（title、body、priority、自定义 fields 等），无需额外适配。
  - [ ] **OSS Alert**：兼容 Prometheus / Alertmanager 风格的告警（labels、annotations、起止时间、来源链接），单条或批量告警都能解析。
  - [ ] **GitHub**：接收 GitHub App 的事件（push、pull request、issue、评论等），通过 GitHub 连接流程自动接入，无需手动建 token。
- [ ] 外部投递的事件被解析为标准化事件后，可携带标题、正文、优先级以及来源链接等信息。

## 2. 专属接入地址与 Token

- [ ] 每个接入点拥有自己专属的 webhook 地址，外部系统向这个地址投递事件。
- [ ] 地址配有一把 token 作为身份凭证：只有带着正确 token 的请求才会被接收，缺失或错误的 token 会被拒绝。
- [ ] 用户可以查看接入点的来源类型、token 前缀等信息，方便在外部服务里配置和辨认。
- [ ] token 与 workspace 绑定，不同 workspace 的地址和凭证互不通用。
- [ ] GitHub 来源由签名校验保护，不使用手动 token。

## 3. 重新生成 Token

- [ ] 用户可以随时重新生成（regenerate）接入点的 webhook token。
- [ ] 重新生成后，旧 token 立即失效，只有新 token 能继续投递事件。
- [ ] 当 token 可能已经泄露，或需要轮换凭证时，用户可以用这个能力收回外部访问权限。

## 4. 查看收到的 Webhook 事件

- [ ] 用户可以在 settings 的 Webhook 事件页查看本 workspace 最近收到的事件列表。
- [ ] 每条事件展示：来源接入点的名称与来源类型、收到时间、处理结果状态。
- [ ] 处理结果状态一目了然：
  - [ ] **processed**：已成功处理（如成功创建了 issue）。
  - [ ] **filtered**：收到但未触发任何动作（如接入点被暂停、或没有匹配的处理规则）。
  - [ ] **deduped**：被识别为重复事件而跳过。
  - [ ] **error**：处理出错，并展示对应的错误信息。
- [ ] 事件可展开查看原始 payload 内容，便于排查外部对接是否正常。
- [ ] 如果事件创建了 issue，可直接从事件记录跳转到该 issue。
- [ ] 还没有任何事件时，展示引导用户的 empty state。

## 5. 事件去重

- [ ] 同一个事件在短时间窗口内被重复投递时，Multica 会识别并去重，不会重复处理或重复触发后续动作。
- [ ] 这让外部系统可以安全地重试投递，而不必担心造成重复的 issue 或评论。

## 6. 事件如何驱动 workspace 内的工作

- [ ] 收到的事件可以**自动创建 issue**：按模板生成标题、正文、优先级，并指派给指定的 agent。
  - [ ] issue 可附带 labels、subscribers，并指定由哪个 runtime / daemon 来执行。
  - [ ] 模板支持把事件里的字段（如标题、正文）填入生成的 issue。
- [ ] 当事件关联到已有的来源（如某个 PR）时，会回链到对应的 issue，而不是重复新建。
- [ ] 收到的事件也可作为 **routine 的触发来源**：让某类外部事件到达时，按 routine 的配置自动开出工作。
- [ ] 这把"外部世界发生了某件事"和"在 workspace 内自动开一条工作"连接起来，无需人工搬运。

## 7. 权限与 Workspace 隔离

- [ ] webhook 的接入地址、token 和收到的事件都严格按 workspace 隔离，互不可见、互不干扰。
- [ ] 只有 workspace 的成员才能查看接入信息、查看收到的事件，以及重新生成 token。
- [ ] 外部投递进来的事件只会落在持有对应 token（或对应 GitHub 连接）的那个 workspace 内，不会跨 workspace 泄露。

## 相关文档

- [Routines（自动创建 issue）](./routines.md)
- [Issue 管理（核心工作单元）](./issues.md)
- [GitHub 集成](./github-integration.md)
- [登录、PAT 与 daemon token](./auth-and-tokens.md)
