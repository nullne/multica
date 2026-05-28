import { Suspense } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor, act } from "@testing-library/react";

const replaceMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: replaceMock }),
  usePathname: () => "/w/test-ws/issues/issue-1",
}));

// Render IssueDetail as a sentinel so we can assert when the page actually
// mounts the issue view vs. the loading skeleton.
vi.mock("@/features/issues/components", () => ({
  IssueDetail: ({ issueId }: { issueId: string }) => (
    <div data-testid="issue-detail">issue:{issueId}</div>
  ),
}));

vi.mock("@/components/multica-icon", () => ({
  MulticaIcon: () => <span data-testid="multica-icon" />,
}));

const wsA = { id: "ws-a", name: "Workspace A", slug: "ws-a" };
const wsB = { id: "ws-b", name: "Workspace B", slug: "ws-b" };

const switchWorkspaceMock = vi.fn();

// Mutable workspace-store state — tests update `state.workspace` to simulate
// hydration and user-driven workspace switches.
const state: {
  workspace: typeof wsA | null;
  workspaces: typeof wsA[];
} = {
  workspace: wsA,
  workspaces: [wsA, wsB],
};

vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: (
    selector: (s: {
      workspace: typeof wsA | null;
      workspaces: typeof wsA[];
      switchWorkspace: typeof switchWorkspaceMock;
    }) => unknown,
  ) =>
    selector({
      workspace: state.workspace,
      workspaces: state.workspaces,
      switchWorkspace: switchWorkspaceMock,
    }),
}));

import WorkspaceIssueDetailPage from "./page";

async function renderPage(workspaceSlug: string, id: string) {
  let result: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <Suspense fallback={<div>loading</div>}>
        <WorkspaceIssueDetailPage
          params={Promise.resolve({ workspaceSlug, id })}
        />
      </Suspense>,
    );
  });
  return result!;
}

describe("WorkspaceIssueDetailPage", () => {
  beforeEach(() => {
    replaceMock.mockClear();
    switchWorkspaceMock.mockClear();
    state.workspace = wsA;
    state.workspaces = [wsA, wsB];
  });

  it("renders the issue when active workspace matches the URL slug", async () => {
    state.workspace = wsA;
    const { getByTestId } = await renderPage("ws-a", "issue-1");

    await waitFor(() => {
      expect(getByTestId("issue-detail").textContent).toBe("issue:issue-1");
    });
    expect(switchWorkspaceMock).not.toHaveBeenCalled();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("syncs the active workspace to the URL slug on direct navigation", async () => {
    // Direct navigation to /w/ws-b/issues/issue-1 while active workspace is A.
    state.workspace = wsA;
    await renderPage("ws-b", "issue-1");

    await waitFor(() => {
      expect(switchWorkspaceMock).toHaveBeenCalledWith("ws-b");
    });
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("redirects to /issues instead of reverting when user switches workspace", async () => {
    // Start in workspace A on /w/ws-a/issues/issue-1.
    state.workspace = wsA;
    const { rerender } = await renderPage("ws-a", "issue-1");

    expect(switchWorkspaceMock).not.toHaveBeenCalled();

    // Simulate the user switching to workspace B via the sidebar.
    state.workspace = wsB;
    await act(async () => {
      rerender(
        <Suspense fallback={<div>loading</div>}>
          <WorkspaceIssueDetailPage
            params={Promise.resolve({ workspaceSlug: "ws-a", id: "issue-1" })}
          />
        </Suspense>,
      );
    });

    await waitFor(() => {
      expect(replaceMock).toHaveBeenCalledWith("/issues");
    });
    // Critically, we must NOT have called switchWorkspace back to ws-a.
    expect(switchWorkspaceMock).not.toHaveBeenCalled();
  });
});
