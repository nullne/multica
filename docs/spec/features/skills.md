# Skills

## Problem

Solving the same class of problem twice with an AI agent is waste. Most engineering work — deploying a service, running a migration, opening a PR, writing a code-review summary — has a small set of recurring patterns. If those patterns are captured once and re-attached to future agents, every successful run makes the next one cheaper. Skills are Multica's primitive for that capture.

## Goals

1. **First-class workspace object** — skills are versioned data, not prompt strings copy-pasted into agent config.
2. **Markdown + bundled files** — a skill is a markdown instruction plus an optional file tree (templates, scripts, schemas).
3. **Many-to-many attachment** — multiple agents can share a skill; an agent can carry multiple skills.
4. **Delivered at task time** — the daemon receives the full skill payload in the task claim response, so the agent CLI has it on disk before execution.
5. **Importable** — a `POST /api/skills/import` endpoint exists so skills can be brought in from external sources.

## Non-Goals

- Public marketplace / cross-workspace sharing.
- Versioning skills with history / rollback (today the latest write wins).
- Auto-generating a skill from a successful run.
- Skill execution sandboxing.

## Personas & user stories

- *As a tech lead*, I want to write a "deploy" skill once and attach it to every agent that ships code.
- *As an engineer*, I want to drop a `runbook.md` and `migrate.sh` into a skill and have agents pick them up.
- *As an admin*, I want to import a skill from a known internal source (e.g. a GitHub URL).
- *As an agent author*, I want to update a skill and have all future agent runs use the new version.

## Requirements

### Skill entity

| Req | Detail | Priority | Status |
|---|---|---|---|
| Workspace-scoped | `workspace_id` FK | P0 | Shipped |
| Name, description | Standard fields | P0 | Shipped |
| Markdown content | `content TEXT` | P0 | Shipped |
| Config blob | `config JSONB` — optional skill-specific config | P0 | Shipped |
| File tree | `skill_file` rows: `(skill_id, path, content)` | P0 | Shipped |
| Many-to-many to agents | `agent_skill` join table | P0 | Shipped |

### CRUD

| Endpoint | Notes |
|---|---|
| `GET /api/skills` | List workspace skills |
| `POST /api/skills` | Admin/owner only |
| `POST /api/skills/import` | Admin/owner only — import flow (source TBD) |
| `GET /api/skills/{id}` | Detail |
| `PUT /api/skills/{id}` | Update markdown + config |
| `DELETE /api/skills/{id}` | Delete |
| `GET /api/skills/{id}/files` | List files |
| `PUT /api/skills/{id}/files` | Upsert (path-keyed) |
| `DELETE /api/skills/{id}/files/{fileId}` | Delete single file |
| `GET /api/agents/{id}/skills` | List skills attached to agent |
| `PUT /api/agents/{id}/skills` | Replace attachments (set semantics) |

### Runtime delivery

- When a task is claimed by a daemon, the response includes the full `AgentSkillData[]` for every skill attached to the agent.
- The daemon materializes those skills into the worktree before invoking the CLI, in a path the CLI is configured to read (provider-dependent — Claude Code uses `.claude/`, Codex uses its own convention).

### Visibility

- Skills are workspace-local. There is no per-skill visibility flag.
- Any member can read; only admin/owner can create / update / delete.

## Data model

```
skill
├── id, workspace_id, name, description
├── content TEXT (markdown)
├── config JSONB
├── created_at, updated_at

skill_file
├── id, skill_id, path TEXT, content TEXT
├── created_at, updated_at

agent_skill
├── agent_id, skill_id
├── created_at
```

## Acceptance criteria

- Given an agent with skill `deploy` attached, when a task is dispatched to that agent, then the daemon receives `deploy.content` and all `deploy.skill_file[]` rows in the task claim payload.
- Given a skill update, when the next task starts, then the daemon receives the updated content (no caching of stale versions).
- Given a `PUT /api/agents/{id}/skills` with `[]`, then all skill attachments are removed; the agent runs with no skills next time.
- Given a member without admin/owner role, when they POST `/api/skills`, then the request is rejected with 403.

## Open questions

- **Import source** — what does `POST /api/skills/import` actually accept today? A URL? A zip? A GitHub raw link? Specify and document.
- Should there be a **public catalog** of curated skills (analogous to Anthropic Skills) or a way to share across workspaces in the same self-host?
- Should skill updates **emit an event** so currently-running tasks can be notified (or at minimum, the user sees "your agent is still using v1")?
- Should skills support **dependencies on other skills**?
- Should `config` JSONB have a **schema validator** (today it's free-form)?
