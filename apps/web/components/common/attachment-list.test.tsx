import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AttachmentList } from "./attachment-list";
import type { Attachment } from "@/shared/types";

describe("AttachmentList", () => {
  it("uses authenticated download URLs for image links and previews", () => {
    const attachment: Attachment = {
      id: "att-1",
      workspace_id: "ws-1",
      issue_id: "issue-1",
      comment_id: null,
      uploader_type: "member",
      uploader_id: "member-1",
      filename: "screenshot.png",
      url: "https://storage.googleapis.com/bucket/screenshot.png",
      download_url: "https://multica.example/api/attachments/att-1/download",
      content_type: "image/png",
      size_bytes: 123,
      created_at: "2026-05-21T00:00:00Z",
    };

    render(<AttachmentList attachments={[attachment]} />);

    const image = screen.getByRole("img", { name: "screenshot.png" });
    expect(image).toHaveAttribute("src", attachment.download_url);
    expect(image.closest("a")).toHaveAttribute("href", attachment.download_url);
  });
});
