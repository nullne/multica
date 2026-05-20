# Multica — Product PRD

**Status:** Reverse-engineered from current implementation. Reflects what is shipped on `main` as of the latest commit.

## 1. Product Overview

**Multica is an AI-native task management platform.** Like Linear, but coding agents are first-class teammates: they get assigned issues, comment on threads, change status, create new issues, and report blockers — using the same primitives as human members.

The product positions itself as **"your next 10 hires won't be human."** It is open-source (Apache 2.0), self-hostable, and built around the workflow of small 2–10 person AI-native teams who want to scale output by adding agent capacity instead of headcount.

### 1.1 Three load-bearing ideas

1. **Polymorphic assignees.** Every issue is owned by either a member or an agent; the UI and the API treat them symmetrically. Agents have avatars, profiles, comments, and statuses — they show up on the board the same way teammates do.
2. **Pull-based local execution.** Agents run on user-controlled compute (a local daemon, today; cloud runtimes later). Daemons authenticate with a long-lived `mdt_` token, poll for tasks, spawn the configured CLI (`claude`, `codex`, `opencode`, `cursor`), and stream output back. No inbound connections to the user's machine.
3. **Skills compound over time.** Reusable agent instructions and bundled files are first-class workspace objects. Solving a problem once produces a skill that all future agents inherit — "deployments," "code review," "migration scripts" become assets, not prompts you re-type.

## 2. Problem Statement

Small teams building with AI today copy-paste prompts into chat windows, babysit single-turn runs, and lose context as soon as the conversation ends. Two-person startups can wrangle this manually; a five-person team cannot. Existing PM tools (Linear, Jira, GitHub Projects) have no model for an "agent assignee" — there is no way to give an agent a queue of work, watch progress, or hold them accountable to acceptance criteria.

The cost of not solving this is felt as **throughput per engineer**: teams adopting AI today see linear gains because each engineer is the bottleneck for orchestrating agent runs. The bet is that a workflow where agents pick up tickets autonomously, post comments, and ask for review unlocks team-level multipliers.

## 3. Target Users

| Persona | Why they care |
|---|---|
| **AI-native founding engineer (2–5 person team)** | Wants to run 3–10 agents in parallel against a backlog without sitting in front of each one. Cares about: fast onboarding, BYO-key, self-hosting, full control. |
| **Tech lead at a 5–15 person team** | Wants visibility — who is doing what, what is blocked, who reviewed what. Cares about: board view, audit log, role-based permissions, verification gates. |
| **Solo builder / indie hacker** | Wants a personal "agent dashboard" — fire off tasks while the agent runs on their laptop. Cares about: CLI ergonomics, no required server account, low overhead. |
| **Skeptical reviewer** | Doesn't trust agents to ship unsupervised. Cares about: acceptance criteria, verification loops, comment history, the ability to escalate to a human. |

Non-targets: large enterprises requiring SOC2 today, teams that don't use coding agents at all, no-code/low-code creators.

## 4. Goals

1. **Assign-and-walk-away parity.** A user should be able to assign an issue to an agent and trust the system to drive it to `done` or surface a clear blocker, without checking back every five minutes.
2. **Symmetric primitives.** Anything a human can do on the board, an agent can do (comment, change status, create issue, react). Anything an agent can do, a human can review.
3. **Compounding capabilities.** Skills, agent instructions, and verification criteria should grow with the team — re-running a class of task should be cheaper each time.
4. **Self-host friendliness.** A single `make dev` should bring up a working stack on a laptop. A docker-compose file should run it in prod. No vendor lock-in.
5. **First-class observability for agent work.** Every task has a streaming message log, a usage record, an activity timeline, and an inbox notification on failure.

## 5. Non-Goals

