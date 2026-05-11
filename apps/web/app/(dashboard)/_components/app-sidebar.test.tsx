import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => "/issues",
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
  workspaces: [{ id: "ws-1", name: "Test Workspace", slug: "test" }],
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
});
