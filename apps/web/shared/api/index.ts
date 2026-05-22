import { createLogger } from "@/shared/logger";
import { ApiClient } from "./client";
import type { RoutineTrigger } from "@/shared/types";

export { ApiClient } from "./client";
export type { LoginResponse } from "./client";
export { WSClient } from "./ws-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "";

export const api = new ApiClient(API_BASE_URL, { logger: createLogger("api") });

export async function regenerateRoutineTriggerToken(
  routineId: string,
  triggerId: string,
): Promise<{ trigger: RoutineTrigger; token: string }> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (typeof window !== "undefined") {
    const token = localStorage.getItem("multica_token");
    const workspaceId = localStorage.getItem("multica_workspace_id");
    if (token) headers.Authorization = `Bearer ${token}`;
    if (workspaceId) headers["X-Workspace-ID"] = workspaceId;
  }

  const res = await fetch(`${API_BASE_URL}/api/routines/${routineId}/triggers/${triggerId}/regenerate-token`, {
    method: "POST",
    headers,
    credentials: "include",
  });
  if (!res.ok) {
    const data = await res.json().catch(() => null) as { error?: string } | null;
    throw new Error(data?.error || `API error: ${res.status} ${res.statusText}`);
  }
  return res.json() as Promise<{ trigger: RoutineTrigger; token: string }>;
}

// Initialize token from localStorage on load
if (typeof window !== "undefined") {
  const token = localStorage.getItem("multica_token");
  if (token) {
    api.setToken(token);
  }
  const wsId = localStorage.getItem("multica_workspace_id");
  if (wsId) {
    api.setWorkspaceId(wsId);
  }

}
