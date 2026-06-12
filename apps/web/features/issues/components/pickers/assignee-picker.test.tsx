import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Agent, MemberWithUser, User } from "@/shared/types";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { AssigneePicker } from "./assignee-picker";

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: "user-1",
    name: "Test User",
    email: "user@example.com",
    avatar_url: null,
    created_at: "",
    updated_at: "",
    ...overrides,
  };
}

function makeMember(overrides: Partial<MemberWithUser> = {}): MemberWithUser {
  return {
    id: `member-${overrides.user_id ?? "user-1"}`,
    workspace_id: "ws-1",
    user_id: "user-1",
    role: "member",
    created_at: "",
    name: "Test User",
    email: "user@example.com",
    avatar_url: null,
    kind: "human",
    ...overrides,
  };
}

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "agent-1",
    workspace_id: "ws-1",
    providers: ["claude"],
    default_provider: null,
    name: "Agent One",
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
    model_config: {},
    created_at: "",
    updated_at: "",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

describe("AssigneePicker", () => {
  const initialAuthState = useAuthStore.getState();
  const initialWorkspaceState = useWorkspaceStore.getState();

  beforeEach(() => {
    useAuthStore.setState({ user: makeUser(), isLoading: false });
    useWorkspaceStore.setState({
      members: [makeMember({ user_id: "user-1", name: "Current User" })],
      agents: [],
    });
  });

  afterEach(() => {
    useAuthStore.setState(initialAuthState);
    useWorkspaceStore.setState(initialWorkspaceState);
  });

  it("renders agents before members and hides unassignable private agents", async () => {
    const user = userEvent.setup();

    useWorkspaceStore.setState({
      members: [
        makeMember({ user_id: "user-1", name: "Current User" }),
        makeMember({ user_id: "member-2", name: "Alice" }),
      ],
      agents: [
        makeAgent({ id: "agent-public", name: "Public Agent" }),
        makeAgent({
          id: "agent-private-hidden",
          name: "Private Hidden Agent",
          visibility: "private",
          owner_id: "other-user",
        }),
      ],
    });

    render(
      <AssigneePicker
        assigneeType={null}
        assigneeId={null}
        onUpdate={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Unassigned" }));

    const agentsHeader = screen.getByRole("button", { name: "Agents" });
    const membersHeader = screen.getByRole("button", { name: "Members" });
    expect(
      agentsHeader.compareDocumentPosition(membersHeader) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).not.toBe(0);

    expect(screen.getByText("Public Agent")).toBeInTheDocument();
    expect(screen.queryByText("Private Hidden Agent")).not.toBeInTheDocument();
  });

  it("toggles members and agents groups from their headers", async () => {
    const user = userEvent.setup();

    useWorkspaceStore.setState({
      members: [
        makeMember({ user_id: "user-1", name: "Current User" }),
        makeMember({ user_id: "member-2", name: "Alice" }),
      ],
      agents: [makeAgent({ id: "agent-2", name: "Build Bot" })],
    });

    render(
      <AssigneePicker
        assigneeType={null}
        assigneeId={null}
        onUpdate={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Unassigned" }));

    const agentsHeader = screen.getByRole("button", { name: "Agents" });
    const membersHeader = screen.getByRole("button", { name: "Members" });

    expect(screen.getByText("Build Bot")).toBeInTheDocument();
    expect(screen.getByText("Alice")).toBeInTheDocument();

    await user.click(agentsHeader);
    expect(screen.queryByText("Build Bot")).not.toBeInTheDocument();

    await user.click(agentsHeader);
    expect(screen.getByText("Build Bot")).toBeInTheDocument();

    await user.click(membersHeader);
    expect(screen.queryByText("Alice")).not.toBeInTheDocument();
  });

  it("search results only include visible assignable options", async () => {
    const user = userEvent.setup();

    useWorkspaceStore.setState({
      members: [
        makeMember({ user_id: "user-1", name: "Current User" }),
        makeMember({ user_id: "member-2", name: "Alice" }),
      ],
      agents: [
        makeAgent({
          id: "agent-private-hidden",
          name: "Secret Agent",
          visibility: "private",
          owner_id: "other-user",
        }),
      ],
    });

    render(
      <AssigneePicker
        assigneeType={null}
        assigneeId={null}
        onUpdate={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Unassigned" }));
    await user.type(screen.getByRole("textbox", { name: "Filter options" }), "secret");

    expect(screen.queryByText("Secret Agent")).not.toBeInTheDocument();
    expect(screen.getByText("No results")).toBeInTheDocument();
  });
});
