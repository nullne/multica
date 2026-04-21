# Design: AGENTS.md-Centric Documentation Architecture

**Date:** 2026-04-21
**Status:** Draft — awaiting review
**Scope:** Agent-facing documentation only (human-contributor and operator docs are out of scope).

## Problem

Today we have 11 markdown files totaling ~3000 lines:

- `AGENTS.md` and `CLAUDE.md` are 270-line near-duplicates, each mixing project context, architecture, commands, coding rules, UI rules, testing, commit format, verification loop, and E2E patterns into one file.
- `CONTRIBUTING.md` (411 lines), `CLI_AND_DAEMON.md` (351 lines), `SELF_HOSTING.md` (298 lines) are topical long-form docs at the repo root.
- `docs/*.md` holds five design documents with no index from the entry point.

Symptoms a baseline evaluation confirms (numbers in "Baseline Evaluation" below):

- Agents over-list files to modify — mean precision 0.44 — because they lack hints on what *not* to touch.
- `docs/*.md` has zero discoverability. Without a router in the entry doc, the archive is invisible to agents.
- Context cost is high (mean 25.8k tokens per task), driven almost entirely by reading large implementation files to reconstruct cross-file abstractions.

## Goals

1. A single entry point (`AGENTS.md`) that routes any code agent to the right docs *and* the right code files quickly.
2. Docs record only what grep + code reading cannot reveal: cross-file abstractions, code-invisible invariants, multi-step operations.
3. Every active doc has a hard size cap. Top-level ≤ 120 lines. Deeper docs ≤ 200 lines.
4. Navigation depth ≤ 2. Any active doc is reachable in one hop from `AGENTS.md`.
5. Active navigation targets ≤ 5 files. History/design archives are one pointer away, not in the active tree.

## Non-goals

- Rewriting human-contributor docs (`CONTRIBUTING.md`, `README*`) — they serve a different audience.
- Rewriting operator docs (`SELF_HOSTING.md`).
- Building permanent CI tooling for the eval protocol — the lightweight parallel-subagent run is enough for now.

## Baseline Evaluation (2026-04-21)

Four tasks drawn from recent commits, run against the current `AGENTS.md` / `CLAUDE.md` with grep/read allowed and writes forbidden. Each task is executed by an independent subagent that outputs the file set it would modify; we score against the files actually changed in the originating commit.

| # | Task | Precision | Recall | Context (tok) |
|---|---|---|---|---|
| T1 | Per-repo override on GitHub event rules | 0.88 | 1.00 | 16.4k |
| T2 | Add `multica` CLI skill subcommands + agents page display | 0.57 | 0.80 | 38.0k |
| T3 | Webhook auto-labeling | 0.29 | 1.00 | 22.9k |
| T4 | Constrain dashboard layout to viewport height | 0.00 | 0.00 | 26.0k |
| — | **Mean** | **0.44** | **0.70** | **25.8k** |

Key findings:

1. **Zero `docs/` hits.** None of the four subagents opened any file under `docs/`. Without a router in the entry doc, the archive might as well not exist.
2. **Grep + domain-based file names already solve "locate".** Agents found the right files via `grep` on feature keywords plus conventional layout (`handler/<domain>.go`, `features/<domain>/`, `queries/<domain>.sql`). A file map would duplicate this.
3. **Missing invariants hurt precision.** T3's agent listed `queries/webhook.sql` and generated sqlc files because it did not know webhook action config is stored in the `webhook_actions.config` JSON column — no schema change is needed. A single invariant line would have saved three false positives.
4. **Context bloat comes from code, not docs.** T2 read ~3500 code lines to reconstruct how the skill command is wired. Compressing cross-file architecture in a doc is the lever, not compressing the doc itself.
5. **`AGENTS.md` vs `CLAUDE.md` filename did not matter.** Agents auto-loaded `CLAUDE.md`; identical content led to identical outcomes. Canonical filename is a compatibility decision, not a design decision.

**Methodology caveat.** All four commits had already merged into `main`. Agents observed the post-fix state of files, which confused T4 — the agent diagnosed the dashboard layout as "already correct" and chose a different file to modify. Synthetic (not-yet-implemented) tasks should be used for the next eval round; see "Evaluation Protocol".

## Design

### Information architecture

```
AGENTS.md                       ≤ 120 lines / ≤ 1200 words   ★ only mandatory doc
├─ Project one-liner            ~5 lines
├─ System map (text)            ~20 lines  — high-level modules and data flow,
│                                            prose only, no file listings
├─ Absolute invariants          ~25 lines  — bullets: workspace_id everywhere,
│                                            no compat layers, do not hand-edit
│                                            generated code, store dep direction,
│                                            polymorphic assignee, …
├─ Quickstart                   ~10 lines  — make dev / check / test
├─ Router                       ~30 lines  — three sections below, each a
│                                            one-line pointer per target doc
│    ## Understand the system
│    → docs/architecture.md
│    ## Standard multi-file operations
│    → docs/recipes/*.md
│    ## Design history (why)
│    → docs/design/
└─ Commit convention            ~10 lines

docs/
├─ architecture.md              ≤ 200 lines — cross-file abstractions only:
│                                TaskService orchestration, Hub broadcast flow,
│                                sqlc codegen chain, Zustand store dep direction,
│                                multi-tenancy filter layer, agent Backend iface
├─ recipes/                     multi-step ops, added when eval reveals the need
│   └─ add-migration.md         ≤ 50 lines — seed recipe: migrate + sqlc regen
│                                            + generated-code usage are linked
└─ design/                      archived historical design docs
    ├─ system.md
    ├─ webhook.md
    ├─ issue-verification-loop.md
    ├─ decouple-agent-runtime.md
    └─ docker-compose-best-practices.md
```

