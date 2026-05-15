import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const pushMock = vi.fn();
const listIssuesMock = vi.fn();
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
    listIssues: (...args: unknown[]) => listIssuesMock(...args),
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
  switchWorkspace: vi.fn(),
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

const issueState = {
  issues: [
    {
      id: "issue-1",
      workspace_id: "ws-1",
      identifier: "TES-1",
      title: "Recent issue",
      status: "todo",
      priority: "medium",
      creator_type: "member",
      creator_id: "user-1",
      updated_at: "2026-01-01T00:00:00Z",
      created_at: "2026-01-01T00:00:00Z",
    },
  ],
};
vi.mock("@/features/issues/store", () => ({
  useIssueStore: Object.assign(
    (selector?: (s: typeof issueState) => unknown) =>
      selector ? selector(issueState) : issueState,
    { getState: () => issueState },
  ),
}));

import { SidebarProvider } from "@/components/ui/sidebar";
import { AppSidebar } from "./app-sidebar";

function renderSidebar() {
  return render(
    <SidebarProvider>
      <AppSidebar />
    </SidebarProvider>,
  );
}

describe("AppSidebar user menu", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPathname = "/issues";
    mockSearchParams = new URLSearchParams();
    listIssuesMock.mockImplementation(({ workspace_id }: { workspace_id: string }) =>
      Promise.resolve({
        issues:
          workspace_id === "ws-2"
            ? [
                {
                  id: "issue-2",
                  workspace_id: "ws-2",
                  identifier: "PRD-2",
                  title: "Other workspace issue",
                  status: "todo",
                  priority: "medium",
                  creator_type: "member",
                  creator_id: "user-1",
                  updated_at: "2026-01-02T00:00:00Z",
                  created_at: "2026-01-02T00:00:00Z",
                },
              ]
            : issueState.issues,
      })
    );
  });

  it("opens the bottom user menu without throwing", async () => {
    const user = userEvent.setup();
    renderSidebar();

    const trigger = screen.getByRole("button", { name: /Test User/i });
    await user.click(trigger);

    expect(await screen.findByText("Account settings")).toBeInTheDocument();
    expect(screen.getByText("Log out")).toBeInTheDocument();
    // Email is shown both on the trigger and inside the menu header.
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
    expect(screen.getByText("Recent issue")).toBeInTheDocument();
    expect(await screen.findByText("Prod Debug")).toBeInTheDocument();
    expect(screen.getByText("Other workspace issue")).toBeInTheDocument();
    expect(screen.queryByText("Inbox")).not.toBeInTheDocument();
    expect(screen.queryByText("Agents")).not.toBeInTheDocument();
  });
});
