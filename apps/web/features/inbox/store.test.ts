import { beforeEach, describe, expect, it, vi } from "vitest";
import type { InboxItem } from "@/shared/types";

let currentWorkspaceId: string | null = null;
const mockListInbox = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    getWorkspaceId: () => currentWorkspaceId,
    listInbox: (...args: unknown[]) => mockListInbox(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

import { useInboxStore } from "./store";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function makeInboxItem(id: string, workspaceId: string): InboxItem {
  return {
    id,
    workspace_id: workspaceId,
    recipient_type: "member",
    recipient_id: "user-1",
    actor_type: "member",
    actor_id: "user-2",
    type: "issue_assigned",
    severity: "info",
    issue_id: null,
    title: id,
    body: null,
    issue_status: null,
    read: false,
    archived: false,
    created_at: "2026-06-01T00:00:00Z",
    details: null,
  };
}

describe("useInboxStore workspace freshness", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentWorkspaceId = null;
    useInboxStore.setState({ items: [], loading: true });
  });

  it("does not let a stale workspace fetch overwrite current inbox items", async () => {
    const oldFetch = deferred<InboxItem[]>();
    currentWorkspaceId = "ws-old";
    mockListInbox.mockReturnValueOnce(oldFetch.promise);

    const oldPromise = useInboxStore.getState().fetch();
    await Promise.resolve();

    currentWorkspaceId = "ws-new";
    mockListInbox.mockResolvedValueOnce([makeInboxItem("new-item", "ws-new")]);
    await useInboxStore.getState().fetch();

    oldFetch.resolve([makeInboxItem("old-item", "ws-old")]);
    await oldPromise;

    expect(useInboxStore.getState().items.map((item) => item.id)).toEqual([
      "new-item",
    ]);
  });
});
