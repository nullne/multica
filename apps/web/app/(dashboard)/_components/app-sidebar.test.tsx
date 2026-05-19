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

beforeEach(() => {
  vi.clearAllMocks();
  mockPathname = "/issues";
  mockSearchParams = new URLSearchParams();
  switchWorkspaceMock.mockImplementation(async () => {});
  useRecentsStore.getState().clear();
  useRecentsPrefsStore.setState({
    collapsedWorkspaceIds: [],
    pinnedWorkspaceIds: [],
  });
  listRecentIssuesMock.mockImplementation(defaultMockImpl);
});

describe("AppSidebar user menu", () => {
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

  it("pins a workspace group and renders it before unpinned groups", async () => {
    const user = userEvent.setup();
    renderSidebar();

    expect(await screen.findByText("Other workspace issue")).toBeInTheDocument();

    const initialGroups = screen.getAllByTestId(/^recents-group-ws-/);
    const initialOrder = initialGroups.map((el) => el.dataset.testid);
    expect(initialOrder).toContain("recents-group-ws-1");
    expect(initialOrder).toContain("recents-group-ws-2");
    expect(initialGroups.every((el) => el.dataset.pinned === "false")).toBe(true);

    await user.click(screen.getByRole("button", { name: /Pin Test Workspace/i }));

    const reorderedGroups = screen.getAllByTestId(/^recents-group-ws-/);
    const firstGroup = reorderedGroups[0]!;
    const secondGroup = reorderedGroups[1]!;
    expect(firstGroup.dataset.testid).toBe("recents-group-ws-1");
    expect(firstGroup.dataset.pinned).toBe("true");
    expect(secondGroup.dataset.testid).toBe("recents-group-ws-2");
    expect(secondGroup.dataset.pinned).toBe("false");
  });

  it("unpins a workspace group from the pin toggle", async () => {
    const user = userEvent.setup();
    useRecentsPrefsStore.setState({ pinnedWorkspaceIds: ["ws-2"] });
    renderSidebar();

    expect(await screen.findByText("Other workspace issue")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Unpin Prod Debug/i }));

    expect(useRecentsPrefsStore.getState().pinnedWorkspaceIds).toEqual([]);
  });

  it("reorders pinned workspaces with the move buttons", async () => {
    const user = userEvent.setup();
    useRecentsPrefsStore.setState({ pinnedWorkspaceIds: ["ws-1", "ws-2"] });
    renderSidebar();

    expect(await screen.findByText("Other workspace issue")).toBeInTheDocument();
    expect(
      screen.getAllByTestId(/^recents-group-ws-/).map((el) => el.dataset.testid),
    ).toEqual(["recents-group-ws-1", "recents-group-ws-2"]);

    await user.click(
      screen.getByRole("button", { name: /Move Prod Debug up/i }),
    );

    expect(useRecentsPrefsStore.getState().pinnedWorkspaceIds).toEqual([
      "ws-2",
      "ws-1",
    ]);
    expect(
      screen.getAllByTestId(/^recents-group-ws-/).map((el) => el.dataset.testid),
    ).toEqual(["recents-group-ws-2", "recents-group-ws-1"]);

    await user.click(
      screen.getByRole("button", { name: /Move Prod Debug down/i }),
    );

    expect(useRecentsPrefsStore.getState().pinnedWorkspaceIds).toEqual([
      "ws-1",
      "ws-2",
    ]);
  });
});
