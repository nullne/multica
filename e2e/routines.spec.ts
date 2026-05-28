import { test, expect, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { loginAsDefault } from "./helpers";
import { TestApiClient } from "./fixtures";

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ?? `http://localhost:${process.env.PORT ?? "8080"}`;
const EVIDENCE_DIR = process.env.ROUTINES_EVIDENCE_DIR;

let api: TestApiClient;
let routineId: string | null = null;
let createdAgentId: string | null = null;

test.beforeEach(async ({ page }) => {
  api = new TestApiClient();
  await api.login("e2e-routines@multica.ai", "Routine Tester");
  await api.ensureWorkspace("E2E Workspace", "e2e-workspace");
  await loginAsDefault(page, api);
});

test.afterEach(async () => {
  if (routineId) {
    try {
      const runs = await authedFetch(`/api/routines/${routineId}/runs`);
      if (runs.ok) {
        const data = await runs.json() as { issue_id: string | null }[];
        for (const run of data) {
          if (run.issue_id) await api.deleteIssue(run.issue_id);
        }
      }
      await authedFetch(`/api/routines/${routineId}`, { method: "DELETE" });
    } catch {
      /* ignore cleanup failures */
    }
    routineId = null;
  }
  if (createdAgentId) {
    try {
      await authedFetch(`/api/agents/${createdAgentId}/archive`, { method: "POST" });
    } catch {
      /* ignore cleanup failures */
    }
    createdAgentId = null;
  }
  await api.cleanup();
});

test("routine detail can run now and shows processed history", async ({ page }) => {
  const agent = await ensureAgent();

  await page.goto("/routines");
  await expect(page.getByText("No routines yet")).toBeVisible();
  await captureEvidence(page, "01-empty-state");

  const createRes = await authedFetch("/api/routines", {
    method: "POST",
    body: JSON.stringify({
      name: `E2E routine ${Date.now()}`,
      instructions: "Created by routines e2e",
      priority: "medium",
      assignee_type: "agent",
      assignee_id: agent.id,
      enabled: true,
      triggers: [
        {
          trigger_type: "schedule",
          schedule: "0 9 * * 1",
          timezone: "UTC",
        },
      ],
    }),
  });
  expect(createRes.status).toBe(201);
  const routine = await createRes.json() as { id: string; name: string };
  routineId = routine.id;

  await page.goto(`/routines/${routine.id}`);
  await expect(page.getByText(routine.name)).toBeVisible();
  await expect(page.getByRole("heading", { name: "Runs" })).toBeVisible();
  await captureEvidence(page, "02-detail-before-run");

  await page.getByRole("button", { name: /Run now/i }).click();

  const issueLink = page.locator('a[href*="/issues/"]').filter({ hasText: routine.name });
  await expect(issueLink).toBeVisible();
  await expect(page.getByText("Manual").last()).toBeVisible();
  await captureEvidence(page, "03-processed-run-history");

  await page.goto("/issues");
  await expect(page.getByText(routine.name)).toBeVisible();
  await captureEvidence(page, "04-created-issue");

  if (EVIDENCE_DIR) {
    const memberApi = new TestApiClient();
    await memberApi.login("e2e-routines-member@multica.ai", "Routine Member");
    const memberRes = await authedFetch(`/api/workspaces/${api.getWorkspaceId()}/members`, {
      method: "POST",
      body: JSON.stringify({ email: "e2e-routines-member@multica.ai", role: "member" }),
    });
    expect([200, 201, 409]).toContain(memberRes.status);
    memberApi.setWorkspaceId(api.getWorkspaceId()!);

    await loginAsDefault(page, memberApi);
    await page.goto(`/routines/${routine.id}`);
    await expect(page.getByText(routine.name).first()).toBeVisible();
    await expect(page.getByText("Read-only")).toBeVisible();
    await expect(page.getByRole("button", { name: /Run now/i })).not.toBeVisible();
    await captureEvidence(page, "05-member-read-only");

    await writeEvidenceSummary([
      "Owner/admin can enter Routines and see the empty state.",
      "Routine detail shows trigger/template context before manual execution.",
      "Manual Run now creates an issue and records a processed run history entry.",
      "The created issue is visible in the Issues list.",
      "Regular members can view routines but see the read-only state without Run now.",
    ]);
  }
});

async function listAgents(): Promise<{ id: string; name: string }[]> {
  const res = await authedFetch("/api/agents");
  return res.json();
}

async function ensureAgent(): Promise<{ id: string; name: string }> {
  const agents = await listAgents();
  if (agents.length > 0) return agents[0]!;

  const res = await authedFetch("/api/agents", {
    method: "POST",
    body: JSON.stringify({
      name: `Routine E2E Agent ${Date.now()}`,
      description: "Created by routines e2e",
      instructions: "Help with routine-created issues.",
      providers: ["codex"],
      default_provider: "codex",
      visibility: "workspace",
      tools: [],
    }),
  });
  expect(res.status).toBe(201);
  const agent = await res.json() as { id: string; name: string };
  createdAgentId = agent.id;
  return agent;
}

async function authedFetch(path: string, init?: RequestInit) {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((init?.headers as Record<string, string>) ?? {}),
  };
  const token = api.getToken();
  const workspaceId = api.getWorkspaceId();
  if (token) headers.Authorization = `Bearer ${token}`;
  if (workspaceId) headers["X-Workspace-ID"] = workspaceId;
  return fetch(`${API_BASE}${path}`, { ...init, headers });
}

async function captureEvidence(page: Page, name: string) {
  if (!EVIDENCE_DIR) return;
  const screenshotDir = path.join(EVIDENCE_DIR, "screenshots");
  await mkdir(screenshotDir, { recursive: true });
  await page.screenshot({
    path: path.join(screenshotDir, `${name}.png`),
    fullPage: true,
  });
}

async function writeEvidenceSummary(checks: string[]) {
  if (!EVIDENCE_DIR) return;
  await mkdir(EVIDENCE_DIR, { recursive: true });
  const lines = [
    "# Routines UI Evidence",
    "",
    `Generated at: ${new Date().toISOString()}`,
    "",
    "## Screenshots",
    "",
    "- `screenshots/01-empty-state.png`",
    "- `screenshots/02-detail-before-run.png`",
    "- `screenshots/03-processed-run-history.png`",
    "- `screenshots/04-created-issue.png`",
    "- `screenshots/05-member-read-only.png`",
    "",
    "## Automated Checklist",
    "",
    ...checks.map((check) => `- [x] ${check}`),
    "",
    "## Playwright Artifacts",
    "",
    "Video, trace, and runner-managed screenshots are written under `playwright-output/` for this evidence run.",
    "",
  ];
  await writeFile(path.join(EVIDENCE_DIR, "summary.md"), lines.join("\n"));
}
