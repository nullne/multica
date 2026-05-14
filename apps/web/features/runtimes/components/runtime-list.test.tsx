import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, beforeEach, afterEach, expect, it } from "vitest";
import { RuntimeList } from "./runtime-list";
import { useRuntimeStore } from "../store";
import type { Daemon, AgentRuntime } from "@/shared/types";

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

describe("RuntimeList", () => {
  const initialState = useRuntimeStore.getState();

  beforeEach(() => {
    useRuntimeStore.setState({
      ...initialState,
      enabledProviders: [],
    });
  });

  afterEach(() => {
    useRuntimeStore.setState(initialState);
  });

  it("renders workspace-visible daemons in the main list", () => {
    const daemons: Daemon[] = [
      makeDaemon({ id: "ws-1", device_name: "Workspace Machine" }),
    ];

    render(
      <RuntimeList
        daemons={daemons}
        runtimes={[]}
        selectedId=""
        onSelect={() => {}}
      />,
    );

    expect(screen.getByText("Workspace Machine")).toBeInTheDocument();
    // Only workspace daemons count toward the online indicator.
    expect(screen.getByText("1/1 online")).toBeInTheDocument();
    expect(screen.queryByText(/Personal/)).not.toBeInTheDocument();
  });

  it("hides personal daemons behind a collapsible section by default", () => {
    const daemons: Daemon[] = [
      makeDaemon({ id: "ws-1", device_name: "Workspace Machine" }),
      makeDaemon({
        id: "personal-1",
        device_name: "My Laptop",
        workspace_visible: false,
      }),
    ];

    render(
      <RuntimeList
        daemons={daemons}
        runtimes={[]}
        selectedId=""
        onSelect={() => {}}
      />,
    );

    // The workspace daemon is visible upfront.
    expect(screen.getByText("Workspace Machine")).toBeInTheDocument();
    // The personal daemon is not rendered until the section is expanded.
    expect(screen.queryByText("My Laptop")).not.toBeInTheDocument();
    // The collapsible trigger advertises the personal count.
    expect(screen.getByText("Personal (1)")).toBeInTheDocument();
    // Online count excludes personal daemons.
    expect(screen.getByText("1/1 online")).toBeInTheDocument();
  });

  it("reveals personal daemons with a Personal badge after expanding the section", async () => {
    const user = userEvent.setup();
    const daemons: Daemon[] = [
      makeDaemon({ id: "ws-1", device_name: "Workspace Machine" }),
      makeDaemon({
        id: "personal-1",
        device_name: "My Laptop",
        workspace_visible: false,
      }),
    ];

    render(
      <RuntimeList
        daemons={daemons}
        runtimes={[]}
        selectedId=""
        onSelect={() => {}}
      />,
    );

    await user.click(screen.getByText("Personal (1)"));

    expect(screen.getByText("My Laptop")).toBeInTheDocument();
    // The "Personal" badge labels the daemon so it can't be confused with a
    // workspace-enabled one.
    expect(screen.getAllByText("Personal").length).toBeGreaterThan(0);
  });

  it("shows a friendly empty state when only personal daemons exist", () => {
    const daemons: Daemon[] = [
      makeDaemon({
        id: "personal-1",
        device_name: "My Laptop",
        workspace_visible: false,
      }),
    ];

    render(
      <RuntimeList
        daemons={daemons}
        runtimes={[]}
        selectedId=""
        onSelect={() => {}}
      />,
    );

    expect(
      screen.getByText("No daemons enabled in this workspace"),
    ).toBeInTheDocument();
    expect(screen.getByText("Personal (1)")).toBeInTheDocument();
    expect(screen.getByText("0/0 online")).toBeInTheDocument();
  });

  it("skips workspace-scoped missing-provider chips for personal daemons", async () => {
    const user = userEvent.setup();
    useRuntimeStore.setState({
      ...initialState,
      enabledProviders: ["claude", "codex"],
    });

    const personal = makeDaemon({
      id: "personal-1",
      device_name: "My Laptop",
      workspace_visible: false,
    });
    const runtime: AgentRuntime = {
      id: "rt-1",
      workspace_id: "ws-1",
      daemon_id: personal.daemon_id,
      daemon_ref: personal.id,
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
    };

    render(
      <RuntimeList
        daemons={[personal]}
        runtimes={[runtime]}
        selectedId=""
        onSelect={() => {}}
      />,
    );

    await user.click(screen.getByText("Personal (1)"));

    // Installed provider is shown.
    expect(screen.getByText("claude")).toBeInTheDocument();
    // The workspace-enabled "codex" provider is NOT advertised as missing on
    // a personal daemon — that label only makes sense for workspace daemons.
    expect(screen.queryByText("codex")).not.toBeInTheDocument();
  });
});
