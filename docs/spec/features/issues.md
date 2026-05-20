# Issues, Comments, Labels, Attachments

## Problem

Tracking work for a mixed human-and-agent team needs all the usual issue-tracker primitives (status, priority, assignee, comments, labels) — plus a polymorphic assignee so agents and members are interchangeable on the board, plus a structured `acceptance_criteria` so an agent has a machine-readable definition of "done."

## Goals

1. **Linear-grade board ergonomics** without inventing new vocabulary (statuses, priorities, labels, subscribers, reactions, threads).
2. **Polymorphic everything** — assignee, creator, comment author, reaction actor, subscriber can all be a `member` or an `agent`.
3. **Threaded comments** that an agent can post into without special-casing.
4. **Structured acceptance criteria** that drive the verification loop in [tasks-and-verification.md](tasks-and-verification.md) when a verifier is configured.
5. **Mentions that do work**, not just notify — `@AgentName` in a comment can dispatch a task; mention chain depth is bounded.

## Non-Goals

- Sprint / cycle planning.
- Estimation, story points, velocity.
- Custom fields per workspace.
- Multi-step workflows beyond the fixed status set.

## Personas & user stories

- *As a tech lead*, I want a board where humans and agents share the same columns so I can see total throughput, not two separate views.
- *As an agent author*, I want my agent to be able to post a comment, attach a file, and change status using the same APIs members use.
- *As a reviewer*, I want a way to write acceptance criteria once, and have the system enforce them at the end.
- *As a member*, I want `@AgentName` in a comment to actually queue work for that agent.
- *As an admin*, I want bulk update / delete to clear out stale backlog quickly.

## Requirements

### Issue entity

| Req | Detail | Priority | Status |
|---|---|---|---|
| Statuses | `backlog | todo | in_progress | in_review | done | blocked | cancelled` | P0 | Shipped |
| Priorities | `urgent | high | medium | low | none` | P0 | Shipped |
| Polymorphic assignee | `assignee_type ∈ {member, agent}` + `assignee_id` (nullable for unassigned) | P0 | Shipped |
| Polymorphic creator | `creator_type ∈ {member, agent, webhook}` + `creator_id` | P0 | Shipped |
| Parent / sub-issues | `parent_issue_id` FK; `GET /issues/{id}/sub-issues` lists children | P0 | Shipped |
| Board ordering | `position FLOAT`; drag-to-reorder | P0 | Shipped |
| Due date | `due_date TIMESTAMPTZ` | P0 | Shipped |
| Numbered display | `issue_number` per workspace (e.g. `MUL-123`) — see migration 020 | P0 | Shipped |
| Acceptance criteria | `acceptance_criteria JSONB` (array of `{id, title, check, severity}`); `criteria_status ∈ {pending, approved}` | P0 | Shipped |
| Context refs | `context_refs JSONB` — array of references the agent should consult | P0 | Shipped |
| Dev links | `issue_dev_links` — links out to PRs, branches, deploys | P0 | Shipped |
| Verifier agent | `verifier_agent_id` + `verification_phase` + `verification_round` + `max_verification_rounds` | P0 | Shipped |
| Batch update | `POST /api/issues/batch-update` | P0 | Shipped |
| Batch delete | `POST /api/issues/batch-delete` | P0 | Shipped |
| Cross-workspace resolution | `GET /api/issues/{id}/resolve` returns the workspace slug for an issue id (used by CLI / deep links) | P0 | Shipped |

### Comments

| Req | Detail | Priority | Status |
|---|---|---|---|
| Polymorphic author | `member | agent` | P0 | Shipped |
| Comment types | `comment | status_change | progress_update | system` | P0 | Shipped |
| Threading | `parent_id` FK with cascading delete (migration 018) | P0 | Shipped |
| Update / delete | `PUT/DELETE /api/comments/{commentId}` | P0 | Shipped |
| Reactions | `comment_reaction` — actor + emoji; unique per actor/comment/emoji | P0 | Shipped |
| Attachments | Attached via `attachment` table — `attachment_to_comment` or `attachment_to_issue` | P0 | Shipped |
| System comments | Special `author_type='system'` (migration 054) for events without a human/agent actor | P0 | Shipped |
| Mentions | `[@Name](mention://agent/<id>)` or `mention://user/<id>` markdown link → parsed by `util.ParseMentions` | P0 | Shipped |

### Labels

| Req | Detail | Priority | Status |
|---|---|---|---|
| Workspace-level CRUD | `/api/labels` | P0 | Shipped |
| Many-to-many | `issue_to_label` join table | P0 | Shipped |
| Per-issue management | `GET/PUT /api/issues/{id}/labels` | P0 | Shipped |

### Reactions

| Req | Detail | Priority | Status |
|---|---|---|---|
| On issues | `POST/DELETE /api/issues/{id}/reactions` | P0 | Shipped |
| On comments | `POST/DELETE /api/comments/{commentId}/reactions` | P0 | Shipped |

### Subscribers & timeline

| Req | Detail | Priority | Status |
|---|---|---|---|
| Auto-subscribe on create / assign / comment | `member` and `agent` creators auto-subscribed | P0 | Shipped |
| Manual subscribe / unsubscribe | `POST /api/issues/{id}/subscribe`, `/unsubscribe` | P0 | Shipped |
| List subscribers | `GET /api/issues/{id}/subscribers` | P0 | Shipped |
| Unified timeline | `GET /api/issues/{id}/timeline` — comments + activity log merged | P0 | Shipped |

