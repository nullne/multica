import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, beforeEach, afterEach, expect, it, vi } from "vitest";
import { AgentDispatchConfirm } from "./assignee-picker";
import { useRuntimeStore } from "@/features/runtimes";
import type { Agent } from "@/shared/types";

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "agent-1",
    workspace_id: "ws-1",
    providers: ["claude"],
    default_provider: null,
    default_model: null,
    name: "Claude Agent",
    description: "",
    instructions: "",
    avatar_url: null,
    visibility: "workspace",
    status: "idle",
    owner_id: null,
    skills: [],
    tools: [],
    triggers: [],
    github_code_access: "write",
    default_daemon_id: null,
    max_concurrent_tasks: 1,
    created_at: "",
    updated_at: "",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

describe("AgentDispatchConfirm", () => {
  const initialState = useRuntimeStore.getState();

  beforeEach(() => {
    useRuntimeStore.setState({
      ...initialState,
      fetchAll: vi.fn().mockResolvedValue(undefined),
      daemons: [
        {
          id: "daemon-default",
          user_id: "u-1",
          workspace_visible: true,
          daemon_id: "daemon-default",
          status: "online",
          cli_version: "1.0.0",
          device_name: "Default Machine",
          device_info: "",
          labels: [],
          metadata: {},
          last_seen_at: null,
          created_at: "",
          updated_at: "",
          archived_at: null,
        },
        {
          id: "daemon-other",
          user_id: "u-1",
          workspace_visible: true,
          daemon_id: "daemon-other",
          status: "online",
          cli_version: "1.0.0",
          device_name: "Other Machine",
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
          daemon_id: "daemon-default",
          daemon_ref: "daemon-default",
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
        {
          id: "rt-2",
          workspace_id: "ws-1",
          daemon_id: "daemon-other",
          daemon_ref: "daemon-other",
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

  it("preselects the agent's default environment but keeps it editable", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    const agent = makeAgent({ default_daemon_id: "daemon-default" });

    render(
      <AgentDispatchConfirm
        agent={agent}
        initialDaemonId="daemon-default"
        initialProvider="claude"
        onBack={() => {}}
        onConfirm={onConfirm}
      />,
    );

    // Default environment is prefilled.
    expect(screen.getByText("Default Machine")).toBeInTheDocument();

    // Environment trigger is interactive — opens the picker and exposes alternatives.
    await user.click(screen.getByText("Default Machine"));
    expect(screen.getByText("Other Machine")).toBeInTheDocument();

    // Switching environments updates the confirm payload.
    await user.click(screen.getByText("Other Machine"));
    await user.click(screen.getByRole("button", { name: /assign agent/i }));
    expect(onConfirm).toHaveBeenCalledWith("daemon-other", "claude");
  });

  it("does not render the locked environment indicator for agents with defaults", () => {
    const agent = makeAgent({ default_daemon_id: "daemon-default" });

    const { container } = render(
      <AgentDispatchConfirm
        agent={agent}
        initialDaemonId="daemon-default"
        initialProvider="claude"
        onBack={() => {}}
        onConfirm={() => {}}
      />,
    );

    // The Environment field renders as a button (the property picker trigger),
    // never as a static row with a Lock icon.
    const envTrigger = screen.getByText("Default Machine").closest("button");
    expect(envTrigger).not.toBeNull();
    expect(container.querySelector(".lucide-lock")).toBeNull();
  });
});
