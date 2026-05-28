import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const pushMock = vi.fn();
const listRecentIssuesMock = vi.fn();
const switchWorkspaceMock = vi.fn(async () => {});
let mockPathname = "/issues";
let mockSearchParams = new URLSearchParams();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => mockPathname,
  useSearchParams: () => mockSearchParams,
}));

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...props
  }: {
    children: React.ReactNode;
    href: string;
    [key: string]: unknown;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("@/shared/api", () => ({
  api: {
    listRecentIssues: (...args: unknown[]) => listRecentIssuesMock(...args),
  },
}));

const authState = {
  user: {
    id: "user-1",
    name: "Test User",
    email: "test@example.com",
    avatar_url: null,
  },
  logout: vi.fn(),
};

vi.mock("@/features/auth", () => ({
  useAuthStore: Object.assign(
    (selector?: (s: typeof authState) => unknown) =>
      selector ? selector(authState) : authState,
    { getState: () => authState },
  ),
}));

const workspaceState = {
  workspace: { id: "ws-1", name: "Test Workspace", slug: "test" },
  workspaces: [
    { id: "ws-1", name: "Test Workspace", slug: "test" },
    { id: "ws-2", name: "Prod Debug", slug: "prod-debug" },
  ],
  switchWorkspace: switchWorkspaceMock,
  clearWorkspace: vi.fn(),
};

vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: Object.assign(
    (selector?: (s: typeof workspaceState) => unknown) =>
      selector ? selector(workspaceState) : workspaceState,
    { getState: () => workspaceState },
  ),
  WorkspaceAvatar: ({ name }: { name: string }) => <span>{name.charAt(0)}</span>,
}));

const inboxState = { unreadCount: () => 0 };
vi.mock("@/features/inbox", () => ({
  useInboxStore: Object.assign(
    (selector?: (s: typeof inboxState) => unknown) =>
      selector ? selector(inboxState) : inboxState,
    { getState: () => inboxState },
  ),
}));

const modalsState = { open: vi.fn() };
vi.mock("@/features/modals", () => ({
  useModalStore: Object.assign(
    (selector?: (s: typeof modalsState) => unknown) =>
      selector ? selector(modalsState) : modalsState,
    { getState: () => modalsState },
  ),
}));

const draftState = { draft: { title: "", description: "" } };
vi.mock("@/features/issues/stores/draft-store", () => ({
  useIssueDraftStore: Object.assign(
    (selector?: (s: typeof draftState) => unknown) =>
      selector ? selector(draftState) : draftState,
    { getState: () => draftState },
  ),
}));

import { SidebarProvider } from "@/components/ui/sidebar";
import { useRecentsPrefsStore } from "@/features/navigation/recents-prefs-store";
import { useRecentsStore } from "@/features/navigation/recents-store";
import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";
import type { AgentTask } from "@/shared/types/agent";
import { AppSidebar } from "./app-sidebar";

function renderSidebar() {
  return render(
    <SidebarProvider>
      <AppSidebar />
    </SidebarProvider>,
  );
}

import type { Issue } from "@/shared/types";

function makeIssue(overrides: Partial<Issue> & {
  id: string;
  workspace_id: string;
  identifier: string;
  title: string;
  updated_at: string;
}): Issue {
  return {
    number: 0,
    description: null,
    status: "todo",
    priority: "medium",
    creator_type: "member",
    creator_id: "user-1",
    assignee_type: null,
    assignee_id: null,
    verifier_agent_id: null,
    max_verification_rounds: null,
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
    created_at: overrides.updated_at,
    ...overrides,
  };
}

function defaultMockImpl({ workspace_id }: { workspace_id: string }) {
  return Promise.resolve({
    issues:
      workspace_id === "ws-2"
        ? [
            makeIssue({
              id: "issue-2",
              workspace_id: "ws-2",
              identifier: "PRD-2",
              title: "Other workspace issue",
              updated_at: "2026-01-02T00:00:00Z",
            }),
          ]
        : [
            makeIssue({
              id: "issue-1",
              workspace_id: "ws-1",
              identifier: "TES-1",
              title: "Recent issue",
              updated_at: "2026-01-01T00:00:00Z",
            }),
          ],
    total: 1,
  });
}

