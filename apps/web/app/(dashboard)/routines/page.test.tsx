import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const mocks = vi.hoisted(() => ({
  api: {
    createRoutine: vi.fn(),
    updateRoutine: vi.fn(),
    listRoutines: vi.fn(),
    getRoutine: vi.fn(),
    listRoutineRuns: vi.fn(),
  },
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
  searchParams: new URLSearchParams(),
}));

vi.mock("@/features/issues/components/pickers", () => ({
  AssigneePicker: ({
    trigger,
    onUpdate,
  }: {
    trigger: React.ReactNode;
    onUpdate: (patch: Record<string, unknown>) => void;
  }) => (
    <div data-testid="issue-assignee-picker">
      {trigger}
      <button
        type="button"
        onClick={() =>
          onUpdate({
            assignee_type: "agent",
            assignee_id: "agent-1",
            dispatch_provider: "codex",
            dispatch_daemon_id: "daemon-1",
            dispatch_daemon_label: null,
          })
        }
      >
        Select Test Agent
      </button>
    </div>
  ),
}));

vi.mock("@/shared/api", () => ({
  api: mocks.api,
}));

vi.mock("sonner", () => ({
  toast: mocks.toast,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.searchParams,
}));

import RoutinesPage from "./page";

describe("RoutinesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.api.listRoutines.mockResolvedValue([]);
    mocks.api.getRoutine.mockResolvedValue(null);
    mocks.api.listRoutineRuns.mockResolvedValue([]);
    mocks.api.createRoutine.mockResolvedValue({
        id: "routine-1",
        workspace_id: "ws-1",
      name: "Daily review",
      triggers: [],
      actions: [],
      subscriber_ids: [],
      label_ids: [],
      enabled: true,
    });
    mocks.api.updateRoutine.mockResolvedValue({});
  });

  it("renders the routines list by default", () => {
    mocks.searchParams = new URLSearchParams();
    render(<RoutinesPage />);

    expect(screen.getByRole("heading", { name: "Routines" })).toBeInTheDocument();
    expect(screen.getByText("Loading routines...")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /New routine/i })).toHaveAttribute("href", "/routines?new=1");
  });

  it("renders routines returned by the API", async () => {
    mocks.searchParams = new URLSearchParams();
    mocks.api.listRoutines.mockResolvedValue([
      {
        id: "routine-1",
        workspace_id: "ws-1",
        name: "Daily code review",
        triggers: [{ id: "trigger-1" }],
        actions: [],
        subscriber_ids: [],
        label_ids: [],
        enabled: true,
      },
    ]);
    render(<RoutinesPage />);

    expect(await screen.findByText("Daily code review")).toBeInTheDocument();
    expect(screen.getByText("1 trigger · Enabled")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Daily code review/i })).toHaveAttribute("href", "/routines/routine-1");
    expect(screen.queryByText("View")).not.toBeInTheDocument();
  });

  it("renders the routine issue template and trigger configuration surface", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1");
    render(<RoutinesPage />);

    expect(screen.getByTestId("routines-page-scroll")).toHaveClass("overflow-y-auto");
    expect(screen.getByRole("heading", { name: "New routine" })).toBeInTheDocument();
    expect(screen.getByLabelText(/Name/)).toBeInTheDocument();
    expect(screen.getByLabelText("Instructions")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Create routine/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Create routine/i })).toBeDisabled();
    expect(screen.queryByText("Save preview")).not.toBeInTheDocument();

    expect(screen.getByRole("heading", { name: "Issue template" })).toBeInTheDocument();
    expect(screen.getByText("Assignee")).toBeInTheDocument();
    expect(screen.getByText("Subscribers")).toBeInTheDocument();
    expect(screen.getByText("Labels")).toBeInTheDocument();
    expect(screen.getByTestId("issue-assignee-picker")).toBeInTheDocument();
    expect(screen.queryByText("Dispatch")).not.toBeInTheDocument();

    expect(screen.queryByText(/Runs once on/)).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Once" })).not.toBeInTheDocument();
    expect(screen.queryByText("Run on a recurring cron schedule or once at a future time")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /GitHub event/i })).not.toBeInTheDocument();
    const addTrigger = screen.getByRole("button", { name: /Add another trigger/i });

    await user.click(addTrigger);

    const scheduleOption = screen.getByRole("button", { name: /Schedule/i });
    const githubTrigger = screen.getByRole("button", { name: /GitHub event/i });
    expect(scheduleOption).toBeInTheDocument();
    expect(githubTrigger).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /API/i })).toBeInTheDocument();
    await user.click(scheduleOption);
    const scheduleTrigger = screen.getByRole("button", { name: /Runs once on/ });
    expect(scheduleTrigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByText(/Runs once on/)).toHaveLength(1);
    expect(screen.getByRole("tab", { name: "Once" })).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Custom" }));
    expect(screen.getByDisplayValue("0 9 * * 1")).toBeInTheDocument();
    expect(screen.getByText("Format: minute hour day-of-month month day-of-week")).toBeInTheDocument();

    await user.click(addTrigger);
    expect(screen.getByRole("button", { name: /Runs on cron 0 9/ })).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByDisplayValue("0 9 * * 1")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /GitHub event/i }));

    expect(screen.getByRole("button", { name: /Runs on cron 0 9/ })).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("Run on a recurring cron schedule or once at a future time")).toBeInTheDocument();
    expect(screen.getAllByText("PR opened").length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByRole("tab", { name: "PR opened" })).not.toBeInTheDocument();
    expect(screen.getByText("Pull requests")).toBeInTheDocument();
    const pullRequestGroup = screen.getByText("Pull requests").parentElement!.parentElement!;
    expect(within(pullRequestGroup).getByRole("button", { name: "Opened" })).toHaveAttribute("aria-pressed", "true");
    expect(within(pullRequestGroup).getByRole("button", { name: "Merged" })).toHaveAttribute("aria-pressed", "false");
    await user.click(within(pullRequestGroup).getByRole("button", { name: "Merged" }));
    expect(screen.getAllByText("PR opened, PR merged").length).toBeGreaterThanOrEqual(1);
    await user.click(within(pullRequestGroup).getByRole("button", { name: "All" }));
    expect(screen.getAllByText("6 events selected").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Add a filter condition")).toBeInTheDocument();
    expect(screen.queryByText("GitHub labels")).not.toBeInTheDocument();

    await user.click(screen.getByText("Add a filter condition"));

    expect(screen.getByDisplayValue("Author")).toBeInTheDocument();
    expect(screen.getByText("is one of")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Values")).toBeInTheDocument();

    await user.type(screen.getByLabelText(/Name/), "Daily review");
    expect(screen.getByRole("button", { name: /Create routine/i })).toBeDisabled();
    await user.type(screen.getByLabelText("Instructions"), "Review the code every day");
    expect(screen.getByRole("button", { name: /Create routine/i })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Select Test Agent" }));
    expect(screen.getByRole("button", { name: /Create routine/i })).not.toBeDisabled();
    await user.click(screen.getByRole("button", { name: /Create routine/i }));

    await waitFor(() => {
      expect(mocks.api.createRoutine).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Daily review",
          instructions: "Review the code every day",
          assignee_type: "agent",
          assignee_id: "agent-1",
          triggers: expect.arrayContaining([
            expect.objectContaining({ trigger_type: "schedule", schedule: "0 9 * * 1" }),
            expect.objectContaining({
              trigger_type: "github",
              config: expect.objectContaining({
                event_types: expect.arrayContaining([
                  "github.pull_request.opened",
                  "github.pull_request.merged",
                ]),
              }),
            }),
          ]),
        }),
      );
    });
  });

  it("loads an existing routine in edit mode and updates it", async () => {
    const user = userEvent.setup();
    mocks.searchParams = new URLSearchParams("new=1&id=routine-1");
    mocks.api.getRoutine.mockResolvedValue({
      id: "routine-1",
      workspace_id: "ws-1",
      name: "Existing routine",
      instructions: "Existing instructions",
      priority: "high",
      assignee_id: "agent-1",
      assignee_type: "agent",
      triggers: [{ id: "trigger-1", trigger_type: "schedule", schedule: "0 9 * * 1", timezone: "UTC", config: {} }],
      actions: [],
      subscriber_ids: [],
      label_ids: [],
      enabled: true,
    });

    render(<RoutinesPage />);

    expect(await screen.findByRole("heading", { name: "Edit routine" })).toBeInTheDocument();
    expect(screen.getByDisplayValue("Existing routine")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Update routine/i })).not.toBeDisabled();
    await user.clear(screen.getByLabelText(/Name/));
    await user.type(screen.getByLabelText(/Name/), "Updated routine");
    await user.click(screen.getByRole("button", { name: /Update routine/i }));

    await waitFor(() => {
      expect(mocks.api.updateRoutine).toHaveBeenCalledWith(
        "routine-1",
        expect.objectContaining({ name: "Updated routine" }),
      );
    });
  });
});
