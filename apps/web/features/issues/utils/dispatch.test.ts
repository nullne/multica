import { describe, it, expect } from "vitest";
import type { Agent, AgentRuntime } from "@/shared/types";
import { getAgentDispatchDefaults, hasCompleteAgentDispatchDefaults, resolveAssigneeChange, pickBestProvider } from "./dispatch";

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "a-1",
    workspace_id: "ws-1",
    providers: ["claude"],
    default_provider: null,
    name: "Agent",
    description: "",
    instructions: "",
    avatar_url: null,
    visibility: "workspace",
    status: "idle",
    owner_id: null,
    skills: [],
    tools: [],
    triggers: [],
    github_code_access: "read",
    default_daemon_id: null,
    max_concurrent_tasks: 1,
    model_config: {},
    created_at: "",
    updated_at: "",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

function makeRuntime(overrides: Partial<AgentRuntime> = {}): AgentRuntime {
  return {
    id: "r-1",
    workspace_id: "ws-1",
    daemon_id: "d-1",
    daemon_ref: "d-1",
    name: "rt",
    runtime_mode: "local",
    provider: "claude",
    status: "online",
    auth_status: "ready",
    device_info: "",
    metadata: {},
    last_seen_at: null,
    created_at: "",
    updated_at: "",
    ...overrides,
  };
}

describe("pickBestProvider", () => {
  it("returns undefined when the agent has no providers", () => {
    expect(pickBestProvider([], [], "d-1")).toBeUndefined();
  });

  it("falls back to the first provider when no daemon is given", () => {
    expect(pickBestProvider(["codex", "claude"], [], undefined)).toBe("codex");
  });

  it("honors an explicit preferred provider when it is valid", () => {
    expect(pickBestProvider(["codex", "claude"], [], "d-1", "claude")).toBe("claude");
  });

  it("prefers a provider with a ready runtime on the chosen daemon", () => {
    const runtimes = [
      makeRuntime({ daemon_ref: "d-1", provider: "claude", auth_status: "ready" }),
      makeRuntime({ daemon_ref: "d-1", provider: "codex", auth_status: "unauthenticated" }),
    ];
    expect(pickBestProvider(["codex", "claude"], runtimes, "d-1")).toBe("claude");
  });

  it("returns the first provider when no runtime is ready on that daemon", () => {
    const runtimes = [
      makeRuntime({ daemon_ref: "d-2", provider: "claude", auth_status: "ready" }),
    ];
    expect(pickBestProvider(["codex", "claude"], runtimes, "d-1")).toBe("codex");
  });
});

describe("resolveAssigneeChange", () => {
  it("clears every agent-only field when switching to a member", () => {
    const patch = resolveAssigneeChange({
      type: "member",
      id: "u-1",
      agents: [],
      runtimes: [],
      currentVerifierId: "a-9",
    });
    expect(patch).toEqual({
      assignee_type: "member",
      assignee_id: "u-1",
      dispatch_provider: null,
      dispatch_daemon_id: null,
      dispatch_daemon_label: null,
      verifier_agent_id: null,
      max_verification_rounds: null,
    });
  });

  it("clears every agent-only field when switching to unassigned", () => {
    const patch = resolveAssigneeChange({
      type: null,
      id: null,
      agents: [],
      runtimes: [],
    });
    expect(patch.assignee_type).toBeNull();
    expect(patch.dispatch_provider).toBeNull();
    expect(patch.dispatch_daemon_id).toBeNull();
    expect(patch.verifier_agent_id).toBeNull();
  });

  it("uses the agent's default daemon and a ready provider when assigning to an agent", () => {
    const agent = makeAgent({ id: "a-1", providers: ["codex", "claude"], default_daemon_id: "d-1" });
    const runtimes = [
      makeRuntime({ daemon_ref: "d-1", provider: "claude", auth_status: "ready" }),
    ];
    const patch = resolveAssigneeChange({
      type: "agent",
      id: "a-1",
      agents: [agent],
      runtimes,
    });
    expect(patch).toMatchObject({
      assignee_type: "agent",
      assignee_id: "a-1",
      dispatch_daemon_id: "d-1",
      dispatch_provider: "claude",
      dispatch_daemon_label: null,
    });
    // Verifier untouched when current verifier ≠ new assignee.
    expect(patch.verifier_agent_id).toBeUndefined();
  });

  it("falls back to saved defaults when the agent has no default daemon", () => {
    const agent = makeAgent({ id: "a-1", providers: ["claude"], default_daemon_id: null });
    const patch = resolveAssigneeChange({
      type: "agent",
      id: "a-1",
      agents: [agent],
      runtimes: [],
      savedDispatch: { daemonId: "d-7", provider: "claude" },
    });
    expect(patch.dispatch_daemon_id).toBe("d-7");
    expect(patch.dispatch_provider).toBe("claude");
  });

  it("prefers the agent's explicit default provider over saved dispatch state", () => {
    const agent = makeAgent({
      id: "a-1",
      providers: ["codex", "claude"],
      default_provider: "codex",
      default_daemon_id: "d-1",
    });
    const patch = resolveAssigneeChange({
      type: "agent",
      id: "a-1",
      agents: [agent],
      runtimes: [],
      savedDispatch: { daemonId: "d-9", provider: "claude" },
    });
    expect(patch.dispatch_daemon_id).toBe("d-1");
    expect(patch.dispatch_provider).toBe("codex");
  });

  it("clears the verifier when the new assignee is the same agent", () => {
    const agent = makeAgent({ id: "a-1", providers: ["claude"] });
    const patch = resolveAssigneeChange({
      type: "agent",
      id: "a-1",
      agents: [agent],
      runtimes: [],
      currentVerifierId: "a-1",
    });
    expect(patch.verifier_agent_id).toBeNull();
    expect(patch.max_verification_rounds).toBeNull();
  });
});

describe("agent dispatch defaults helpers", () => {
  it("detects when an agent has complete dispatch defaults", () => {
    expect(
      hasCompleteAgentDispatchDefaults(
        makeAgent({ default_provider: "claude", default_daemon_id: "d-1" }),
      ),
    ).toBe(true);
    expect(hasCompleteAgentDispatchDefaults(makeAgent({ default_provider: null }))).toBe(false);
  });

  it("returns the agent's explicit defaults ahead of saved dispatch state", () => {
    const defaults = getAgentDispatchDefaults(
      makeAgent({
        default_provider: "claude",
        default_daemon_id: "d-1",
        providers: ["claude", "codex"],
      }),
      [],
      { daemonId: "d-9", provider: "codex" },
    );
    expect(defaults).toEqual({ daemonId: "d-1", provider: "claude" });
  });
});