function makeAgentTask(issueId: string): AgentTask {
  return {
    id: `task-${issueId}`,
    agent_id: "agent-1",
    runtime_id: "rt-1",
    issue_id: issueId,
    status: "running",
    priority: 0,
    dispatched_at: null,
    started_at: null,
    completed_at: null,
    result: null,
    error: null,
    created_at: "2026-05-01T00:00:00Z",
    trigger_comment_id: null,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockPathname = "/issues";
  mockSearchParams = new URLSearchParams();
  switchWorkspaceMock.mockImplementation(async () => {});
  useRecentsStore.getState().clear();
  useRecentsPrefsStore.setState({
    collapsedWorkspaceIds: [],
    pinnedIssueIds: [],
  });
  useActiveTaskStore.setState({ tasks: new Map() });
  listRecentIssuesMock.mockImplementation(defaultMockImpl);
});

describe("AppSidebar user menu", () => {
  it("shows Routines as a top-level workspace navigation item", () => {
    renderSidebar();

    const routinesLink = screen.getByRole("link", { name: /Routines/i });
    expect(routinesLink).toHaveAttribute("href", "/routines");
    const issuesLink = screen.getByRole("link", { name: "Issues" });
    const allLinks = screen.getAllByRole("link");
    expect(allLinks.indexOf(routinesLink)).toBe(allLinks.indexOf(issuesLink) + 1);
  });

  it("keeps Routines active on routine detail routes", () => {
    mockPathname = "/routines/routine-1";
    renderSidebar();

    const routinesLink = screen.getByRole("link", { name: /Routines/i });
    expect(routinesLink).toHaveAttribute("data-active");

    const issuesLink = screen.getByRole("link", { name: "Issues" });
    expect(issuesLink).not.toHaveAttribute("data-active");
  });

  it("opens the bottom user menu without throwing", async () => {
    const user = userEvent.setup();
    renderSidebar();

    const trigger = screen.getByRole("button", { name: /Test User/i });
    await user.click(trigger);

    expect(await screen.findByText("Account settings")).toBeInTheDocument();
    expect(screen.getByText("Log out")).toBeInTheDocument();
    expect(screen.getAllByText("test@example.com").length).toBeGreaterThanOrEqual(1);
  });

  it("navigates to /account when Account settings is clicked", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByRole("button", { name: /Test User/i }));
    await user.click(await screen.findByText("Account settings"));

    expect(pushMock).toHaveBeenCalledWith("/account");
  });

  it("navigates to personal daemon management from the user menu", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByRole("button", { name: /Test User/i }));
    await user.click(await screen.findByText("My daemons"));

    expect(pushMock).toHaveBeenCalledWith("/account?tab=daemons");
  });

  it("shows only new issue and recents navigation on the home route", async () => {
    mockPathname = "/home";
    renderSidebar();

    expect(screen.getByRole("button", { name: "New" })).toBeInTheDocument();
    expect(screen.getByText("Recents")).toBeInTheDocument();
    expect(await screen.findByText("Recent issue")).toBeInTheDocument();
    expect(screen.getByText("Prod Debug")).toBeInTheDocument();
    expect(screen.getByText("Other workspace issue")).toBeInTheDocument();
    expect(screen.queryByText("Inbox")).not.toBeInTheDocument();
    expect(screen.queryByText("Agents")).not.toBeInTheDocument();
  });
});