### Attachments

| Req | Detail | Priority | Status |
|---|---|---|---|
| Pre-signed upload | `POST /api/upload-file` returns the URL | P0 | Shipped |
| Attach to issue or comment | `attachment` row stored with FK | P0 | Shipped |
| Download | `GET /api/attachments/{id}/download` — auth-gated, workspace-resolved by handler | P0 | Shipped |
| List on issue | `GET /api/issues/{id}/attachments` | P0 | Shipped |
| Delete | `DELETE /api/attachments/{id}` | P0 | Shipped |
| Storage | S3 backend, optional CloudFront URL signing (see `auth.NewCloudFrontSignerFromEnv`) | P0 | Shipped |

### Mention chain (loop prevention)

| Req | Detail | Priority | Status |
|---|---|---|---|
| Agent mention triggers a task | When `@AgentName` is in a new comment, the system enqueues a task for that agent with `trigger_comment_id` | P0 | Shipped |
| Chain count | `issue.agent_mention_chain_count` increments per agent-to-agent mention | P0 | Shipped |
| Chain generation | `agent_mention_chain_generation BIGINT` — logical clock; humans can reset it | P0 | Shipped |
| Max chain depth | Hard cap enforced before enqueue (prevents agent ping-pong) | P0 | Shipped |
| Human reset | Posting a human comment increments generation, clearing the chain | P0 | Shipped |

## Data model

```
issue
├── id, workspace_id, issue_number, title, description
├── status, priority, position, due_date
├── assignee_type, assignee_id (polymorphic)
├── creator_type, creator_id (polymorphic; member|agent|webhook)
├── parent_issue_id
├── acceptance_criteria JSONB[], criteria_status
├── verifier_agent_id, verification_phase, verification_round, max_verification_rounds
├── last_verification_result JSONB
├── context_refs JSONB[]
├── agent_mention_chain_count, agent_mention_chain_generation
├── created_at, updated_at

comment (threaded)
├── id, issue_id, workspace_id
├── author_type (member|agent|system), author_id
├── parent_id (self-FK, cascade delete)
├── type (comment|status_change|progress_update|system)
├── content

issue_label / issue_to_label
issue_dependency (blocks | blocked_by | related)
issue_subscriber (recipient_type, recipient_id)
issue_reaction / comment_reaction (actor, emoji)
attachment (issue or comment, uploader, filename, url, content_type, size_bytes)
issue_dev_link (type, url, label)
```

## API surface (summary)

```
GET    /api/issues
POST   /api/issues
POST   /api/issues/batch-update
POST   /api/issues/batch-delete

GET    /api/issues/{id}
PUT    /api/issues/{id}
DELETE /api/issues/{id}
GET    /api/issues/{id}/sub-issues
GET    /api/issues/{id}/timeline

POST   /api/issues/{id}/comments
GET    /api/issues/{id}/comments

GET    /api/issues/{id}/subscribers
POST   /api/issues/{id}/subscribe
POST   /api/issues/{id}/unsubscribe

GET    /api/issues/{id}/active-task
POST   /api/issues/{id}/tasks/{taskId}/cancel
GET    /api/issues/{id}/task-runs

PUT    /api/issues/{id}/criteria
POST   /api/issues/{id}/criteria/approve
POST   /api/issues/{id}/criteria/reject

GET    /api/issues/{id}/labels
PUT    /api/issues/{id}/labels

POST   /api/issues/{id}/reactions
DELETE /api/issues/{id}/reactions

GET    /api/issues/{id}/attachments

PUT    /api/comments/{commentId}
DELETE /api/comments/{commentId}
POST   /api/comments/{commentId}/reactions
DELETE /api/comments/{commentId}/reactions

GET    /api/labels
POST   /api/labels
PUT    /api/labels/{id}
DELETE /api/labels/{id}

DELETE /api/attachments/{id}
GET    /api/attachments/{id}/download

GET    /api/issues/{id}/resolve   (cross-workspace)
```

## Acceptance criteria (spec-level)

- Given an issue with no assignee, when a member assigns it to an agent, then a task is enqueued and the WS hub emits `issue.updated`.
- Given a comment containing `[@A](mention://agent/<id>)`, when posted by a human, then a task is enqueued for agent A with `trigger_comment_id` set and `chain_generation` advanced.
- Given a comment containing `[@B](mention://agent/<id>)`, when posted by agent A and chain depth is below the cap, then a task is enqueued for agent B.
- Given a comment containing `[@B](mention://agent/<id>)`, when posted by agent A and chain depth equals the cap, then no task is enqueued and a system comment notes the cap was hit.
- Given an issue with a `verifier_agent_id` set, when assigned, the system enters the verification loop (see [tasks-and-verification.md](tasks-and-verification.md)).
- Given an attachment, when a non-member downloads via the signed URL, then the server returns 403 (workspace membership is checked at fetch time).

## Open questions

- Should `position` reorder be coarsened (LexoRank-style) — the FLOAT approach risks precision loss after many reorders?
- Should issues link out to runs in the **board column UI** by default (saves a click)?
- Should the **mention chain cap** be workspace-configurable?
- Webhook-authored issues set `creator_type='webhook'`; the inbox routing assumes member/agent creators auto-subscribe. Confirm there is no notification gap for "this webhook-created issue has no subscribers" (today the assigned agent and team are still routed).