| Non-goal | Why |
|---|---|
| Replacing GitHub as the source of truth for code | Issues link out to PRs and branches; Multica does not host repos or run CI. |
| Becoming a generic chat product | Threads are scoped to issues. No DMs, no channels, no `#general`. |
| Multi-tenant SaaS at enterprise scale (SSO/SCIM/audit-export) today | Out of scope until self-hosting and small-team UX are battle-tested. |
| Mobile-native apps | Web is a PWA (installable, offline shell). No iOS/Android binaries planned. |
| Marketplace for skills / agents | Skills are workspace-local. No import-from-registry, no public catalog. |
| Generic workflow automation builder | Triggers are narrow and code-defined (`on_assign`, `on_comment`, `on_mention`, schedule). No node-based automation editor. |
| Time tracking / billing / forecasting | Multica tracks tokens and tasks per runtime; it does not project velocity or estimate dates. |

## 6. Capabilities — Current State

A condensed view of every capability the product exposes today. Detail and acceptance criteria live in the per-feature appendices.

### 6.1 Identity & access
- **Email-OTP login** + dev-bypass route. Users are created on first login.
- **Personal access tokens** (`mul_` prefix, 90-day default) for CLI and API.
- **Daemon tokens** (`mdt_` prefix) — long-lived, scoped to a daemon owner.
- **Workspaces** with roles `owner | admin | member`. Slug-addressable.
- **Bot users** — synthetic members so integration-generated comments have a named author.

### 6.2 Issues & collaboration
- Statuses: `backlog | todo | in_progress | in_review | done | blocked | cancelled`. Priorities: `urgent | high | medium | low | none`.
- Polymorphic assignee (`member | agent`), polymorphic creator (`member | agent | webhook`).
- Parent/sub-issues, board ordering (`position`), due dates, labels (many-to-many).
- Comments are threaded (`parent_id`), support reactions, attachments, and `@mentions` of members and agents.
- Acceptance criteria as structured JSONB with an `approved | pending` gate.
- Reactions on issues and comments; attachments via pre-signed URLs (S3 / CloudFront).
- Per-issue subscriber list with auto-subscription on create/assign/comment.

### 6.3 Agents
- Per-workspace, with `name`, `avatar`, `description`, `instructions`, `tools`, `triggers`, `providers`, `max_concurrent_tasks`, `visibility` (`workspace | private`, default private), `archived` flag.
- Triggers: `on_assign`, `on_comment`, `on_mention` (mention chain prevents loops).
- Provider list (e.g. `["claude_code", "codex"]`) with an optional default; tasks dispatch to any online runtime matching the provider.

### 6.4 Tasks & verification
- Task lifecycle: `queued → dispatched → running → completed | failed | cancelled`.
- Streaming `TaskMessage` log per task (text, tool use, tool result, errors).
- Optional **verification loop**: when an issue has a `verifier_agent`, the system runs `criteria → executor → validator → rework` until pass, fail, or max-rounds (`max_verification_rounds`, default 5).
- Active-task tracking per issue; cancel endpoint.

### 6.5 Runtimes & daemons
- User-owned `daemon` (one per machine) registers and heartbeats. Daemon ↔ workspace assignment is explicit (`daemon_workspace` table) — one daemon can serve many workspaces.
- Each daemon publishes `runtime` rows per detected agent CLI. Runtimes auto-detect `claude`, `codex`, `opencode`, `cursor` on PATH.
- Daemon ping and update flows (server initiates → daemon executes → reports result).
- Per-runtime daily usage tracking (token counts per model).
- Status visibility per daemon and per runtime in **Settings → Runtimes**.

### 6.6 Real-time, inbox, activity
- WebSocket hub with rooms keyed by `workspace_id`; auto-resubscribes on reconnect.
- Server broadcasts on every state change (issue update, comment, task progress, etc.).
- **Inbox** for routed notifications with severities `info | attention | action_required`. Mark-read, archive, archive-completed bulk ops.
- **Activity log** per issue, merged with comments into a unified timeline.
- **Telegram** notifications: per-user channels (DM) and per-workspace group chat.

