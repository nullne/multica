import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { mockUser, mockWorkspace, mockMembers } from "@/test/helpers";
import type { RecurringTemplate } from "@/shared/types";

// vi.mock factories are hoisted above top-level `const` declarations, so any
// values they reference must be defined via vi.hoisted.
const mocks = vi.hoisted(() => ({
  api: {
    listRecurringTemplates: vi.fn(),
    createRecurringTemplate: vi.fn(),
    updateRecurringTemplate: vi.fn(),
    deleteRecurringTemplate: vi.fn(),
  },
}));

vi.mock("@/features/auth", () => ({
  useAuthStore: (selector: (s: unknown) => unknown) =>
    selector({ user: mockUser, isLoading: false }),
}));

const workspaceState = {
  workspace: mockWorkspace,
  workspaces: [mockWorkspace],
  members: mockMembers,
  agents: [],
};
vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: Object.assign(
    (selector: (s: unknown) => unknown) => selector(workspaceState),
    { getState: () => workspaceState },
  ),
  useActorName: () => ({
    getMemberName: (id: string) => (id === "user-1" ? "Test User" : "Unknown"),
    getAgentName: () => "Unknown Agent",
    getActorName: (type: string, id: string) => {
      if (type === "member" && id === "user-1") return "Test User";
      return "Unknown";
    },
    getActorInitials: () => "TU",
  }),
}));

// Mock the assignee picker so it doesn't pull in heavy issue UI for tests.
vi.mock("@/features/issues/components/pickers", () => ({
  AssigneePicker: ({
    trigger,
    onUpdate,
  }: {
    trigger: React.ReactNode;
    onUpdate: (u: Record<string, unknown>) => void;
  }) => (
    <div>
      <div data-testid="assignee-trigger">{trigger}</div>
      <button
        type="button"
        data-testid="select-member"
        onClick={() => onUpdate({ assignee_type: "member", assignee_id: "user-1" })}
      >
        Select Test User
      </button>
    </div>
  ),
}));

vi.mock("@/components/common/actor-avatar", () => ({
  ActorAvatar: ({ actorId }: { actorId: string }) => (
    <span data-testid={`avatar-${actorId}`} />
  ),
}));

vi.mock("@/shared/api", () => ({
  api: mocks.api,
}));

vi.mock("@/features/recurring-templates/store", async () => {
  const { create } = await import("zustand");
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const store = create<any>((set: unknown) => {
    void set;
    return {
      templates: [],
      loading: false,
      fetch: vi.fn(),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      setTemplates: (t: any) => store.setState({ templates: t }),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      addTemplate: (t: any) =>
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        store.setState((s: any) => ({ templates: [...s.templates, t] })),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      updateTemplate: (id: string, u: any) =>
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        store.setState((s: any) => ({
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          templates: s.templates.map((t: any) => (t.id === id ? { ...t, ...u } : t)),
        })),
      removeTemplate: (id: string) =>
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        store.setState((s: any) => ({
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          templates: s.templates.filter((t: any) => t.id !== id),
        })),
    };
  });
  return {
    useRecurringTemplateStore: Object.assign(
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (sel: any) => sel(store.getState()),
      { getState: store.getState, setState: store.setState },
    ),
  };
});

// Import after mocks so the manager picks up the mocked store.
import { RecurringTemplateManager } from "./recurring-template-manager";
import { useRecurringTemplateStore } from "@/features/recurring-templates/store";

const mockTemplate: RecurringTemplate = {
  id: "tpl-1",
  workspace_id: "ws-1",
  title: "Weekly Review",
  description: "Review items",
  priority: "medium",
  schedule: "0 9 * * 1",
  timezone: "UTC",
  enabled: true,
  next_run_at: "2026-05-11T09:00:00Z",
  created_by_id: "user-1",
  created_by_type: "member",
  created_at: "2026-05-01T00:00:00Z",
  updated_at: "2026-05-01T00:00:00Z",
};

beforeEach(() => {
  vi.clearAllMocks();
  mocks.api.listRecurringTemplates.mockResolvedValue([mockTemplate]);
  // Reset store with the mock template seeded so list-render tests don't
  // depend on the manager's mount-time fetch resolving first.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (useRecurringTemplateStore as any).setState({ templates: [mockTemplate], loading: false });
});

