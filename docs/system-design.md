# Multica System Design

AI-native task management platform. Agents are first-class: they get assigned issues, comment, change status, and run code — just like human teammates.

## Core Concepts

### Identity & Tenancy

| Concept | Description |
|---------|-------------|
| **User** | Global identity (email login). Belongs to workspaces via membership. |
| **Workspace** | Tenant boundary. All data (issues, agents, runtimes) is workspace-scoped. |
| **Member** | User ↔ Workspace binding with a role (owner / admin / member). |

### Work

| Concept | Description |
|---------|-------------|
| **Issue** | Unit of work. Has status, priority, assignee (member or agent), optional parent issue, acceptance criteria. |
| **Comment** | Threaded discussion on an issue. Authors can be members or agents. Supports replies via `parent_id`. |
| **Label** | Workspace-level tag attached to issues (many-to-many). |
| **Subscriber** | User/agent watching an issue. Auto-subscribed on create, assign, or comment. Drives inbox notifications. |

### Agents & Execution

| Concept | Description |
|---------|-------------|
| **Agent** | A configured AI worker. Bound to a runtime, has skills, triggers, and instructions. Polymorphic assignee alongside members. |
| **Runtime** | A registered agent CLI instance (claude, codex, opencode, cursor). Belongs to a daemon. Identified by `(workspace_id, daemon_id, provider)`. |
| **Daemon** | A local process running on a machine. Manages one or more runtimes. First-class DB entity with status, CLI version, and device info. Heartbeats and updates are daemon-scoped. |
| **Task** | A job dispatched to an agent for a specific issue. Lifecycle: queued → dispatched → running → completed/failed/cancelled. |
| **Task Message** | Streaming output from a running task (text, tool use, tool result, errors). |
| **Skill** | Reusable instruction set attached to agents. Contains markdown content + optional files. |

### Workspace Config

| Concept | Description |
|---------|-------------|
| **Provider Config** | Workspace-level settings for code agent providers. Stored in workspace `settings` JSONB. Includes enable/disable, API keys, target versions. |
| **Workspace Repos** | Git repositories linked to the workspace. Stored in workspace `repos` JSONB. Used by daemons to set up worktrees. |

### Notifications & Activity

| Concept | Description |
|---------|-------------|
| **Inbox Item** | Notification routed to a user. Types: issue_assigned, status_changed, new_comment, task_failed, etc. Has severity (info / action_required). |
| **Activity Log** | Audit trail entry for an issue (created, status changed, assigned, etc.). Merged with comments into a unified timeline. |

### Auth & Tokens

| Concept | Description |
|---------|-------------|
| **Verification Code** | Email-based OTP for login. Creates user on-the-fly if not found. |
| **Personal Access Token (PAT)** | Long-lived `mul_`-prefixed token for CLI/API access. User-scoped. |
| **Daemon Token** | `mdt_`-prefixed token for daemon ↔ server auth. Workspace-scoped. |

## Relationships

```
Workspace
├── Members ← User (role: owner/admin/member)
├── Issues
│   ├── Comments (threaded)
│   ├── Subscribers (auto-managed)
│   ├── Labels (many-to-many)
│   ├── Reactions
│   ├── Activity Log
│   └── Attachments
├── Agents
│   ├── → Runtime (bound to one)
│   ├── → Skills (many-to-many)
│   ├── Triggers (on_assign / on_comment / scheduled)
│   └── Tasks
│       └── Task Messages (streaming output)
├── Daemons (one per physical machine)
│   └── Runtimes (one per provider on that machine)
│       └── Runtime Usage (daily token counts per model)
├── Skills (workspace-level, attached to agents)
├── Provider Config (in workspace.settings JSONB)
├── Inbox Items → routed to members
└── Daemon Tokens
```

## Data Flow

### Issue → Agent Execution

```
1. Issue created/assigned to agent
2. Task enqueued (agent_task_queue)
3. Daemon polls /api/daemon/tasks/claim
4. Daemon runs agent CLI (claude/codex/opencode)
5. Agent streams messages → daemon → POST /api/daemon/tasks/:id/messages
6. Agent completes → daemon reports → task status updated
7. Issue status synced automatically (in_progress → done)
8. WS events broadcast at each step → UI updates live
```

### Verification Loop (optional)

```
1. Issue has acceptance criteria + verifier agent
2. Executor agent completes work
3. System enqueues criteria-extraction task (role: criteria)
4. System enqueues verification task (role: validator)
5. If verification fails → rework task dispatched (role: rework)
6. Loop repeats up to max_verification_rounds
```

### Real-time Updates

```
Browser ←→ WebSocket ←→ Hub (rooms keyed by workspace_id)
                          ↑
              Handlers / TaskService / Event Bus
                          ↑
                   REST API mutations
```

All mutations (issue update, comment create, task progress) publish events to both:
- **WebSocket Hub** → browser clients in the workspace room
- **Internal Event Bus** → server-side listeners (subscribers, notifications, activity log)

## Key Design Decisions

- **Polymorphic assignees**: `assignee_type` + `assignee_id` on issues. Can be "member" or "agent".
- **Daemon is a first-class entity**: The `daemon` table stores status, CLI version, and device info. Heartbeats and updates are daemon-scoped (one call per machine, not per provider). Runtimes reference their daemon via `daemon_ref` FK.
- **Workspace-level provider config**: API keys and versions stored in workspace `settings` JSONB, not per-daemon. Daemons receive config at registration time.
- **Pull-based task dispatch**: Daemons poll for tasks (no push). Simple, reliable, NAT-friendly.
- **Event-driven side effects**: Subscriber management, inbox notifications, and activity logging are all driven by the internal event bus, decoupled from handlers.
