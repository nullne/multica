"use client";

import { NodeViewWrapper } from "@tiptap/react";
import type { NodeViewProps } from "@tiptap/react";
import { useIssueStore } from "@/features/issues/store";
import { useWorkspaceStore } from "@/features/workspace";
import { issueUrl } from "@/features/issues/utils/url";
import { StatusIcon } from "@/features/issues/components/status-icon";

/**
 * MentionView — shared NodeView for mention nodes (both editing and readonly).
 *
 * Rendering and behavior are identical in both modes.
 * Issue mentions are always clickable (open in new tab).
 */
export function MentionView({ node }: NodeViewProps) {
  const { type, id, label } = node.attrs;

  if (type === "issue") {
    return (
      <NodeViewWrapper as="span" className="inline">
        <IssueMention issueId={id} fallbackLabel={label} />
      </NodeViewWrapper>
    );
  }

  return (
    <NodeViewWrapper as="span" className="inline">
      <span className="mention">@{label ?? id}</span>
    </NodeViewWrapper>
  );
}

// ---------------------------------------------------------------------------
// IssueMention — inline card, always opens in new tab
// ---------------------------------------------------------------------------

function IssueMention({
  issueId,
  fallbackLabel,
}: {
  issueId: string;
  fallbackLabel?: string;
}) {
  const issue = useIssueStore((s) => s.issues.find((i) => i.id === issueId));
  const workspaceSlug = useWorkspaceStore((s) => s.workspace?.slug ?? "");
  const url = issueUrl(issueId, workspaceSlug);

  const handleClick = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    window.open(url, "_blank", "noopener,noreferrer");
  };

  if (!issue) {
    return (
      <a
        href={url}
        onClick={handleClick}
        className="issue-mention text-primary font-medium cursor-pointer hover:underline"
      >
        {fallbackLabel ?? issueId.slice(0, 8)}
      </a>
    );
  }

  return (
    <a
      href={url}
      onClick={handleClick}
      className="issue-mention inline-flex items-center align-middle gap-1.5 rounded-md border px-2 py-0.5 text-sm hover:bg-accent transition-colors cursor-pointer"
    >
      <StatusIcon status={issue.status} className="h-3.5 w-3.5" />
      <span className="font-medium text-muted-foreground">{issue.identifier}</span>
      <span className="text-foreground">{issue.title}</span>
    </a>
  );
}