describe("RecurringTemplateManager", () => {
  it("renders the header and new template button for admins", () => {
    render(<RecurringTemplateManager />);
    expect(screen.getByText("Recurring Templates")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /new template/i })).toBeInTheDocument();
  });

  it("shows existing templates with schedule and next run", () => {
    render(<RecurringTemplateManager />);
    expect(screen.getByText("Weekly Review")).toBeInTheDocument();
    expect(screen.getByText("0 9 * * 1")).toBeInTheDocument();
    expect(screen.getByText(/Next:/)).toBeInTheDocument();
  });

  it("opens create dialog when New template is clicked", async () => {
    const user = userEvent.setup();
    render(<RecurringTemplateManager />);
    await user.click(screen.getByRole("button", { name: /new template/i }));
    expect(screen.getByText("New recurring template")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Weekly standup issue")).toBeInTheDocument();
  });

  it("includes assignee in createRecurringTemplate payload", async () => {
    const newTemplate: RecurringTemplate = {
      ...mockTemplate,
      id: "tpl-2",
      title: "Daily Standup",
      schedule: "0 9 * * *",
      assignee_type: "member",
      assignee_id: "user-1",
    };
    mocks.api.createRecurringTemplate.mockResolvedValueOnce(newTemplate);

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    await user.click(screen.getByRole("button", { name: /new template/i }));
    await user.type(screen.getByPlaceholderText("Weekly standup issue"), "Daily Standup");
    await user.type(screen.getByPlaceholderText("0 9 * * 1"), "0 9 * * *");
    await user.click(screen.getByTestId("select-member"));
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() => {
      expect(mocks.api.createRecurringTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Daily Standup",
          schedule: "0 9 * * *",
          assignee_type: "member",
          assignee_id: "user-1",
        }),
      );
    });
  });

  it("sends explicit nulls for cleared fields when updating", async () => {
    const seededTemplate: RecurringTemplate = {
      ...mockTemplate,
      assignee_type: "member",
      assignee_id: "user-1",
      due_date_offset_hours: 48,
    };
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (useRecurringTemplateStore as any).setState({
      templates: [seededTemplate],
      loading: false,
    });
    mocks.api.updateRecurringTemplate.mockResolvedValueOnce({
      ...seededTemplate,
      assignee_type: undefined,
      assignee_id: undefined,
      description: undefined,
      due_date_offset_hours: undefined,
    });

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    // Open the edit dialog. The pencil icon is the second hover-action button
    // on the row (after the toggle switch).
    const row = screen.getByText("Weekly Review").closest(".group");
    if (!row) throw new Error("row not found");
    const buttons = row.querySelectorAll("button");
    const pencil = buttons[buttons.length - 2];
    if (!pencil) throw new Error("edit button not found");
    await act(async () => {
      fireEvent.click(pencil);
    });

    // Clear description and due-date offset, then save.
    const description = screen.getByPlaceholderText(
      "Optional description for issues created from this template",
    );
    await user.clear(description);
    const offset = screen.getByPlaceholderText("Optional, e.g. 72 for 3 days");
    await user.clear(offset);
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(mocks.api.updateRecurringTemplate).toHaveBeenCalledWith(
        "tpl-1",
        expect.objectContaining({
          description: null,
          due_date_offset_hours: null,
        }),
      );
    });
  });

  it("calls deleteRecurringTemplate after confirmation", async () => {
    mocks.api.deleteRecurringTemplate.mockResolvedValueOnce(undefined);
    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    const row = screen.getByText("Weekly Review").closest(".group");
    if (!row) throw new Error("row not found");
    const buttons = row.querySelectorAll("button");
    const trash = buttons[buttons.length - 1];
    if (!trash) throw new Error("delete button not found");
    await act(async () => {
      fireEvent.click(trash);
    });

    await waitFor(() =>
      expect(screen.getByText(/delete recurring template/i)).toBeInTheDocument(),
    );

    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    await waitFor(() => {
      expect(mocks.api.deleteRecurringTemplate).toHaveBeenCalledWith("tpl-1");
    });
  });

  it("toggles enabled state via switch", async () => {
    const updatedTemplate = { ...mockTemplate, enabled: false };
    mocks.api.updateRecurringTemplate.mockResolvedValueOnce(updatedTemplate);

    render(<RecurringTemplateManager />);

    const switches = screen.getAllByRole("switch");
    const first = switches[0];
    if (!first) throw new Error("toggle switch not found");
    await act(async () => {
      fireEvent.click(first);
    });

    await waitFor(() => {
      expect(mocks.api.updateRecurringTemplate).toHaveBeenCalledWith("tpl-1", {
        enabled: false,
      });
    });
  });
});
