"use client";

import Link from "next/link";
import { useIssueStore } from "@/features/issues/store";
import { useWorkspaceStore } from "@/features/workspace";
import { issueUrl } from "@/features/issues/utils/url";
import { StatusIcon } from "./status-icon";

interface IssueMentionCardProps {
  issueId: string;
  /** Fallback text when issue is not in store (e.g. "MUL-7") */
  fallbackLabel?: string;
}

export function IssueMentionCard({ issueId, fallbackLabel }: IssueMentionCardProps) {
  const issue = useIssueStore((s) => s.issues.find((i) => i.id === issueId));
  const workspaceSlug = useWorkspaceStore((s) => s.workspace?.slug ?? "");

  if (!issue) {
    return (
      <Link
        href={issueUrl(issueId, workspaceSlug)}
        className="text-primary font-medium cursor-pointer hover:underline"
      >
        {fallbackLabel ?? issueId.slice(0, 8)}
      </Link>
    );
  }

  return (
    <Link
      href={issueUrl(issueId, workspaceSlug)}
      className="inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-sm hover:bg-accent transition-colors cursor-pointer no-underline"
    >
      <StatusIcon status={issue.status} className="h-3.5 w-3.5" />
      <span className="font-medium text-muted-foreground">{issue.identifier}</span>
      <span className="text-foreground">{issue.title}</span>
    </Link>
  );
}
