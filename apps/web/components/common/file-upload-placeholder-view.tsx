"use client";

import { NodeViewWrapper } from "@tiptap/react";
import type { NodeViewProps } from "@tiptap/react";
import { Loader2 } from "lucide-react";

export function FileUploadPlaceholderView({ node }: NodeViewProps) {
  const filename: string = node.attrs.filename || "file";
  return (
    <NodeViewWrapper as="span" className="inline">
      <span
        contentEditable={false}
        data-file-upload-placeholder
        className="inline-flex max-w-full items-center gap-1.5 rounded-md border border-dashed bg-muted/40 px-2 py-0.5 align-middle text-xs text-muted-foreground select-none"
      >
        <Loader2 className="h-3 w-3 shrink-0 animate-spin" aria-hidden="true" />
        <span className="truncate">Uploading {filename}…</span>
      </span>
    </NodeViewWrapper>
  );
}
