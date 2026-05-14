import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Daemon, Workspace } from "@/shared/types";

const mockListRuntimes = vi.fn();
const mockListDaemons = vi.fn();
const mockGetProviderConfig = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    listRuntimes: (...args: unknown[]) => mockListRuntimes(...args),
    listDaemons: (...args: unknown[]) => mockListDaemons(...args),
    getProviderConfig: (...args: unknown[]) => mockGetProviderConfig(...args),
  },
}));

const workspace: Workspace = {
  id: "ws-1",
  name: "Workspace 1",
  slug: "ws-1",
  description: null,
  context: null,
  settings: {},
  repos: [],
  issue_prefix: "MUL",
  github_connected: false,
  created_at: "2026-05-06T00:00:00Z",
  updated_at: "2026-05-06T00:00:00Z",
};

vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: {
    getState: () => ({ workspace }),
  },
}));

import { useRuntimeStore } from "./store";

function makeDaemon(overrides: Partial<Daemon> & Pick<Daemon, "id">): Daemon {
  const { id, ...rest } = overrides;
  return {
    id,
    user_id: "u-1",
    workspace_visible: true,
    daemon_id: id,
    status: "online",
    cli_version: "1.0.0",
    device_name: id,
    device_info: "",
    labels: [],
    metadata: {},
    last_seen_at: null,
    created_at: "",
    updated_at: "",
    archived_at: null,
    ...rest,
  };
}

describe("useRuntimeStore.fetchAll default selection", () => {
  const initialState = useRuntimeStore.getState();

  beforeEach(() => {
    vi.clearAllMocks();
    useRuntimeStore.setState({
      ...initialState,
      daemons: [],
      runtimes: [],
      enabledProviders: [],
      selectedDaemonId: "",
      selectedId: "",
      fetching: true,
    });
    mockListRuntimes.mockResolvedValue([]);
    mockGetProviderConfig.mockResolvedValue({});
  });

  it("defaults selection to the first workspace-visible daemon", async () => {
    mockListDaemons.mockResolvedValue([
      makeDaemon({
        id: "personal-1",
        workspace_visible: false,
      }),
      makeDaemon({ id: "ws-1" }),
    ]);

    await useRuntimeStore.getState().fetchAll();

    expect(useRuntimeStore.getState().selectedDaemonId).toBe("ws-1");
  });

  it("leaves selection empty when only personal daemons exist", async () => {
    mockListDaemons.mockResolvedValue([
      makeDaemon({ id: "personal-1", workspace_visible: false }),
      makeDaemon({ id: "personal-2", workspace_visible: false }),
    ]);

    await useRuntimeStore.getState().fetchAll();

    // Personal daemons must stay hidden until the user expands the
    // collapsible Personal section — auto-opening the detail pane on one
    // would defeat the purpose of the new layout.
    expect(useRuntimeStore.getState().selectedDaemonId).toBe("");
  });

  it("ignores archived workspace daemons when picking a default", async () => {
    mockListDaemons.mockResolvedValue([
      makeDaemon({ id: "ws-archived", archived_at: "2026-05-01T00:00:00Z" }),
      makeDaemon({ id: "ws-active" }),
    ]);

    await useRuntimeStore.getState().fetchAll();

    expect(useRuntimeStore.getState().selectedDaemonId).toBe("ws-active");
  });

  it("preserves an existing valid selection across refetches", async () => {
    mockListDaemons.mockResolvedValue([
      makeDaemon({ id: "ws-1" }),
      makeDaemon({ id: "personal-1", workspace_visible: false }),
    ]);
    useRuntimeStore.setState({ selectedDaemonId: "personal-1" });

    await useRuntimeStore.getState().fetchAll();

    // User-driven selections (even of personal daemons) are preserved.
    expect(useRuntimeStore.getState().selectedDaemonId).toBe("personal-1");
  });
});
