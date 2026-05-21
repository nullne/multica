import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("@/features/issues/components/pickers", () => ({
  AssigneePicker: ({ trigger }: { trigger: React.ReactNode }) => (
    <div data-testid="issue-assignee-picker">{trigger}</div>
  ),
}));

import RoutinesPage from "./page";

describe("RoutinesPage", () => {
  it("renders the routine issue template and trigger configuration surface", async () => {
    const user = userEvent.setup();
    render(<RoutinesPage />);

    expect(screen.getByTestId("routines-page-scroll")).toHaveClass("overflow-y-auto");
    expect(screen.queryByRole("heading", { name: "New routine" })).not.toBeInTheDocument();
    expect(screen.getByLabelText(/Name/)).toBeInTheDocument();
    expect(screen.getByLabelText("Instructions")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Create routine/i })).toBeInTheDocument();
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
  });
});