describe("AppSidebar recents fetching", () => {
  beforeEach(() => {
    mockPathname = "/home";
  });

  it("calls the recents endpoint exactly once per workspace on mount", async () => {
    renderSidebar();
    await screen.findByText("Other workspace issue");

    expect(listRecentIssuesMock).toHaveBeenCalledTimes(2);
    expect(listRecentIssuesMock).toHaveBeenCalledWith(
      expect.objectContaining({ workspace_id: "ws-1" }),
    );
    expect(listRecentIssuesMock).toHaveBeenCalledWith(
      expect.objectContaining({ workspace_id: "ws-2" }),
    );
  });

  it("does not refetch when an issue is updated incrementally", async () => {
    renderSidebar();
    await screen.findByText("Other workspace issue");
    const initialCalls = listRecentIssuesMock.mock.calls.length;

    useRecentsStore.getState().upsertIssue(
      makeIssue({
        id: "issue-1",
        workspace_id: "ws-1",
        identifier: "TES-1",
        title: "Recent issue (renamed)",
        updated_at: "2026-01-05T00:00:00Z",
      }),
    );

    await screen.findByText("Recent issue (renamed)");
    expect(listRecentIssuesMock).toHaveBeenCalledTimes(initialCalls);
  });

  it("pushes the new URL before awaiting any workspace switch", async () => {
    let resolveSwitch: () => void = () => {};
    switchWorkspaceMock.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveSwitch = resolve;
        }),
    );

    const user = userEvent.setup();
    renderSidebar();
    const item = await screen.findByText("Other workspace issue");

    await user.click(item);

    // URL push must happen synchronously — before the workspace switch
    // resolves — so the detail view can render the new issue immediately.
    expect(pushMock).toHaveBeenCalledWith("/home?issue=issue-2");
    expect(switchWorkspaceMock).toHaveBeenCalledWith("ws-2");
    resolveSwitch();
  });
});

describe("AppSidebar recents filtering", () => {
  beforeEach(() => {
    mockPathname = "/home";
  });

  it("requests recents with the server-side mine filter by default", async () => {
    renderSidebar();
    await screen.findByText("Other workspace issue");

    for (const call of listRecentIssuesMock.mock.calls) {
      expect(call[0]).toMatchObject({ mine: true });
    }
  });

  it("re-requests recents without the mine filter when the toggle is turned off", async () => {
    const userInteract = userEvent.setup();
    renderSidebar();
    await screen.findByText("Other workspace issue");

    const callsBeforeToggle = listRecentIssuesMock.mock.calls.length;

    await userInteract.click(screen.getByLabelText("Filter recents"));
    await userInteract.click(await screen.findByText("Only my issues"));

    await waitFor(() => {
      expect(listRecentIssuesMock.mock.calls.length).toBeGreaterThan(
        callsBeforeToggle,
      );
    });
    const newCalls = listRecentIssuesMock.mock.calls.slice(callsBeforeToggle);
    for (const call of newCalls) {
      expect(call[0]).not.toMatchObject({ mine: true });
    }
  });

  it("shows the user's issues even when they would fall outside the global top 50", async () => {
    listRecentIssuesMock.mockImplementation(
      ({ workspace_id, mine }: { workspace_id: string; mine?: boolean }) => {
        if (workspace_id !== "ws-1") return Promise.resolve({ issues: [], total: 0 });
        // The 50 globally most recent issues in this workspace are all
        // unrelated to the user — so a client-side filter over the
        // unfiltered list would render Recents empty.
        if (!mine) {
          return Promise.resolve({
            issues: Array.from({ length: 50 }, (_, idx) =>
              makeIssue({
                id: `issue-other-${idx}`,
                workspace_id: "ws-1",
                identifier: `TES-${100 + idx}`,
                title: `Someone else's issue ${idx}`,
                creator_id: "user-other",
                assignee_id: null,
                updated_at: `2026-02-${String(idx + 1).padStart(2, "0")}T00:00:00Z`,
              }),
            ),
            total: 50,
          });
        }
        // Server-side mine filter pulls the user's older issue back in.
        return Promise.resolve({
          issues: [
            makeIssue({
              id: "issue-buried-mine",
              workspace_id: "ws-1",
              identifier: "TES-9",
              title: "My buried issue",
              creator_id: "user-1",
              assignee_id: null,
              updated_at: "2026-01-15T00:00:00Z",
            }),
          ],
          total: 1,
        });
      },
    );

    renderSidebar();

    expect(await screen.findByText("My buried issue")).toBeInTheDocument();
    expect(
      screen.queryByText("Someone else's issue 0"),
    ).not.toBeInTheDocument();
  });
});

