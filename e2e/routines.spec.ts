import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";
import { TestApiClient } from "./fixtures";

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ?? `http://localhost:${process.env.PORT ?? "8080"}`;

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

  await page.getByRole("button", { name: /Run now/i }).click();

  await expect(page.locator('a[href^="/issues/"]').filter({ hasText: routine.name })).toBeVisible();
  await expect(page.getByText("Manual").last()).toBeVisible();
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
