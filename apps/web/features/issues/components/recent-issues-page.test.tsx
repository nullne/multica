import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Issue, MemberWithUser, Workspace } from "@/shared/types";

const mockCreateIssue = vi.fn();
const mockSetWorkspaceId = vi.fn();
const mockSetIssueLabels = vi.fn();
const mockPush = vi.fn();
let mockSearchParams = new URLSearchParams();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => mockSearchParams,
}));

vi.mock("@/shared/api", () => ({
  api: {
    createIssue: (...args: unknown[]) => mockCreateIssue(...args),
    setWorkspaceId: (...args: unknown[]) => mockSetWorkspaceId(...args),
    setIssueLabels: (...args: unknown[]) => mockSetIssueLabels(...args),
  },
}));

vi.mock("./issue-detail", () => ({
  IssueDetail: ({ issueId }: { issueId: string }) => (
    <div data-testid="issue-detail">Issue detail {issueId}</div>
  ),
}));

import { useIssueStore } from "@/features/issues/store";
import { useWorkspaceStore } from "@/features/workspace/store";
import { RecentIssuesPage } from "./recent-issues-page";

function makeWorkspace(id: string, name: string): Workspace {
  return {
    id,
    name,
    slug: name.toLowerCase().replaceAll(" ", "-"),
    description: null,
    context: null,
    settings: {},
    repos: [],
    issue_prefix: name.slice(0, 3).toUpperCase(),
    github_connected: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function makeIssue(
  id: string,
  workspaceId: string,
  identifier: string,
  title: string,
  updatedAt: string
): Issue {
  return {
    id,
    workspace_id: workspaceId,
    number: Number(identifier.replace(/\D/g, "")),
    identifier,
    title,
    description: null,
    status: "todo",
    priority: "medium",
    assignee_type: null,
    assignee_id: null,
    verifier_agent_id: null,
    max_verification_rounds: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: null,
    acceptance_criteria: [],
    criteria_status: null,
    position: 0,
    due_date: null,
    dispatch_provider: null,
    dispatch_daemon_id: null,
    dispatch_daemon_label: null,
    github_auto_fix_enabled: false,
    labels: [],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: updatedAt,
  };
}

function makeMember(
  workspaceId: string,
  userId: string,
  name: string
): MemberWithUser {
  return {
    id: `member-${userId}`,
    workspace_id: workspaceId,
    user_id: userId,
    role: "member",
    created_at: "2026-01-01T00:00:00Z",
    name,
    email: `${userId}@example.com`,
    avatar_url: null,
    kind: "human",
  };
}

describe("RecentIssuesPage", () => {
  const qwip = makeWorkspace("ws-qwip", "Qwip");
  const prod = makeWorkspace("ws-prod", "Prod Debug");
  const qwipMember = makeMember(qwip.id, "user-1", "Test User");
  const prodMember = makeMember(prod.id, "user-2", "Prod User");
  const issues = [
    makeIssue("issue-1", qwip.id, "CON-486", "Fix side effects", "2026-01-03T00:00:00Z"),
    makeIssue("issue-2", prod.id, "CON-522", "Fix PWA login", "2026-01-02T00:00:00Z"),
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParams = new URLSearchParams();
    useIssueStore.setState({ issues, loading: false, activeIssueId: null });
    useWorkspaceStore.setState({
      workspace: qwip,
      workspaces: [qwip, prod],
      members: [qwipMember],
      agents: [],
      skills: [],
    });
  });

  it("opens the selected issue detail from the URL", () => {
    mockSearchParams = new URLSearchParams("issue=issue-2");
    render(<RecentIssuesPage />);

    expect(screen.getByTestId("issue-detail")).toHaveTextContent("issue-2");
  });

  it("shows the chat-style create form by default and submits with cmd enter", async () => {
    mockCreateIssue.mockResolvedValueOnce({
      ...issues[0],
      id: "issue-new",
      identifier: "CON-999",
      title: "Investigate local login",
    });
    const user = userEvent.setup();
    render(<RecentIssuesPage />);

    expect(screen.queryByLabelText("Title")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Workspace" })).toHaveTextContent("Qwip");
    expect(screen.getByRole("button", { name: "Assign" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Assign" })).toHaveTextContent("Assignee");
    expect(screen.getByText("Medium")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Attach file" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create issue" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Create Issue" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Assign" }));
    expect(await screen.findByText("Unassigned")).toBeInTheDocument();
    await user.click(await screen.findByText("Test User"));
    expect(screen.getByText("Test User")).toBeInTheDocument();

    await user.type(
      screen.getByPlaceholderText("Describe the issue..."),
      "Investigate local login\nSteps are flaky"
    );
    await user.keyboard("{Meta>}{Enter}{/Meta}");

    await waitFor(() => {
      expect(mockSetWorkspaceId).toHaveBeenCalledWith(qwip.id);
      expect(mockCreateIssue).toHaveBeenCalledWith({
        title: "Investigate local login",
        description: "Investigate local login\nSteps are flaky",
        status: "backlog",
        priority: "medium",
        assignee_type: "member",
        assignee_id: "user-1",
        dispatch_provider: undefined,
        dispatch_daemon_id: undefined,
        dispatch_daemon_label: undefined,
        due_date: undefined,
      });
      expect(mockPush).toHaveBeenCalledWith("/home?issue=issue-new");
    });
  });

  it("switches the global workspace and clears the assignee picker when picking a different workspace", async () => {
    const switchWorkspaceSpy = vi.fn(async (workspaceId: string) => {
      const target = useWorkspaceStore
        .getState()
        .workspaces.find((w) => w.id === workspaceId);
      if (!target) return;
      useWorkspaceStore.setState({
        workspace: target,
        members: target.id === prod.id ? [prodMember] : [qwipMember],
        agents: [],
        skills: [],
      });
    });
    useWorkspaceStore.setState({ switchWorkspace: switchWorkspaceSpy });

    const user = userEvent.setup();
    render(<RecentIssuesPage />);

    // Pick the qwip member as assignee in the starting workspace.
    await user.click(screen.getByRole("button", { name: "Assign" }));
    await user.click(await screen.findByText("Test User"));
    expect(screen.getByRole("button", { name: "Assign" })).toHaveTextContent(
      "Test User"
    );

    // Switch workspace via the composer's workspace picker.
    await user.click(screen.getByRole("button", { name: "Workspace" }));
    await user.click(await screen.findByText("Prod Debug"));

    expect(switchWorkspaceSpy).toHaveBeenCalledWith(prod.id);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Workspace" })).toHaveTextContent(
        "Prod Debug"
      );
    });

    // The previous assignee from the old workspace must be cleared.
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Assign" })).toHaveTextContent(
        "Assignee"
      );
      expect(
        screen.queryByRole("button", { name: /Test User/ })
      ).not.toBeInTheDocument();
    });

    // The assignee picker should list only the new workspace's members.
    await user.click(screen.getByRole("button", { name: "Assign" }));
    expect(await screen.findByText("Prod User")).toBeInTheDocument();
    expect(screen.queryByText("Test User")).not.toBeInTheDocument();
  });
});