describe("AppSidebar recents running indicator", () => {
  beforeEach(() => {
    mockPathname = "/home";
  });

  it("renders the shared spinning ring when the recent issue has an active agent task", async () => {
    listRecentIssuesMock.mockImplementation(({ workspace_id }: { workspace_id: string }) =>
      Promise.resolve({
        issues:
          workspace_id === "ws-1"
            ? [
                makeIssue({
                  id: "issue-1",
                  workspace_id: "ws-1",
                  identifier: "TES-1",
                  title: "Running issue",
                  status: "in_progress",
                  assignee_type: "agent",
                  assignee_id: "agent-1",
                  updated_at: "2026-01-01T00:00:00Z",
                }),
              ]
            : [],
        total: 1,
      }),
    );
    useActiveTaskStore.setState({
      tasks: new Map([["issue-1", makeAgentTask("issue-1")]]),
    });

    renderSidebar();
    await screen.findByText("Running issue");

    expect(screen.getByTitle("Agent is working")).toBeInTheDocument();
    expect(document.querySelector(".animate-ping")).toBeNull();
  });

  it("does not show the ring for an in_progress agent issue when no active task is tracked", async () => {
    listRecentIssuesMock.mockImplementation(({ workspace_id }: { workspace_id: string }) =>
      Promise.resolve({
        issues:
          workspace_id === "ws-1"
            ? [
                makeIssue({
                  id: "issue-stale",
                  workspace_id: "ws-1",
                  identifier: "TES-2",
                  title: "Stale in_progress issue",
                  status: "in_progress",
                  assignee_type: "agent",
                  assignee_id: "agent-1",
                  updated_at: "2026-01-01T00:00:00Z",
                }),
              ]
            : [],
        total: 1,
      }),
    );

    renderSidebar();
    await screen.findByText("Stale in_progress issue");

    expect(screen.queryByTitle("Agent is working")).not.toBeInTheDocument();
    expect(document.querySelector(".animate-ping")).toBeNull();
  });
});

describe("AppSidebar grouped recents", () => {
  beforeEach(() => {
    mockPathname = "/home";
  });

  it("collapses a workspace group when its header is clicked and expands again on re-click", async () => {
    const user = userEvent.setup();
    renderSidebar();

    expect(await screen.findByText("Other workspace issue")).toBeInTheDocument();

    const header = screen.getByRole("button", { name: "Prod Debug" });
    expect(header).toHaveAttribute("aria-expanded", "true");

    await user.click(header);
    expect(header).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Other workspace issue")).not.toBeInTheDocument();

    await user.click(header);
    expect(header).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Other workspace issue")).toBeInTheDocument();
  });

  it("limits a large group to a bounded scrollable region", async () => {
    listRecentIssuesMock.mockImplementation(
      ({ workspace_id }: { workspace_id: string }) =>
        Promise.resolve({
          issues:
            workspace_id === "ws-2"
              ? Array.from({ length: 15 }, (_, idx) =>
                  makeIssue({
                    id: `issue-bulk-${idx}`,
                    workspace_id: "ws-2",
                    identifier: `PRD-${idx + 10}`,
                    title: `Bulk issue ${idx}`,
                    updated_at: `2026-01-${String(idx + 1).padStart(2, "0")}T00:00:00Z`,
                  }),
                )
              : [],
          total: 15,
        }),
    );

    renderSidebar();
    const body = await screen.findByTestId("recents-group-body-ws-2");
    expect(body.getAttribute("data-scrollable")).toBe("true");
    expect(body.style.maxHeight).not.toBe("");
    expect(within(body).getAllByText(/^Bulk issue/)).toHaveLength(15);
  });

  it("does not mark a small group as scrollable", async () => {
    renderSidebar();
    const body = await screen.findByTestId("recents-group-body-ws-2");
    await waitFor(() => {
      expect(body.getAttribute("data-scrollable")).toBe("false");
    });
    expect(body.style.maxHeight).toBe("");
  });

  it("does not render workspace-level pin or move controls", async () => {
    renderSidebar();
    expect(await screen.findByText("Other workspace issue")).toBeInTheDocument();

    expect(
      screen.queryByRole("button", { name: /Pin Test Workspace/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Pin Prod Debug/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Move .* (up|down)/i }),
    ).not.toBeInTheDocument();
  });
});

