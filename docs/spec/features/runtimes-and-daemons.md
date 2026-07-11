# Runtimes, Daemons, CLI

## Problem

Coding agents (Claude Code, Codex, opencode, cursor) need to run somewhere the user trusts with their source tree and credentials — usually their own laptop or a workstation. Multica must let a user install one binary, authenticate once, and have all their machines available as compute for agent work, without opening inbound ports or sharing credentials with the cloud.

## Goals

1. **Single binary, single command** — `brew install multica` then `multica login && multica daemon start`.
2. **Pull-only** — daemons never accept inbound connections; they poll an authenticated endpoint.
3. **Per-machine, multi-workspace** — one daemon serves any number of workspaces the user belongs to.
4. **Auto-detect available CLIs** — the daemon registers one runtime per detected provider.
5. **Visibility** — Settings → Daemons / Runtimes shows liveness, CLI version, recent activity, daily token usage.
6. **Self-service updates and pings** — admin can poke a daemon to verify health or force an update without SSHing in.

## Non-Goals

- Hosted (cloud) runtimes today.
- A web-based terminal for the daemon.
- Sandboxing the agent CLI beyond what the CLI itself does.
- Auto-installing missing agent CLIs.

## Personas & user stories

- *As an engineer*, I want to install Multica, log in, and have my Mac show up as compute in five minutes.
- *As an admin*, I want to see which of my team's machines are online and what they're running.
- *As an oncall engineer*, I want to "ping" a daemon and see whether the agent CLI is healthy.
- *As an engineer*, I want to silence a daemon for a specific workspace without uninstalling it.
- *As a user with multiple Multica servers (e.g. self-host + cloud)*, I want a profile-based daemon so each server has its own state.

## Requirements

### Daemon lifecycle

| Req | Detail | Priority | Status |
|---|---|---|---|
| User-owned daemons | Each `daemon` row has `user_id` (since migration 060). Daemons are personal compute. | P0 | Shipped |
| Workspace assignment | `daemon_workspace` join table; user toggles which of their workspaces the daemon serves | P0 | Shipped |
| Daemon token | `mdt_` prefix; minted at first registration; stored on the daemon | P0 | Shipped |
| Register / deregister | `POST /api/daemon/register`, `POST /api/daemon/deregister` | P0 | Shipped |
| Heartbeat | `POST /api/daemon/heartbeat` every 15s by default | P0 | Shipped |
| Status fields | `id`, `daemon_id`, `status`, `last_heartbeat_at`, `cli_version`, `device_info JSONB`, `archived_at` | P0 | Shipped |
| Archive / restore | `POST /api/daemons/{daemonId}/archive`, `/restore` (migration 046) | P0 | Shipped |
| Get / update | `GET /api/daemons/{daemonId}`, `PATCH /api/daemons/{daemonId}` | P0 | Shipped |
| Env inspection | `GET /api/daemons/{daemonId}/env` — returns the env vars the server expects this daemon to run with | P0 | Shipped |
| Force update across workspace | `POST /api/workspaces/{id}/update-all-daemons` | P0 | Shipped |

### Runtimes

| Req | Detail | Priority | Status |
|---|---|---|---|
| Auto-detect CLIs | Daemon scans PATH for `claude`, `codex`, `opencode`, `cursor` at startup | P0 | Shipped |
| Register runtime per CLI per workspace | One `agent_runtime` row keyed by `(workspace_id, daemon_id, provider)` | P0 | Shipped |
| Status | `online | offline` driven by heartbeat freshness | P0 | Shipped |
| Per-runtime daily usage | `runtime_usage` row per (runtime, date, model) with token counters | P0 | Shipped |
| List runtimes | `GET /api/runtimes` | P0 | Shipped |
| Get usage | `GET /api/runtimes/{runtimeId}/usage` | P0 | Shipped |
| Get recent activity | `GET /api/runtimes/{runtimeId}/activity` | P0 | Shipped |
| Ping (health check) | Server requests, daemon executes, reports result: `POST /api/runtimes/{runtimeId}/ping` → `GET /ping/{pingId}` | P0 | Shipped |
| Update (force CLI / daemon upgrade) | `POST /api/runtimes/{runtimeId}/update` → `GET /update/{updateId}` | P0 | Shipped |
| Workspace root | `MULTICA_WORKSPACES_ROOT` (default `~/multica_workspaces`) — per-task worktree dir | P0 | Shipped |

### Daemon ↔ server protocol

```
POST   /api/daemon/register           (body: daemon_id, device_info, cli_versions[])
POST   /api/daemon/deregister
POST   /api/daemon/heartbeat

POST   /api/daemon/runtimes/{runtimeId}/tasks/claim
GET    /api/daemon/runtimes/{runtimeId}/tasks/pending
POST   /api/daemon/runtimes/{runtimeId}/usage         (daily token report)
POST   /api/daemon/runtimes/{runtimeId}/ping/{pingId}/result
POST   /api/daemon/runtimes/{runtimeId}/update/{updateId}/result

GET    /api/daemon/tasks/{taskId}/status
POST   /api/daemon/tasks/{taskId}/start
POST   /api/daemon/tasks/{taskId}/progress
POST   /api/daemon/tasks/{taskId}/complete
POST   /api/daemon/tasks/{taskId}/fail
POST   /api/daemon/tasks/{taskId}/messages
GET    /api/daemon/tasks/{taskId}/messages
```

