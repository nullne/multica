import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MyDaemonsTab } from "./my-daemons-tab";

const listMyDaemons = vi.fn();
const listMyDaemonWorkspaces = vi.fn();
const setMyDaemonWorkspaceEnabled = vi.fn();

vi.mock("@/shared/api", () => ({
  api: {
    listMyDaemons: (...args: unknown[]) => listMyDaemons(...args),
    listMyDaemonWorkspaces: (...args: unknown[]) => listMyDaemonWorkspaces(...args),
    setMyDaemonWorkspaceEnabled: (...args: unknown[]) =>
      setMyDaemonWorkspaceEnabled(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

describe("MyDaemonsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listMyDaemons.mockResolvedValue([
      {
        id: "daemon-1",
        user_id: "user-1",
        daemon_id: "host-a",
        status: "online",
        cli_version: "0.2.71",
        device_name: "Lex Studio",
        device_info: "Lex Studio",
        labels: [],
        metadata: {},
        last_seen_at: "2026-05-12T08:00:00Z",
        created_at: "2026-05-12T08:00:00Z",
        updated_at: "2026-05-12T08:00:00Z",
        archived_at: null,
      },
    ]);
    listMyDaemonWorkspaces.mockResolvedValue([
      { workspace_id: "ws-1", workspace_name: "Lex", enabled: true },
      { workspace_id: "ws-2", workspace_name: "Side Project", enabled: false },
    ]);
    setMyDaemonWorkspaceEnabled.mockResolvedValue(undefined);
  });

  it("loads the user's daemons and workspace assignments", async () => {
    render(<MyDaemonsTab />);

    expect((await screen.findAllByText("Lex Studio")).length).toBeGreaterThanOrEqual(
      1,
    );
    expect(screen.getByText("host-a")).toBeInTheDocument();
    expect(await screen.findByText("Lex")).toBeInTheDocument();
    expect(screen.getByText("Side Project")).toBeInTheDocument();

    expect(listMyDaemons).toHaveBeenCalledTimes(1);
    expect(listMyDaemonWorkspaces).toHaveBeenCalledWith("daemon-1");
  });

  it("updates a daemon workspace assignment", async () => {
    const user = userEvent.setup();
    render(<MyDaemonsTab />);

    const sideProjectSwitch = await screen.findByRole("switch", {
      name: /enable side project/i,
    });
    await user.click(sideProjectSwitch);

    expect(setMyDaemonWorkspaceEnabled).toHaveBeenCalledWith("daemon-1", "ws-2", true);
    await waitFor(() => {
      expect(listMyDaemonWorkspaces).toHaveBeenCalledTimes(2);
    });
  });
});