describe("AppSidebar recents active highlight", () => {
  beforeEach(() => {
    mockPathname = "/home";
  });

  function activeRow() {
    return document.querySelector('[aria-current="page"]');
  }

  it("highlights the recents row matching the ?issue=<id> URL param", async () => {
    mockSearchParams = new URLSearchParams("issue=issue-2");
    renderSidebar();

    const row = await screen.findByRole("button", {
      name: "Other workspace issue",
    });
    expect(row).toHaveAttribute("aria-current", "page");
    // base-ui renders the active state as a value-less `data-active` attribute.
    expect(row).toHaveAttribute("data-active");

    const otherRow = await screen.findByRole("button", { name: "Recent issue" });
    expect(otherRow).not.toHaveAttribute("aria-current");
    expect(otherRow).not.toHaveAttribute("data-active");
  });

  it("leaves all rows unhighlighted when no issue is in the URL", async () => {
    mockSearchParams = new URLSearchParams();
    renderSidebar();

    await screen.findByText("Recent issue");
    expect(activeRow()).toBeNull();
  });

  it("moves the highlight to a row when its URL becomes active", async () => {
    mockSearchParams = new URLSearchParams("issue=issue-1");
    const { rerender } = renderSidebar();

    await screen.findByText("Other workspace issue");
    await waitFor(() => {
      expect(activeRow()?.textContent).toContain("Recent issue");
    });

    mockSearchParams = new URLSearchParams("issue=issue-2");
    rerender(
      <SidebarProvider>
        <AppSidebar />
      </SidebarProvider>,
    );

    await waitFor(() => {
      expect(activeRow()?.textContent).toContain("Other workspace issue");
    });
    const otherRow = screen.getByRole("button", { name: "Recent issue" });
    expect(otherRow).not.toHaveAttribute("aria-current");
  });

  it("highlights a pinned row when the URL targets it", async () => {
    mockSearchParams = new URLSearchParams("issue=issue-2");
    useRecentsPrefsStore.setState({ pinnedIssueIds: ["issue-2"] });
    renderSidebar();

    const pinnedSection = await screen.findByTestId("recents-pinned-section");
    // The unpin action button shares "Other workspace issue" in its aria-label,
    // so match the row's menu button by its exact visible text instead.
    const pinnedRow = within(pinnedSection).getByRole("button", {
      name: "Other workspace issue",
    });
    expect(pinnedRow).toHaveAttribute("aria-current", "page");
    expect(pinnedRow).toHaveAttribute("data-active");
  });
});