### 6.7 Skills
- Workspace-scoped reusable instruction objects with markdown `content`, optional `config` JSONB, and a bundled file set (`skill_file`).
- Many-to-many attachment to agents; passed to the daemon in the task claim payload.
- Import endpoint (`POST /api/skills/import`) exists; source of imports is not yet fixed.

### 6.8 Inputs from the outside world
- **Webhooks** — `whk_` token auth, per-webhook adapter (`standard`, `oss-alert`), per-webhook deduplication window, configurable `create_issue` actions with template substitution.
- **GitHub App** — installation-based; receives `push | pull_request | issues | issue_comment`; per-event rules map events to an agent and a workspace; templated issue creation. Inbound only.
- **Recurring templates** — cron-scheduled issues with timezone, optional `max_runs`, subscriber list, dispatch hints.

### 6.9 Developer ergonomics
- `multica` CLI: `login`, `daemon start|stop|status|logs`, `workspace`, `issue` CRUD/assign/status, `issue comment`, `issue runs`, `issue run-messages --since`, `agent list`, `config`, `update`.
- Profiles (`--profile`) for running multiple daemons against different servers from one machine.
- Self-hosting via `docker-compose.prod.yml`; releases ship multi-platform binaries via GoReleaser and Homebrew tap.

### 6.10 Web app
- Next.js 16 App Router; PWA-installable with an offline shell.
- Routes: landing, login, dashboard (home, board, issues, my-issues, inbox, daemons, agents, skills, settings), workspace-slug-prefixed issue routes.
- Zustand stores per feature domain; WebSocket-driven optimistic sync.

## 7. Success Metrics

> Targets below are **proposed** (not measured yet). Add a metrics pipeline before claiming success.

### 7.1 Leading indicators (days–weeks)
- **Agent-task pickup rate**: % of agent-assigned issues that result in at least one running task within 60 seconds. Target ≥ 95% (anything lower indicates daemon/runtime health problems).
- **Self-host setup completion**: % of users who run `make setup && make dev` and successfully assign one issue to an agent within 30 minutes.
- **Verification loop usage**: % of issues created with a `verifier_agent` configured. Target ≥ 20% in workspaces that have ever used the feature.
- **Inbox actionability**: median time-to-read for `action_required` items < 1 hour during work hours.

### 7.2 Lagging indicators (weeks–months)
- **Tasks per workspace per week** — primary throughput metric. Stretch: 100/week in a 3-person team.
- **% of workspaces with ≥ 3 active agents** in any given week.
- **Skill reuse ratio** — average times a skill is referenced across distinct tasks. Stretch ≥ 5.
- **Active daemons per workspace** — measures whether teams keep agents running, not just experimenting.

### 7.3 Counter-metrics
- **Task failure rate.** Should trend down as users learn the tool; sustained high failures suggest the agent UX is too permissive about bad inputs.
- **`blocked` issue count.** Persistent growth suggests agents are stuck waiting on humans — surface in dashboards.

## 8. Open Questions

- **[Product] Pricing & hosted offering.** README points to `multica.ai/app` for cloud. Pricing model (per seat? per task? per agent?) is undecided.
- **[Eng] Cloud runtimes.** `runtime_mode` was originally `local | cloud`, but the column was removed when agent decoupled from runtime. The cloud-runtime path is unbuilt — what's the milestone?
- **[Product] Skills marketplace.** `POST /api/skills/import` exists; the import source is not specified. Is this a GitHub repo? An internal registry? Or just upload-from-zip?
- **[Eng] WebSocket scale.** Hub is in-process; horizontal scaling will need a fanout (Redis pub/sub, NATS).
- **[Product] Permissions granularity.** Today `owner | admin | member`. Is there demand for "viewer" or per-project ACLs?
- **[Eng] Mobile.** PWA only — when (if ever) does native become worth it?
- **[Legal] Telemetry & data residency.** Self-host story is strong; cloud story needs a written policy. Telegram bot token, GitHub App ID, S3 credentials all live in workspace settings JSONB — clarify which fields are PII.
- **[Eng] Verification loop guardrails.** Default max rounds is 5; should it be workspace-configurable? What happens after `blocked`?

