# Targeted UI Verification with Playwright (Screenshots / Recording)

How to manually verify a single UI feature against the running dev environment
using Playwright browser automation — taking step-by-step screenshots to walk
through a flow. This complements (does not replace) the E2E suite: use it to
eyeball a feature you just built, reproduce a UI bug, or capture screenshots
for a PR description.

Agents with Playwright MCP tools (`browser_navigate`, `browser_snapshot`,
`browser_click`, `browser_take_screenshot`, `browser_run_code_unsafe`, ...)
should follow this flow. The same steps work from a hand-written Playwright
script.

## 1. Prerequisites

- `make dev` is running: frontend on `http://localhost:3000`, backend on
  `http://localhost:8080`.
- Dev auth bypass is enabled on the backend (default in `make dev`), so
  `POST /auth/dev` issues a token without a real login.

## 2. Log in (inject a dev token)

Navigate to `http://localhost:3000` first (any page on the origin), then run
in the page context:

```js
const r = await fetch("http://localhost:8080/auth/dev", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ email: "e2e@multica.ai", name: "E2E User" }),
});
const { token } = await r.json();
localStorage.setItem("multica_token", token);

const workspaces = await (
  await fetch("http://localhost:8080/api/workspaces", {
    headers: { Authorization: `Bearer ${token}` },
  })
).json();
localStorage.setItem("multica_workspace_id", workspaces[0].id);
```

Then navigate to `/issues` and wait for `[data-slot="sidebar"]` to become
visible (auth init settled). This mirrors `loginAsDefault` in `e2e/helpers.ts`.

## 3. Create exactly the data the feature needs

Call the REST API from the page context with the token from localStorage.
Always send `Authorization: Bearer <token>` and `X-Workspace-ID`:

```js
const headers = {
  "Content-Type": "application/json",
  Authorization: `Bearer ${localStorage.getItem("multica_token")}`,
  "X-Workspace-ID": localStorage.getItem("multica_workspace_id"),
};

// e.g. an agent assignee (needed by agent-only UI such as dispatch-after)
await fetch("http://localhost:8080/api/agents", {
  method: "POST",
  headers,
  body: JSON.stringify({ name: "Claude", providers: ["claude"] }),
});

// e.g. an issue in a specific state
await fetch("http://localhost:8080/api/issues", {
  method: "POST",
  headers,
  body: JSON.stringify({ title: "Demo", assignee_type: "agent", assignee_id: "..." }),
});
```

Reload the page after seeding so the zustand stores pick the data up.

## 4. Drive the UI

- Take an accessibility snapshot first and interact via element refs/roles —
  do not guess selectors from pixels.
- Useful entry points:
  - `Cmd/Ctrl+Shift+O` — open the create-issue modal.
  - `/issues/<id>` — issue detail page (properties panel pickers).
- For icon-only buttons, locate by the lucide icon class, e.g.
  `svg.lucide-timer` then `xpath=ancestor::button[1]`.

## 5. Capture evidence

- **Screenshots**: capture one per meaningful step (before / interaction /
  after). A sequence of 3–5 screenshots reads like a short recording and is
  usually enough to demonstrate a flow.
- **Recording**: the Playwright MCP plugin cannot record video. If a real
  video is required, write a throwaway spec and run it with video enabled:

```bash
pnpm exec playwright test e2e/tests/my-demo.spec.ts --headed
# playwright.config: use: { video: "on" } → video saved under test-results/
```

- Verify outcomes with both the DOM (text content, disabled states) and the
  screenshot — a screenshot alone can hide a wrong state.

## 6. Clean up

Delete whatever you created (`DELETE /api/issues/<id>`, archive agents) so
the shared dev workspace stays clean. Screenshots saved into the repo root
are untracked artifacts — delete them after use.

## Gotchas

- **Stale JS from the PWA service worker**: in older builds the service
  worker cached `/_next/` chunks and kept serving outdated code even after
  container restarts. Dev now auto-unregisters the worker, but if a browser
  profile still shows old UI, clear it manually:

```js
for (const r of await navigator.serviceWorker.getRegistrations()) await r.unregister();
for (const k of await caches.keys()) await caches.delete(k);
```

- **Verify the served bundle when behavior looks impossible**: fetch the page
  chunks with `cache: "no-store"` and grep for a symbol you just added. If
  the symbol is missing the browser is running old code — fix the caching
  problem before debugging your feature.
- **Time-dependent UI**: the dev backend runs schedulers on real time (e.g.
  the issue dispatch scheduler ticks every 60s) — seed timestamps far enough
  in the future that the state does not flip mid-verification.
