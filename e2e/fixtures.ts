/**
 * TestApiClient — lightweight API helper for E2E test data setup/teardown.
 *
 * Uses raw fetch so E2E tests have zero build-time coupling to the web app.
 */

import pg from "pg";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? `http://localhost:${process.env.PORT ?? "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";

// Cache tokens by email to avoid repeated logins across tests.
const tokenCache = new Map<string, { token: string; workspaceId: string }>();

interface TestWorkspace {
  id: string;
  name: string;
  slug: string;
}

export class TestApiClient {
  private token: string | null = null;
  private workspaceId: string | null = null;
  private email: string | null = null;
  private createdIssueIds: string[] = [];

  async login(email: string, name: string) {
    this.email = email;

    // Reuse cached token to avoid repeated auth round-trips across tests.
    const cached = tokenCache.get(email);
    if (cached) {
      this.token = cached.token;
      this.workspaceId = cached.workspaceId;
      return { token: cached.token };
    }

    const loginRes = await fetch(`${API_BASE}/auth/dev`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, name }),
    });
    const data = await loginRes.json();
    if (!loginRes.ok || !data.token) {
      throw new Error(`dev login failed: ${loginRes.status} (is DEV_AUTH_BYPASS=1 set on the backend?)`);
    }
    this.token = data.token;

    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      if (name && data.user?.name !== name) {
        await this.authedFetch("/api/me", {
          method: "PATCH",
          body: JSON.stringify({ name }),
        });
      }
    } finally {
      await client.end();
    }

    return data;
  }

  async getWorkspaces(): Promise<TestWorkspace[]> {
    const res = await this.authedFetch("/api/workspaces");
    return res.json();
  }

  setWorkspaceId(id: string) {
    this.workspaceId = id;
  }

  async ensureWorkspace(name = "E2E Workspace", slug = "e2e-workspace") {
    const workspaces = await this.getWorkspaces();
    const workspace = workspaces.find((item) => item.slug === slug) ?? workspaces[0];
    if (workspace) {
      this.workspaceId = workspace.id;
      this.cacheToken();
      return workspace;
    }

    const res = await this.authedFetch("/api/workspaces", {
      method: "POST",
      body: JSON.stringify({ name, slug }),
    });
    if (res.ok) {
      const created = (await res.json()) as TestWorkspace;
      this.workspaceId = created.id;
      this.cacheToken();
      return created;
    }

    const refreshed = await this.getWorkspaces();
    const created = refreshed.find((item) => item.slug === slug) ?? refreshed[0];
    if (created) {
      this.workspaceId = created.id;
      this.cacheToken();
      return created;
    }

    throw new Error(`Failed to ensure workspace ${slug}: ${res.status} ${res.statusText}`);
  }

  /** Cache the current token so subsequent TestApiClient instances can reuse it. */
  private cacheToken() {
    if (this.token && this.workspaceId && this.email) {
      tokenCache.set(this.email, { token: this.token, workspaceId: this.workspaceId });
    }
  }

  async createIssue(title: string, opts?: Record<string, unknown>) {
    const res = await this.authedFetch("/api/issues", {
      method: "POST",
      body: JSON.stringify({ title, ...opts }),
    });
    const issue = await res.json();
    this.createdIssueIds.push(issue.id);
    return issue;
  }

  async deleteIssue(id: string) {
    await this.authedFetch(`/api/issues/${id}`, { method: "DELETE" });
  }

  /** Clean up all issues created during this test. */
  async cleanup() {
    for (const id of this.createdIssueIds) {
      try {
        await this.deleteIssue(id);
      } catch {
        /* ignore — may already be deleted */
      }
    }
    this.createdIssueIds = [];
  }

  getToken() {
    return this.token;
  }

  getWorkspaceId() {
    return this.workspaceId;
  }

  async getProviderConfig(): Promise<{ providers?: Record<string, { enabled: boolean; api_key: string; target_version?: string }> }> {
    const res = await this.authedFetch(`/api/workspaces/${this.workspaceId}/providers`);
    return res.json();
  }

  async updateProviderConfig(data: { providers?: Record<string, { enabled: boolean; api_key: string; target_version?: string }> }) {
    const res = await this.authedFetch(`/api/workspaces/${this.workspaceId}/providers`, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
    return res.json();
  }

  private async authedFetch(path: string, init?: RequestInit) {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((init?.headers as Record<string, string>) ?? {}),
    };
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`;
    if (this.workspaceId) headers["X-Workspace-ID"] = this.workspaceId;
    return fetch(`${API_BASE}${path}`, { ...init, headers });
  }
}