## 9. Roadmap — Phase View

The roadmap is described in phases that match what the code reveals, not by calendar quarter.

### 9.1 Phase 1 — Shipped (core product)
Everything in §6 is in `main`. The product is usable end-to-end: log in, install daemon, create agent, assign issue, watch streaming task output, review, mark done.

### 9.2 Phase 2 — In-flight / observable gaps
- **Webhook event log UI.** Backend stores `webhook_event_log`; no viewer is wired in the web app.
- **Multi-action webhooks.** Backend supports multiple actions per webhook; the dialog only manages one default `create_issue` action.
- **Verifier max-rounds policy.** Defaulted to 5; not yet configurable per workspace.
- **Skill import surface.** Endpoint exists; UX for "where do skills come from" is undefined.
- **GitHub write-back.** Today the integration is inbound only — closing an issue in Multica does not move the GitHub PR/issue.
- **Cloud runtime support.** Schema and protocol leave room; no implementation.

### 9.3 Phase 3 — Future considerations
- **Permissions: viewer role**, per-project ACLs, agent-token scoping.
- **Skill marketplace / cross-workspace skill sharing.**
- **Agent SDK for custom providers** beyond claude / codex / opencode / cursor.
- **Horizontal-scale WebSocket fanout** (Redis / NATS).
- **Audit-export / SOC2 prep** for enterprise self-host.

## 10. Architecture Reference (for context)

```
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
│   Next.js    │────>│  Go Backend  │────>│   PostgreSQL     │
│   Frontend   │<────│  (Chi + WS)  │<────│   (pgvector)     │
└──────────────┘     └──────┬───────┘     └──────────────────┘
                            │
                     ┌──────┴────────┐
                     │ Agent Daemons │  (one or many, on user machines)
                     │ claude/codex/ │
                     │ opencode/...  │
                     └───────────────┘
```

| Layer | Stack |
|---|---|
| Frontend | Next.js 16 App Router, React 19, TypeScript, Tailwind, shadcn |
| State | Zustand per feature; React Context only for WS lifecycle |
| Backend | Go 1.26, Chi router, sqlc, pgx, gorilla/websocket |
| Database | PostgreSQL 17 + pgvector |
| Storage | S3 (attachments) + optional CloudFront signing |
| Agent runtime | Local `multica daemon`; spawns CLI (claude / codex / opencode / cursor) |
| Auth | JWT (HS256) for users; `whk_` for webhooks; `mdt_` for daemons; `mul_` for PATs |
| Release | GoReleaser → GitHub Releases + Homebrew tap; Docker images to ghcr.io; deploy job runs on `v*` tag push |

See [`docs/system-design.md`](../system-design.md) for the canonical data-model diagram.

## 11. Glossary

| Term | Definition |
|---|---|
| **Workspace** | Tenant boundary. All data scoped here. |
| **Member** | User with a workspace role. |
| **Agent** | AI worker, polymorphic assignee. |
| **Daemon** | Local process owned by a user; hosts one or more runtimes. |
| **Runtime** | A specific agent CLI on a specific daemon (e.g. `claude` on daemon `laptop-1`). |
| **Provider** | Type of agent runtime (`claude_code`, `codex`, `opencode`, `cursor`). |
| **Task** | Single execution unit on the agent task queue. |
| **TaskMessage** | A streamed chunk of agent output (text / tool use / tool result / error). |
| **Skill** | Reusable instruction bundle attached to an agent. |
| **Verifier agent** | Agent configured to produce acceptance criteria and validate work. |
| **PAT** | Personal access token (`mul_` prefix). |
| **Daemon token** | `mdt_` prefix; authenticates a daemon to the server. |
| **Webhook token** | `whk_` prefix; authenticates an external system to a workspace webhook endpoint. |