### Content principles

- **`AGENTS.md`** contains only what *every* agent needs on *every* task: invariants, commands, commit format, router. No "nice to have" sections, no domain-specific content.
- **`docs/architecture.md`** compresses abstractions the agent would otherwise reconstruct from reading 5+ files. One concrete line worth writing: *"TaskService orchestrates agent lifecycle — enqueue → claim → start → complete/fail. It syncs `issue.status` on each transition and broadcasts WS events."* That single sentence replaces reading `service/task.go` plus three handlers.
- **`docs/recipes/*.md`** captures only multi-file operations where an agent would otherwise miss a step. Added based on eval failures, not on anticipated needs.
- **`docs/design/*.md`** holds "why" content — architectural decisions, RFCs, historical context. It is reachable through one router pointer; individual files are not navigation targets.

### Relationship to existing files

| File | Action | Reason |
|---|---|---|
| `AGENTS.md` | rewrite, shrink to ≤ 120 lines | current content leaks into domains that belong elsewhere |
| `CLAUDE.md` | replace with a 1-line pointer to `AGENTS.md` | preserves Claude Code auto-load; removes duplication |
| `CONTRIBUTING.md` | keep as-is | human-contributor onboarding (env files, worktrees); out of scope |
| `CLI_AND_DAEMON.md` | fold architecture parts into `docs/architecture.md`, retire the top-level file | mixed-purpose content; low agent reach today |
| `SELF_HOSTING.md` | keep as-is | operator-facing; out of scope |
| `README.md`, `README.zh-CN.md` | keep as-is | marketing / first impression; not agent-facing |
| `docs/*.md` | move under `docs/design/` | archive; reachable via router, not direct nav target |

### Size budgets

| Layer | Hard cap | Rationale |
|---|---|---|
| `AGENTS.md` | 120 lines / 1200 words | read end-to-end by every agent on every task |
| `docs/architecture.md` | 200 lines | single-hop read; must not overflow context for a lookup |
| `docs/recipes/*.md` | 50 lines | steps, not essays |
| `docs/design/*.md` | no cap | reference-only; deep-read on demand |

Exceeding a hard cap requires splitting the file, not raising the cap.

## Evaluation Protocol

The baseline run documented above becomes the template for future runs.

**Changes from baseline methodology:**

- **Switch to synthetic, not-yet-implemented tasks** to avoid post-fix contamination. Seed examples:
  - "Add a `deadline` timestamp to Issue; surface it in the list view and in issue-detail."
  - "Add a `multica` CLI subcommand `daemon tail` that streams the local daemon's log."
  - "Add a WebSocket event `workspace.member_role_changed` and wire it into the inbox store."
  - "Add an E2E test verifying an invited member can see the workspace switcher after accepting."
- **Keep 4 task types:** one cross-end, one CLI + one end, one pure-backend, one pure-frontend.
- **Metrics unchanged:** precision, recall, total context tokens, doc share (fraction of tokens spent on docs vs code).
- **Thresholds for new skeleton:** mean precision ≥ 0.75, mean recall ≥ 0.85, mean context ≤ 18k tokens (≥ 30% reduction over the 25.8k baseline).
- **Execution:** four parallel subagents, identical instruction template, one eval run per skeleton revision.

Eval artifacts — the task descriptions, ground truth file sets, subagent outputs, and scoring — are committed to `docs/design/doc-architecture-eval.md` for reproducibility.

## Rollout

**Phase 1 — this spec (current PR).** Design document only. No file moves, no content rewrites. This document is the deliverable.

**Phase 2 — implementation (follow-up PR).** Apply the restructure in a single PR:

1. Write the new `AGENTS.md`, `docs/architecture.md`, and `docs/recipes/add-migration.md`.
2. Move existing `docs/*.md` into `docs/design/`.
3. Replace `CLAUDE.md` with a one-line pointer.
4. Delete `CLI_AND_DAEMON.md` after folding architecture content into `docs/architecture.md`.
5. Run the first post-change eval; commit artifacts to `docs/design/doc-architecture-eval.md`.

**Phase 3 — iterate.** If the second-run thresholds are missed, add a recipe or expand `docs/architecture.md` targeted at the failing task types. Repeat until thresholds pass. Stop iterating once passed — do not pre-emptively add recipes for scenarios the eval has not flagged.

## Open Questions

These require a call from the user before Phase 2 starts; defaults are noted.

1. **`CLI_AND_DAEMON.md` split.** The file mixes user-facing CLI usage ("how to run `multica daemon`") with daemon mechanics (polling, routing, provider registration). Default plan: move architecture content into `docs/architecture.md`; inline one-liners for "how to run" into `AGENTS.md` Quickstart; retire the file. Alternative: keep usage content under `docs/ops/cli.md` (introduces a third agent-facing top-level category).

2. **Chinese mirror.** `README.zh-CN.md` exists. Default: do **not** mirror agent-facing docs to zh-CN — agent context is English, and mirroring doubles maintenance cost without measurable agent benefit. Confirm before Phase 2.

3. **`SELF_HOSTING.md` reachability from the router.** Currently operator-facing and outside the agent tree. An agent working on a deploy task might still benefit from a pointer. Default: add one line under AGENTS.md Router (pointing to `SELF_HOSTING.md`), no content change. Confirm.
