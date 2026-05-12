import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, beforeEach, afterEach, expect, it } from "vitest";
import { DaemonPicker } from "./daemon-picker";
import { useRuntimeStore } from "@/features/runtimes";

describe("DaemonPicker", () => {
  const initialState = useRuntimeStore.getState();

  beforeEach(() => {
    useRuntimeStore.setState({
      ...initialState,
      daemons: [
        {
          id: "workspace-daemon",
          user_id: "u-1",
          workspace_visible: true,
          daemon_id: "workspace-daemon",
          status: "online",
          cli_version: "1.0.0",
          device_name: "Workspace Machine",
          device_info: "",
          labels: [],
          metadata: {},
          last_seen_at: null,
          created_at: "",
          updated_at: "",
          archived_at: null,
        },
        {
          id: "owned-hidden-daemon",
          user_id: "u-1",
          workspace_visible: false,
          daemon_id: "owned-hidden-daemon",
          status: "online",
          cli_version: "1.0.0",
          device_name: "My Hidden Machine",
          device_info: "",
          labels: [],
          metadata: {},
          last_seen_at: null,
          created_at: "",
          updated_at: "",
          archived_at: null,
        },
      ],
      runtimes: [
        {
          id: "rt-1",
          workspace_id: "ws-1",
          daemon_id: "workspace-daemon",
          daemon_ref: "workspace-daemon",
          name: "claude",
          runtime_mode: "local",
          provider: "claude",
          status: "online",
          auth_status: "ready",
          device_info: "",
          metadata: {},
          last_seen_at: null,
          created_at: "",
          updated_at: "",
        },
      ],
    });
  });

  afterEach(() => {
    useRuntimeStore.setState(initialState);
  });

  it("separates owned hidden daemons into their own section", async () => {
    const user = userEvent.setup();

    render(
      <DaemonPicker
        daemonId={null}
        provider="claude"
        onSelect={() => {}}
      />,
    );

    await user.click(screen.getByText("Environment"));

    expect(screen.getByText("Compatible")).toBeInTheDocument();
    expect(screen.getByText("Workspace Machine")).toBeInTheDocument();
    expect(screen.getByText("Your Daemons")).toBeInTheDocument();
    expect(screen.getByText("My Hidden Machine")).toBeInTheDocument();
  });
});