describe("AppSidebar pinned issues", () => {
  beforeEach(() => {
    mockPathname = "/home";
  });

  it("does not render the Pinned section when nothing is pinned", async () => {
    renderSidebar();
    await screen.findByText("Other workspace issue");

    expect(screen.queryByTestId("recents-pinned-section")).not.toBeInTheDocument();
    expect(screen.queryByText("Pinned")).not.toBeInTheDocument();
  });

  it("pins an issue from its row and renders it in the Pinned section above the workspace groups", async () => {
    const user = userEvent.setup();
    renderSidebar();

    expect(await screen.findByText("Other workspace issue")).toBeInTheDocument();
    expect(screen.queryByTestId("recents-pinned-section")).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: /Pin Other workspace issue/i }),
    );

    expect(useRecentsPrefsStore.getState().pinnedIssueIds).toEqual(["issue-2"]);

    const pinnedSection = await screen.findByTestId("recents-pinned-section");
    expect(within(pinnedSection).getByText("Other workspace issue")).toBeInTheDocument();

    // The pinned row leaves the workspace group it came from.
    const prodBody = screen.queryByTestId("recents-group-body-ws-2");
    expect(prodBody).toBeNull();

    // Pinned section renders above the remaining workspace groups.
    const sectionContainers = screen
      .getAllByTestId(/^recents-(pinned-section|group-ws-)/)
      .map((el) => el.dataset.testid);
    expect(sectionContainers[0]).toBe("recents-pinned-section");
  });

  it("unpins an issue from the Pinned section", async () => {
    const user = userEvent.setup();
    useRecentsPrefsStore.setState({ pinnedIssueIds: ["issue-2"] });
    renderSidebar();

    await screen.findByTestId("recents-pinned-section");
    expect(useRecentsPrefsStore.getState().pinnedIssueIds).toEqual(["issue-2"]);

    await user.click(
      screen.getByRole("button", { name: /Unpin Other workspace issue/i }),
    );

    expect(useRecentsPrefsStore.getState().pinnedIssueIds).toEqual([]);
    expect(screen.queryByTestId("recents-pinned-section")).not.toBeInTheDocument();
  });

  it("hides the Pinned section when the pinned issue is no longer in Recents", async () => {
    useRecentsPrefsStore.setState({ pinnedIssueIds: ["issue-missing"] });
    renderSidebar();

    await screen.findByText("Other workspace issue");
    expect(screen.queryByTestId("recents-pinned-section")).not.toBeInTheDocument();
  });

  it("drops focus from the pin button after a mouse click so the hover-revealed cluster collapses on pointer leave", async () => {
    const user = userEvent.setup();
    renderSidebar();

    const pinButton = await screen.findByRole("button", {
      name: /Pin Other workspace issue/i,
    });
    const blurSpy = vi.spyOn(pinButton, "blur");

    await user.click(pinButton);

    // A pointer-driven click leaves the button focused, which keeps the
    // parent row's :focus-within matching even after the mouse leaves —
    // so the hover-revealed pin action stays visible until the user
    // clicks empty space. Blurring on pointer activations is what makes
    // the cluster collapse when the pointer leaves.
    expect(blurSpy).toHaveBeenCalled();
  });

  it("keeps focus on the pin button after a keyboard activation so tab navigation stays intact", async () => {
    const user = userEvent.setup();
    renderSidebar();

    const pinButton = await screen.findByRole("button", {
      name: /Pin Other workspace issue/i,
    });
    pinButton.focus();
    const blurSpy = vi.spyOn(pinButton, "blur");

    await user.keyboard("{Enter}");

    // Keyboard activations land with `event.detail === 0`; focus must
    // stay on the button so a sighted keyboard user can keep tabbing
    // from where they were, and the action cluster remains visible via
    // :focus-within while the user is navigating by focus.
    expect(blurSpy).not.toHaveBeenCalled();
    expect(useRecentsPrefsStore.getState().pinnedIssueIds).toEqual(["issue-2"]);
  });

  it("hides a pinned done issue when Hide completed is enabled", async () => {
    const userInteract = userEvent.setup();
    listRecentIssuesMock.mockImplementation(
      ({ workspace_id }: { workspace_id: string }) =>
        Promise.resolve({
          issues:
            workspace_id === "ws-1"
              ? [
                  makeIssue({
                    id: "issue-done",
                    workspace_id: "ws-1",
                    identifier: "TES-3",
                    title: "Done pinned issue",
                    status: "done",
                    updated_at: "2026-01-03T00:00:00Z",
                  }),
                ]
              : [
                  makeIssue({
                    id: "issue-open",
                    workspace_id: "ws-2",
                    identifier: "PRD-4",
                    title: "Open issue",
                    updated_at: "2026-01-04T00:00:00Z",
                  }),
                ],
          total: 1,
        }),
    );
    useRecentsPrefsStore.setState({ pinnedIssueIds: ["issue-done"] });

    renderSidebar();

    const pinnedSection = await screen.findByTestId("recents-pinned-section");
    expect(within(pinnedSection).getByText("Done pinned issue")).toBeInTheDocument();

    await userInteract.click(screen.getByLabelText("Filter recents"));
    await userInteract.click(await screen.findByText("Hide completed"));

    await waitFor(() => {
      expect(
        screen.queryByTestId("recents-pinned-section"),
      ).not.toBeInTheDocument();
    });
    expect(screen.queryByText("Done pinned issue")).not.toBeInTheDocument();
    expect(screen.getByText("Open issue")).toBeInTheDocument();
  });
});
