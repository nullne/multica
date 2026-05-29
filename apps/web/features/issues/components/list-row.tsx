"use client";

import { memo } from "react";
import Link from "next/link";
import type { Issue } from "@/shared/types";
import { AbsoluteTime } from "@/components/common/absolute-time";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useIssueSelectionStore } from "@/features/issues/stores/selection-store";
import { useIssueStore } from "@/features/issues/store";
import { useWorkspaceStore } from "@/features/workspace";
import { issueUrl } from "@/features/issues/utils/url";
import { PriorityIcon } from "./priority-icon";
import { AgentDispatchBadge } from "./agent-dispatch-badge";
import { RunningIndicatorRing } from "./running-indicator-ring";

export const ListRow = memo(function ListRow({ issue }: { issue: Issue }) {
  const selected = useIssueSelectionStore((s) => s.selectedIds.has(issue.id));
  const toggle = useIssueSelectionStore((s) => s.toggle);
  const isActive = useIssueStore((s) => s.activeIssueId === issue.id);
  const workspaceSlug = useWorkspaceStore((s) => s.workspace?.slug ?? "");

  // Highlight precedence: currently opened (active) > batch selected > hover.
  // The active state uses a primary leading bar so it stays distinct from the
  // softer accent tint used for multi-select.
  const rowStateClass = isActive
    ? "bg-accent text-accent-foreground before:absolute before:left-0 before:top-1 before:bottom-1 before:w-[3px] before:rounded-r before:bg-primary"
    : selected
      ? "bg-accent/30"
      : "";

  return (
    <div
      aria-current={isActive ? "true" : undefined}
      data-active={isActive || undefined}
      className={`group/row relative flex h-9 items-center gap-2 px-2 md:px-4 text-sm transition-colors hover:bg-accent/50 ${rowStateClass}`}
    >
      <RunningIndicatorRing
        issueId={issue.id}
        className="relative w-4 h-4"
      >
        <PriorityIcon
          priority={issue.priority}
          className={selected ? "hidden" : "group-hover/row:hidden"}
        />
        <input
          type="checkbox"
          checked={selected}
          onChange={() => toggle(issue.id)}
          className={`absolute inset-0 cursor-pointer accent-primary ${
            selected ? "" : "hidden group-hover/row:block"
          }`}
        />
      </RunningIndicatorRing>
      <Link
        href={issueUrl(issue.id, workspaceSlug)}
        className="flex flex-1 items-center gap-2 min-w-0"
      >
        <span className="hidden md:inline w-16 shrink-0 text-xs text-muted-foreground">
          {issue.identifier}
        </span>
        <span className="min-w-0 flex-1 truncate">{issue.title}</span>
        {issue.labels.length > 0 && (
          <span className="hidden md:flex items-center gap-1 shrink-0">
            {issue.labels.slice(0, 3).map((l) => (
              <span
                key={l.id}
                className="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px] border truncate max-w-[80px]"
                style={{
                  borderColor: l.color + "40",
                  backgroundColor: l.color + "15",
                }}
              >
                <span
                  className="h-1.5 w-1.5 rounded-full shrink-0"
                  style={{ backgroundColor: l.color }}
                />
                <span className="truncate">{l.name}</span>
              </span>
            ))}
            {issue.labels.length > 3 && (
              <span className="text-[10px] text-muted-foreground">
                +{issue.labels.length - 3}
              </span>
            )}
          </span>
        )}
        {issue.due_date && (
          <AbsoluteTime
            value={issue.due_date}
            style="shortDate"
            className="hidden sm:inline shrink-0 text-xs text-muted-foreground"
          />
        )}
        {issue.assignee_type && issue.assignee_id && (
          issue.assignee_type === "agent" ? (
            <AgentDispatchBadge issue={issue} layout="inline" />
          ) : (
            <ActorAvatar
              actorType={issue.assignee_type}
              actorId={issue.assignee_id}
              size={20}
            />
          )
        )}
      </Link>
    </div>
  );
});
