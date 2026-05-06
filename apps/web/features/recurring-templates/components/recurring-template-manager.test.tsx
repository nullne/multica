import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RecurringTemplateManager } from "./recurring-template-manager";
import { mockUser, mockWorkspace, mockMembers } from "@/test/helpers";
import type { RecurringTemplate } from "@/shared/types";

vi.mock("@/features/auth", () => ({
  useAuthStore: (selector: (s: any) => any) =>
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
    (selector: (s: any) => any) => selector(workspaceState),
    { getState: () => workspaceState },
  ),
  useActorName: () => ({}),
}));

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

const mockApi = {
  listRecurringTemplates: vi.fn().mockResolvedValue([mockTemplate]),
  createRecurringTemplate: vi.fn(),
  updateRecurringTemplate: vi.fn(),
  deleteRecurringTemplate: vi.fn(),
};

vi.mock("@/shared/api", () => ({
  api: mockApi,
}));

vi.mock("@/features/recurring-templates/store", async () => {
  const { create } = await import("zustand");
  const store = create<any>((set: any) => ({
    templates: [],
    loading: false,
    fetch: vi.fn().mockImplementation(async () => {
      store.setState({ templates: [mockTemplate], loading: false });
    }),
    addTemplate: (t: any) => store.setState((s: any) => ({ templates: [...s.templates, t] })),
    updateTemplate: (id: string, u: any) =>
      store.setState((s: any) => ({
        templates: s.templates.map((t: any) => (t.id === id ? { ...t, ...u } : t)),
      })),
    removeTemplate: (id: string) =>
      store.setState((s: any) => ({ templates: s.templates.filter((t: any) => t.id !== id) })),
  }));
  return {
    useRecurringTemplateStore: Object.assign(
      (sel: any) => sel(store.getState()),
      { getState: store.getState, setState: store.setState },
    ),
  };
});

describe("RecurringTemplateManager", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the header and new template button for admins", async () => {
    render(<RecurringTemplateManager />);
    expect(screen.getByText("Recurring Templates")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /new template/i })).toBeInTheDocument();
  });

  it("shows existing templates", async () => {
    render(<RecurringTemplateManager />);
    await waitFor(() => {
      expect(screen.getByText("Weekly Review")).toBeInTheDocument();
      expect(screen.getByText("0 9 * * 1")).toBeInTheDocument();
    });
  });

  it("opens create dialog when New template is clicked", async () => {
    const user = userEvent.setup();
    render(<RecurringTemplateManager />);
    await user.click(screen.getByRole("button", { name: /new template/i }));
    expect(screen.getByText("New recurring template")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Weekly standup issue")).toBeInTheDocument();
  });

  it("calls createRecurringTemplate on submit", async () => {
    const newTemplate = { ...mockTemplate, id: "tpl-2", title: "Daily Standup", schedule: "0 9 * * *" };
    mockApi.createRecurringTemplate.mockResolvedValueOnce(newTemplate);

    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    await user.click(screen.getByRole("button", { name: /new template/i }));
    await user.type(screen.getByPlaceholderText("Weekly standup issue"), "Daily Standup");
    await user.type(screen.getByPlaceholderText("0 9 * * 1"), "0 9 * * *");
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() => {
      expect(mockApi.createRecurringTemplate).toHaveBeenCalledWith(
        expect.objectContaining({ title: "Daily Standup", schedule: "0 9 * * *" }),
      );
    });
  });

  it("calls deleteRecurringTemplate after confirmation", async () => {
    mockApi.deleteRecurringTemplate.mockResolvedValueOnce(undefined);
    const user = userEvent.setup();
    render(<RecurringTemplateManager />);

    await waitFor(() => expect(screen.getByText("Weekly Review")).toBeInTheDocument());

    const deleteBtn = screen.getByRole("button", { name: "" });
    // Hover to reveal delete button, then click it
    fireEvent.mouseEnter(screen.getByText("Weekly Review").closest(".group")!);
    const deleteBtns = screen.getAllByRole("button");
    const trashBtn = deleteBtns.find((b) => b.querySelector("svg"));
    // Find the delete button specifically by clicking the trash icon area
    const row = screen.getByText("Weekly Review").closest(".group")!;
    const buttons = row.querySelectorAll("button");
    // Last button is delete
    await act(async () => {
      fireEvent.click(buttons[buttons.length - 1]);
    });

    await waitFor(() =>
      expect(screen.getByText(/delete recurring template/i)).toBeInTheDocument(),
    );

    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    await waitFor(() => {
      expect(mockApi.deleteRecurringTemplate).toHaveBeenCalledWith("tpl-1");
    });
  });

  it("toggles enabled state via switch", async () => {
    const updatedTemplate = { ...mockTemplate, enabled: false };
    mockApi.updateRecurringTemplate.mockResolvedValueOnce(updatedTemplate);

    render(<RecurringTemplateManager />);
    await waitFor(() => expect(screen.getByText("Weekly Review")).toBeInTheDocument());

    const switches = screen.getAllByRole("switch");
    await act(async () => {
      fireEvent.click(switches[0]);
    });

    await waitFor(() => {
      expect(mockApi.updateRecurringTemplate).toHaveBeenCalledWith("tpl-1", { enabled: false });
    });
  });
});
