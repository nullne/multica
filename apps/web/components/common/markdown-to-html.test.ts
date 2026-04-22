import { describe, it, expect } from "vitest";
import { markdownToHtml } from "./markdown-to-html";

describe("markdownToHtml", () => {
  describe("literal \\n escape sequence handling", () => {
    it("converts literal \\n to real newlines in plain text", () => {
      const input = "line1\\nline2\\nline3";
      const html = markdownToHtml(input);
      // After converting \n → real newlines, marked renders them as paragraphs or <br>
      expect(html).not.toContain("\\n");
      expect(html).toContain("line1");
      expect(html).toContain("line2");
      expect(html).toContain("line3");
    });

    it("converts literal \\n\\n to paragraph breaks", () => {
      const input = "paragraph1\\n\\nparagraph2";
      const html = markdownToHtml(input);
      expect(html).not.toContain("\\n");
      // Two newlines = two paragraphs
      expect(html).toContain("<p>paragraph1</p>");
      expect(html).toContain("<p>paragraph2</p>");
    });

    it("preserves \\n inside inline code", () => {
      const input = "use `\\n` for newlines";
      const html = markdownToHtml(input);
      expect(html).toContain("\\n");
    });

    it("preserves \\n inside fenced code blocks", () => {
      const input = "text before\n```\nprint(\"hello\\nworld\")\n```\ntext after";
      const html = markdownToHtml(input);
      // The \n inside the code block should be preserved as literal text
      expect(html).toContain("\\n");
    });

    it("handles mixed literal \\n and real newlines", () => {
      const input = "line1\\nline2\nline3";
      const html = markdownToHtml(input);
      expect(html).not.toContain("\\n");
      expect(html).toContain("line1");
      expect(html).toContain("line2");
      expect(html).toContain("line3");
    });

    it("converts literal \\t to real tabs", () => {
      const input = "col1\\tcol2";
      const html = markdownToHtml(input);
      expect(html).not.toContain("\\t");
    });

    it("handles agent-style output with escaped newlines", () => {
      // Simulates what happens when an agent posts via CLI --content "..."
      const input =
        "结论：线上库不一致。\\n\\n对比结果：\\n- migration 最新到 000045\\n- 线上记录 version=34";
      const html = markdownToHtml(input);
      expect(html).not.toContain("\\n");
      expect(html).toContain("结论");
      expect(html).toContain("对比结果");
      // The list items should be rendered
      expect(html).toContain("migration");
    });

    it("does nothing when no literal \\n present", () => {
      const input = "normal text\nwith real newlines";
      const html = markdownToHtml(input);
      expect(html).toContain("normal text");
    });
  });
});
