import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Label } from "@/shared/types";

let currentWorkspaceId: string | null = null;
const mockListLabels = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    getWorkspaceId: () => currentWorkspaceId,
    listLabels: (...args: unknown[]) => mockListLabels(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

import { useLabelStore } from "./store";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function makeLabel(id: string, workspaceId: string): Label {
  return {
    id,
    workspace_id: workspaceId,
    name: id,
    color: "#000000",
  };
}

describe("useLabelStore workspace freshness", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentWorkspaceId = null;
    useLabelStore.setState({ labels: [], loading: true });
  });

  it("does not let a stale workspace fetch overwrite current labels", async () => {
    const oldFetch = deferred<Label[]>();
    currentWorkspaceId = "ws-old";
    mockListLabels.mockReturnValueOnce(oldFetch.promise);

    const oldPromise = useLabelStore.getState().fetch();
    await Promise.resolve();

    currentWorkspaceId = "ws-new";
    mockListLabels.mockResolvedValueOnce([makeLabel("new-label", "ws-new")]);
    await useLabelStore.getState().fetch();

    oldFetch.resolve([makeLabel("old-label", "ws-old")]);
    await oldPromise;

    expect(useLabelStore.getState().labels.map((label) => label.id)).toEqual([
      "new-label",
    ]);
  });
});
