import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FileUploadButton } from "./file-upload-button";

describe("FileUploadButton", () => {
  it("renders the paperclip icon when idle", () => {
    render(<FileUploadButton onSelect={() => {}} />);
    const button = screen.getByRole("button", { name: /attach file/i });
    expect(button).not.toBeDisabled();
    expect(button.querySelector(".animate-spin")).toBeNull();
  });

  it("swaps in a spinner and disables the button while busy", async () => {
    const onSelect = vi.fn();
    render(<FileUploadButton busy onSelect={onSelect} />);

    const button = screen.getByRole("button", { name: /uploading file/i });
    expect(button).toBeDisabled();
    expect(button.querySelector(".animate-spin")).not.toBeNull();
    expect(button).toHaveAttribute("aria-busy", "true");

    // Clicks while busy must not trigger another file dialog / selection.
    await userEvent.click(button);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("is disabled when `disabled` is passed, even without busy", () => {
    render(<FileUploadButton disabled onSelect={() => {}} />);
    const button = screen.getByRole("button");
    expect(button).toBeDisabled();
    expect(button.querySelector(".animate-spin")).toBeNull();
  });
});
