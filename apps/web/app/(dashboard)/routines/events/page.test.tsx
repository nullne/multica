import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const mocks = vi.hoisted(() => ({
  api: {
    listRoutineEvents: vi.fn(),
  },
  clipboardWriteText: vi.fn(),
}));

vi.mock("@/shared/api", () => ({
  api: mocks.api,
}));

import RoutineEventsPage from "./page";

describe("RoutineEventsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(window.navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: mocks.clipboardWriteText.mockResolvedValue(undefined),
      },
    });
    Object.defineProperty(globalThis.navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: mocks.clipboardWriteText,
      },
    });
  });

  it("loads recent routine events and fetches the next page when scrolled near the bottom", async () => {
    mocks.api.listRoutineEvents
      .mockResolvedValueOnce(
        Array.from({ length: 20 }, (_, index) => ({
          id: `event-${index + 1}`,
          workspace_id: "ws-1",
          source_type: "github",
          event_type: index === 0 ? "github.pull_request.opened" : "github.pull_request.synchronize",
          dedup_key: `delivery-${index + 1}`,
          external_delivery_id: `delivery-${index + 1}`,
          data: { action: "opened" },
          payload: { title: "Open PR" },
          status: "processed",
          error_message: null,
          created_at: "2026-06-09T08:00:00Z",
          updated_at: "2026-06-09T08:00:00Z",
        })),
      )
      .mockResolvedValueOnce([
        {
          id: "event-2",
          workspace_id: "ws-1",
          source_type: "api",
          event_type: "custom",
          dedup_key: "delivery-2",
          external_delivery_id: null,
          data: {},
          payload: { title: "API call" },
          status: "no_matching_trigger",
          error_message: null,
          created_at: "2026-06-09T07:00:00Z",
          updated_at: "2026-06-09T07:00:00Z",
        },
      ]);

    render(<RoutineEventsPage />);

    expect(await screen.findByText("Routine events")).toBeInTheDocument();
    expect(await screen.findByText("github.pull_request.opened")).toBeInTheDocument();
    expect(screen.getAllByText("processed").length).toBeGreaterThan(0);
    expect(mocks.api.listRoutineEvents).toHaveBeenCalledWith({ limit: 20, offset: 0 });

    const scrollContainer = screen.getByTestId("routine-events-scroll");
    Object.defineProperty(scrollContainer, "scrollHeight", { configurable: true, value: 1000 });
    Object.defineProperty(scrollContainer, "clientHeight", { configurable: true, value: 500 });
    Object.defineProperty(scrollContainer, "scrollTop", { configurable: true, value: 480 });
    fireEvent.scroll(scrollContainer);

    await waitFor(() => {
      expect(mocks.api.listRoutineEvents).toHaveBeenCalledWith({ limit: 20, offset: 20 });
    });
    expect(await screen.findByText("no_matching_trigger")).toBeInTheDocument();
  });

  it("toggles the payload preview by clicking the event card", async () => {
    const user = userEvent.setup();
    mocks.api.listRoutineEvents.mockResolvedValueOnce([
      {
        id: "event-1",
        workspace_id: "ws-1",
        source_type: "api",
        event_type: "custom.demo_event",
        dedup_key: "seed-routine-events-1",
        external_delivery_id: null,
        data: { seeded: true },
        payload: { title: "Copy me" },
        status: "processed",
        error_message: null,
        created_at: "2026-06-09T08:00:00Z",
        updated_at: "2026-06-09T08:00:00Z",
      },
    ]);

    render(<RoutineEventsPage />);

    expect(screen.queryByRole("button", { name: "Copy payload preview" })).not.toBeInTheDocument();
    expect(screen.queryByText(/Copy me/)).not.toBeInTheDocument();
    const eventCard = await screen.findByRole("button", { name: /custom.demo_event/i });

    await user.click(eventCard);

    expect(screen.getByRole("button", { name: "Copy payload preview" })).toBeInTheDocument();
    expect(screen.getByText(/Copy me/)).toBeInTheDocument();

    await user.click(eventCard);

    expect(screen.queryByRole("button", { name: "Copy payload preview" })).not.toBeInTheDocument();
    expect(screen.queryByText(/Copy me/)).not.toBeInTheDocument();
  });

  it("does not toggle the payload preview while text is selected", async () => {
    const user = userEvent.setup();
    const getSelectionSpy = vi.spyOn(window, "getSelection").mockReturnValue({
      toString: () => "custom.demo_event",
    } as Selection);
    mocks.api.listRoutineEvents.mockResolvedValueOnce([
      {
        id: "event-1",
        workspace_id: "ws-1",
        source_type: "api",
        event_type: "custom.demo_event",
        dedup_key: "seed-routine-events-1",
        external_delivery_id: null,
        data: { seeded: true },
        payload: { title: "Copy me" },
        status: "processed",
        error_message: null,
        created_at: "2026-06-09T08:00:00Z",
        updated_at: "2026-06-09T08:00:00Z",
      },
    ]);

    render(<RoutineEventsPage />);

    await user.click(await screen.findByRole("button", { name: /custom.demo_event/i }));

    expect(screen.queryByText(/Copy me/)).not.toBeInTheDocument();
    getSelectionSpy.mockRestore();
  });

  it("reloads events with status source and event type filters", async () => {
    const user = userEvent.setup();
    mocks.api.listRoutineEvents
      .mockResolvedValueOnce([])
      .mockResolvedValue([]);

    render(<RoutineEventsPage />);

    await screen.findByText("No routine events yet");
    await user.selectOptions(screen.getByLabelText("Status"), "error");
    await user.selectOptions(screen.getByLabelText("Source"), "github");
    await user.type(screen.getByLabelText("Event type"), "github.pull_request.opened");

    await waitFor(() => {
      expect(mocks.api.listRoutineEvents).toHaveBeenLastCalledWith({
        limit: 20,
        offset: 0,
        status: "error",
        source_type: "github",
        event_type: "github.pull_request.opened",
      });
    });
  });
});
