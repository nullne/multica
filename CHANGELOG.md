# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Releases use [Semantic Versioning](https://semver.org/).

---

## [0.1.4] - 2026-04-01

### My Issues & i18n

- My Issues page with kanban board, list view, and scope tabs
- Simplified Chinese localization for the landing page
- About and Changelog pages for the marketing site
- Agent avatar upload in settings
- Attachment support for CLI comments and issue/comment APIs
- Unified avatar rendering with ActorAvatar across all pickers
- SEO optimization and auth flow improvements for landing pages
- CLI defaults to production API URLs
- License changed to Apache 2.0

## [0.1.3] - 2026-03-31

### Agent Intelligence

- Trigger agents via @mention in comments
- Stream live agent output to issue detail page
- Rich text editor — mentions, link paste, emoji reactions, collapsible threads
- File upload with S3 + CloudFront signed URLs and attachment tracking
- Agent-driven repo checkout with bare clone cache for task isolation
- Batch operations for issue list view
- Environment authentication and security hardening

## [0.1.2] - 2026-03-28

### Collaboration

- Email verification login and browser-based CLI auth
- Multi-workspace environment with hot-reload
- Runtime dashboard with usage charts and activity heatmaps
- Subscriber-driven notification model replacing hardcoded triggers
- Unified activity timeline with threaded comment replies
- Kanban board redesign with drag sorting, filters, and display settings
- Human-readable issue identifiers (e.g. JIA-1)
- Skill import from ClawHub and Skills.sh

## [0.1.1] - 2026-03-25

### Core Platform

- Multi-workspace switching and creation
- Agent management UI with skills, tools, and triggers
- Unified agent SDK supporting Claude Code and Codex backends
- Comment CRUD with real-time WebSocket updates
- Task service layer and environment REST protocol
- Event bus with workspace-scoped WebSocket isolation
- Inbox notifications with unread badge and archive
- CLI with cobra subcommands for workspace and issue management

## [0.1.0] - 2026-03-22

### Foundation

- Go backend with REST API, JWT auth, and real-time WebSocket
- Next.js frontend with Linear-inspired UI
- Issues with board and list views and drag-and-drop kanban
- Agents, Inbox, and Settings pages
- One-click setup, migration CLI, and seed tool
- Comprehensive test suite — Go unit/integration, Vitest, Playwright E2E
