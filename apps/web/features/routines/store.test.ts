import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Routine } from "@/shared/types";

let currentWorkspaceId: string | null = null;
const mockListRoutines = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    getWorkspaceId: () => currentWorkspaceId,
    listRoutines: (...args: unknown[]) => mockListRoutines(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

import { useRoutineStore } from "./store";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function makeRoutine(id: string, workspaceId: string): Routine {
  return {
    id,
    workspace_id: workspaceId,
    name: id,
    instructions: null,
    priority: "medium",
    assignee_type: null,
    assignee_id: null,
    due_date_offset_hours: null,
    dispatch_provider: null,
    dispatch_daemon_id: null,
    dispatch_daemon_label: null,
    github_auto_fix_enabled: false,
    enabled: true,
    created_by_id: "user-1",
    created_by_type: "member",
    subscriber_ids: [],
    label_ids: [],
    triggers: [],
    actions: [],
    created_at: "2026-06-01T00:00:00Z",
    updated_at: "2026-06-01T00:00:00Z",
  };
}

describe("useRoutineStore", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentWorkspaceId = null;
    useRoutineStore.setState({
      routines: [],
      loading: true,
      selectedId: null,
    });
  });

  it("does not let a stale workspace fetch overwrite current routines", async () => {
    const oldFetch = deferred<Routine[]>();
    currentWorkspaceId = "ws-old";
    mockListRoutines.mockReturnValueOnce(oldFetch.promise);

    const oldPromise = useRoutineStore.getState().fetch();
    await Promise.resolve();

    currentWorkspaceId = "ws-new";
    mockListRoutines.mockResolvedValueOnce([makeRoutine("new-routine", "ws-new")]);
    await useRoutineStore.getState().fetch();

    oldFetch.resolve([makeRoutine("old-routine", "ws-old")]);
    await oldPromise;

    expect(useRoutineStore.getState().routines.map((routine) => routine.id)).toEqual([
      "new-routine",
    ]);
  });

  it("keeps selectedId valid when routines are removed", () => {
    useRoutineStore.setState({
      routines: [makeRoutine("routine-1", "ws-1")],
      selectedId: "routine-1",
      loading: false,
    });

    useRoutineStore.getState().remove("routine-1");

    expect(useRoutineStore.getState().routines).toEqual([]);
    expect(useRoutineStore.getState().selectedId).toBeNull();
  });

  it("patches an existing routine without changing the selection", () => {
    useRoutineStore.setState({
      routines: [makeRoutine("routine-1", "ws-1")],
      selectedId: "routine-1",
      loading: false,
    });

    useRoutineStore.getState().patch("routine-1", { enabled: false });

    expect(useRoutineStore.getState().routines[0]?.enabled).toBe(false);
    expect(useRoutineStore.getState().selectedId).toBe("routine-1");
  });
});
