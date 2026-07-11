# Multica Product Specification

This directory is the product specification for Multica. It is reverse-engineered from the current codebase (`server/` and `apps/web/`) and is intended to describe **what is built today** plus the immediate roadmap implied by the code.

When code and spec disagree, the code is the source of truth — please update the spec.

## How to read this

- Start with **[multica.md](multica.md)** — the product-level PRD: vision, personas, capabilities, success metrics, non-goals.
- The per-feature appendices in **[features/](features/)** drill into each domain with detailed user stories, requirements, data model, APIs, and open questions.

## Index

| Document | Scope |
|---|---|
| [multica.md](multica.md) | Product-level PRD |
| [features/workspaces-and-auth.md](features/workspaces-and-auth.md) | Workspaces, members, auth, personal access tokens |
| [features/issues.md](features/issues.md) | Issues, comments, labels, reactions, attachments, mention chain |
| [features/agents.md](features/agents.md) | Agents, providers, triggers, instructions, bot users |
| [features/tasks-and-verification.md](features/tasks-and-verification.md) | Task lifecycle, dispatch, verification loop |
| [features/runtimes-and-daemons.md](features/runtimes-and-daemons.md) | Daemons, runtimes, CLI |
| [features/realtime-and-inbox.md](features/realtime-and-inbox.md) | WebSocket events, inbox, activity log, notifications |
| [features/skills.md](features/skills.md) | Reusable skills |
| [features/webhooks-and-integrations.md](features/webhooks-and-integrations.md) | Webhook ingest, GitHub App, Telegram |
| [features/recurring-templates.md](features/recurring-templates.md) | Cron-scheduled recurring issues |

## Conventions

- **P0 / P1 / P2** in requirements mean: must-have (shipped or required for the feature to function), nice-to-have (observable gaps or close fast-follow), future consideration (out of scope today, but documented to guide architecture).
- **Status: Shipped / Partial / Planned** is annotated on each requirement based on inspection of the current code.
- "Member" = human user with workspace membership. "Agent" = AI worker. "Actor" = either, when the distinction does not matter.
- Field names match the database (`snake_case`); API request/response fields match the handlers.
