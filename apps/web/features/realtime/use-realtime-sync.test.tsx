import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

const listRecentIssuesMock = vi.fn();
const listActiveTasksMock = vi.fn();
const listIssuesMock = vi.fn();
const listMembersMock = vi.fn();
const listAgentsMock = vi.fn();
const listSkillsMock = vi.fn();
const listInboxMock = vi.fn();
const listLabelsMock = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    listRecentIssues: (...args: unknown[]) => listRecentIssuesMock(...args),
    listActiveTasks: (...args: unknown[]) => listActiveTasksMock(...args),
    listIssues: (...args: unknown[]) => listIssuesMock(...args),
    listMembers: (...args: unknown[]) => listMembersMock(...args),
    listAgents: (...args: unknown[]) => listAgentsMock(...args),
    listSkills: (...args: unknown[]) => listSkillsMock(...args),
    listInbox: (...args: unknown[]) => listInboxMock(...args),
    listLabels: (...args: unknown[]) => listLabelsMock(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), info: vi.fn() },
}));

import { useRecentsStore } from "@/features/navigation/recents-store";
import { useRealtimeSync } from "./use-realtime-sync";

interface FakeWSClient {
  onAny: ReturnType<typeof vi.fn>;
  on: ReturnType<typeof vi.fn>;
  onReconnect: ReturnType<typeof vi.fn>;
  triggerReconnect: () => Promise<void>;
}

function makeFakeWS(): FakeWSClient {
  let reconnectCb: (() => void | Promise<void>) | null = null;
  return {
    onAny: vi.fn(() => () => {}),
    on: vi.fn(() => () => {}),
    onReconnect: vi.fn((cb: () => void | Promise<void>) => {
      reconnectCb = cb;
      return () => {
        if (reconnectCb === cb) reconnectCb = null;
      };
    }),
    triggerReconnect: async () => {
      if (reconnectCb) await reconnectCb();
    },
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  useRecentsStore.getState().clear();
  // The hook fires off many parallel refetches on reconnect — give every
  // one a safe noop default so the awaited Promise.all settles.
  listRecentIssuesMock.mockResolvedValue({ issues: [], total: 0 });
  listActiveTasksMock.mockResolvedValue([]);
  listIssuesMock.mockResolvedValue({ issues: [], total: 0 });
  listMembersMock.mockResolvedValue([]);
  listAgentsMock.mockResolvedValue([]);
  listSkillsMock.mockResolvedValue([]);
  listInboxMock.mockResolvedValue([]);
  listLabelsMock.mockResolvedValue([]);
});

describe("useRealtimeSync reconnect", () => {
  it("refetches the Recents store after a WS reconnect", async () => {
    // Seed the recents store as if the sidebar had already loaded data,
    // so refresh() has a workspace set + mine flag to replay.
    await useRecentsStore
      .getState()
      .fetch(["ws-1", "ws-2"], { mine: true, userId: "user-1" });
    const callsBeforeReconnect = listRecentIssuesMock.mock.calls.length;

    const ws = makeFakeWS();
    renderHook(() => useRealtimeSync(ws as unknown as Parameters<typeof useRealtimeSync>[0]));

    await ws.triggerReconnect();

    await waitFor(() => {
      expect(listRecentIssuesMock.mock.calls.length).toBeGreaterThan(
        callsBeforeReconnect,
      );
    });
    // The reconnect refetch must reuse the loaded mine flag, otherwise a
    // user who toggled "Only my issues" would silently get unfiltered data
    // back after a reconnect.
    const reconnectCalls = listRecentIssuesMock.mock.calls.slice(
      callsBeforeReconnect,
    );
    expect(reconnectCalls).toHaveLength(2);
    for (const call of reconnectCalls) {
      expect(call[0]).toMatchObject({ mine: true });
    }
  });

  it("does not call the Recents endpoint on reconnect if nothing was loaded", async () => {
    const ws = makeFakeWS();
    renderHook(() => useRealtimeSync(ws as unknown as Parameters<typeof useRealtimeSync>[0]));

    await ws.triggerReconnect();

    expect(listRecentIssuesMock).not.toHaveBeenCalled();
  });
});
