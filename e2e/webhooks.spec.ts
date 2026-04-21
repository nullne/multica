import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ?? `http://localhost:${process.env.PORT ?? "8080"}`;

interface CreatedWebhook {
  webhook: { id: string; source_type: string };
  token: string;
}

/**
 * Helper: create a token-authenticated webhook via the JSON API and return
 * the (id, token) pair so the test can drive the public ingest endpoint.
 */
async function createWebhook(api: TestApiClient, opts: { name: string; agentId: string }): Promise<CreatedWebhook> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const token = api.getToken();
  const wsId = api.getWorkspaceId();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  if (wsId) headers["X-Workspace-ID"] = wsId;

  const res = await fetch(`${API_BASE}/api/webhooks`, {
    method: "POST",
    headers,
    body: JSON.stringify({
      name: opts.name,
      source_type: "standard",
      agent_id: opts.agentId,
      title_template: "[hook] {{.title}}",
    }),
  });
  if (!res.ok) {
    throw new Error(`createWebhook failed: ${res.status} ${await res.text()}`);
  }
  return (await res.json()) as CreatedWebhook;
}

async function deleteWebhook(api: TestApiClient, id: string) {
  const headers: Record<string, string> = {};
  const token = api.getToken();
  const wsId = api.getWorkspaceId();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  if (wsId) headers["X-Workspace-ID"] = wsId;
  await fetch(`${API_BASE}/api/webhooks/${id}`, { method: "DELETE", headers });
}

async function listAgents(api: TestApiClient): Promise<{ id: string; name: string }[]> {
  const headers: Record<string, string> = {};
  const token = api.getToken();
  const wsId = api.getWorkspaceId();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  if (wsId) headers["X-Workspace-ID"] = wsId;
  const res = await fetch(`${API_BASE}/api/agents`, { headers });
  return res.json();
}

let api: TestApiClient;
let webhookId: string | null = null;

test.beforeEach(async () => {
  api = new TestApiClient();
  await api.login("e2e-webhooks@multica.ai", "Webhook Tester");
  await api.ensureWorkspace();
});

test.afterEach(async () => {
  if (webhookId) {
    try {
      await deleteWebhook(api, webhookId);
    } catch {
      /* ignore */
    }
    webhookId = null;
  }
  await api.cleanup();
});

test("standard webhook ingest creates an issue via the unified pipeline", async () => {
  const agents = await listAgents(api);
  test.skip(agents.length === 0, "no agent available in workspace");

  const created = await createWebhook(api, { name: "Standard Hook", agentId: agents[0]!.id });
  webhookId = created.webhook.id;

  // Send a standard payload — this exercises ingestWithWebhook end-to-end:
  // authenticate by token, parse via standard adapter, run create_issue
  // action, persist issue, log webhook_event_log row.
  const ingestRes = await fetch(`${API_BASE}/api/webhooks/${created.webhook.id}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${created.token}`,
    },
    body: JSON.stringify({
      title: "From E2E test",
      body: "automated",
      dedup_key: `e2e-${Date.now()}`,
    }),
  });
  expect(ingestRes.status).toBe(202);
  const ingestBody = await ingestRes.json();
  expect(ingestBody.created).toBe(1);
  expect(ingestBody.received).toBe(1);
});
