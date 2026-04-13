/**
 * TestApiClient — lightweight API helper for E2E test data setup/teardown.
 *
 * Uses raw fetch so E2E tests have zero build-time coupling to the web app.
 */

import pg from "pg";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? `http://localhost:${process.env.PORT ?? "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";

interface TestWorkspace {
  id: string;
  name: string;
  slug: string;
}

interface CachedAuth {
  token: string;
  workspaceId: string;
}

let cachedAuth: CachedAuth | null = null;

export class TestApiClient {
  private token: string | null = null;
  private workspaceId: string | null = null;
  private createdIssueIds: string[] = [];

  async login(email: string, name: string) {
    if (cachedAuth) {
      this.token = cachedAuth.token;
      return { token: cachedAuth.token };
    }

    // Step 1: Send verification code (may be rate-limited)
    await fetch(`${API_BASE}/auth/send-code`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    });

    // Step 2: Read code from database; if all codes are used, insert one directly
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      let result = await client.query(
        "SELECT code FROM verification_code WHERE email = $1 AND used = FALSE AND expires_at > now() ORDER BY created_at DESC LIMIT 1",
        [email]
      );

      if (result.rows.length === 0) {
        const code = String(Math.floor(100000 + Math.random() * 900000));
        await client.query(
          "INSERT INTO verification_code (email, code, expires_at) VALUES ($1, $2, now() + interval '10 minutes')",
          [email, code]
        );
        result = { rows: [{ code }] } as typeof result;
      }

      const code = result.rows[0].code;

      // Step 3: Verify code to get JWT
      const verifyRes = await fetch(`${API_BASE}/auth/verify-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, code }),
      });
      const data = await verifyRes.json();
      this.token = data.token;
      cachedAuth = { token: this.token!, workspaceId: "" };

      if (name && data.user?.name !== name) {
        await this.authedFetch("/api/me", {
          method: "PATCH",
          body: JSON.stringify({ name }),
        });
      }

      return data;
    } finally {
      await client.end();
    }
  }

  async getWorkspaces(): Promise<TestWorkspace[]> {
    const res = await this.authedFetch("/api/workspaces");
    return res.json();
  }

  setWorkspaceId(id: string) {
    this.workspaceId = id;
  }

  async ensureWorkspace(name = "E2E Workspace", slug = "e2e-workspace") {
    if (cachedAuth?.workspaceId) {
      this.workspaceId = cachedAuth.workspaceId;
      return { id: cachedAuth.workspaceId, name, slug };
    }

    const workspaces = await this.getWorkspaces();
    const workspace = workspaces.find((item) => item.slug === slug) ?? workspaces[0];
    if (workspace) {
      this.workspaceId = workspace.id;
      if (cachedAuth) cachedAuth.workspaceId = workspace.id;
      else cachedAuth = { token: this.token!, workspaceId: workspace.id };
      return workspace;
    }

    const res = await this.authedFetch("/api/workspaces", {
      method: "POST",
      body: JSON.stringify({ name, slug }),
    });
    if (res.ok) {
      const created = (await res.json()) as TestWorkspace;
      this.workspaceId = created.id;
      if (cachedAuth) cachedAuth.workspaceId = created.id;
      else cachedAuth = { token: this.token!, workspaceId: created.id };
      return created;
    }

    const refreshed = await this.getWorkspaces();
    const created = refreshed.find((item) => item.slug === slug) ?? refreshed[0];
    if (created) {
      this.workspaceId = created.id;
      if (cachedAuth) cachedAuth.workspaceId = created.id;
      else cachedAuth = { token: this.token!, workspaceId: created.id };
      return created;
    }

    throw new Error(`Failed to ensure workspace ${slug}: ${res.status} ${res.statusText}`);
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
