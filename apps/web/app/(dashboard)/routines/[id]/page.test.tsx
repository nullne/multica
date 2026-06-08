import { describe, expect, it, vi } from "vitest";

const redirectMock = vi.fn();

vi.mock("next/navigation", () => ({
  redirect: (...args: unknown[]) => redirectMock(...args),
}));

import RoutineDetailPage from "./page";

describe("RoutineDetailPage", () => {
  it("redirects legacy routine detail routes to the split routines page", async () => {
    await RoutineDetailPage({ params: Promise.resolve({ id: "routine-1" }) });

    expect(redirectMock).toHaveBeenCalledWith("/routines?routine=routine-1");
  });
});