All daemon routes are authenticated by `mdt_` token (not JWT, not workspace header). The middleware sets the daemon identity from the token.

### CLI

The `multica` binary is the user-facing surface for daemons, with bonus issue / workspace commands so the CLI can be used as a scripting target.

| Command | Status |
|---|---|
| `multica login` (browser flow) | Shipped |
| `multica login --token` | Shipped |
| `multica auth status / logout` | Shipped |
| `multica daemon start [--foreground]` | Shipped |
| `multica daemon stop / status / logs [-f] [-n N]` | Shipped |
| `multica workspace list / get / members / watch / unwatch` | Shipped |
| `multica issue list / get / create / update / assign / status` | Shipped |
| `multica issue comment list / add / delete` | Shipped |
| `multica issue runs / run-messages [--since N]` | Shipped |
| `multica agent list` | Shipped |
| `multica config show / set <key> <value>` | Shipped |
| `multica version / update` | Shipped |
| `multica --profile <name> …` (multi-server isolation) | Shipped |

### Daemon config (env / flags)

| Setting | Flag | Env | Default |
|---|---|---|---|
| Poll interval | `--poll-interval` | `MULTICA_DAEMON_POLL_INTERVAL` | `3s` |
| Heartbeat interval | `--heartbeat-interval` | `MULTICA_DAEMON_HEARTBEAT_INTERVAL` | `15s` |
| Agent timeout | `--agent-timeout` | `MULTICA_AGENT_TIMEOUT` | `2h` |
| Max concurrent tasks | `--max-concurrent-tasks` | `MULTICA_DAEMON_MAX_CONCURRENT_TASKS` | `20` |
| Daemon ID | `--daemon-id` | `MULTICA_DAEMON_ID` | hostname |
| Device name | `--device-name` | `MULTICA_DAEMON_DEVICE_NAME` | hostname |
| Runtime name | `--runtime-name` | `MULTICA_AGENT_RUNTIME_NAME` | `Local Agent` |
| Workspaces root | — | `MULTICA_WORKSPACES_ROOT` | `~/multica_workspaces` |
| Claude path | — | `MULTICA_CLAUDE_PATH` | (auto-discover) |
| Claude model | — | `MULTICA_CLAUDE_MODEL` | (CLI default) |
| Codex path | — | `MULTICA_CODEX_PATH` | (auto-discover) |
| Codex model | — | `MULTICA_CODEX_MODEL` | (CLI default) |

## Data model

```
daemon
├── id, user_id, daemon_id, status, cli_version, device_info JSONB
├── archived_at, archived_by
├── last_heartbeat_at, created_at, updated_at

daemon_workspace (M:N)
├── daemon_id, workspace_id, enabled, created_at

agent_runtime
├── id, workspace_id, daemon_id, provider
├── name, status (online | offline)
├── last_heartbeat_at, cli_version, capabilities JSONB

runtime_usage
├── runtime_id, date, model
├── input_tokens, output_tokens, cache_read_tokens, cache_write_tokens

daemon_ping  / daemon_update (server-initiated, daemon-executed)
├── id, runtime_id, status, requested_at, completed_at, result JSONB
```

## Acceptance criteria

- Given a fresh daemon, when `multica daemon start` runs, then within `MULTICA_DAEMON_POLL_INTERVAL` the server shows the daemon as `online` with one runtime per detected CLI.
- Given a daemon whose last heartbeat is older than 3 × `MULTICA_DAEMON_HEARTBEAT_INTERVAL`, then its runtimes are marked `offline` and pending tasks for those runtimes do not get re-dispatched until a new heartbeat arrives.
- Given a user removed from a workspace, then the corresponding `daemon_workspace` rows lose access and the daemon stops polling for that workspace.
- Given a runtime archived via the admin UI, then it no longer appears in the assignee runtime selection.
- Given a ping initiated by the server, the daemon must report back within a fixed timeout (today: ping-specific; see `runtime_ping.go`) — otherwise the ping is `failed`.

## Open questions

- Should daemon-CLI mismatch (e.g. server expects `claude >= 1.5` but daemon has older) **fail the runtime registration** or just warn?
- Should `runtime_usage` aggregation by **workspace and user** be precomputed instead of computed on the fly?
- For workspaces with no online runtime, should the **assignee picker disable agent assignment** with a message, or allow the task to queue silently?
- Should the **CLI emit machine-readable progress events** (e.g. JSON line on stderr) so wrappers / scripts can react?
