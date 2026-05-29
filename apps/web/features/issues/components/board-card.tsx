"use client";

import { useCallback, memo } from "react";
import Link from "next/link";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { toast } from "sonner";
import type { Issue, UpdateIssueRequest } from "@/shared/types";
import { CalendarDays } from "lucide-react";
import { AbsoluteTime } from "@/components/common/absolute-time";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { api } from "@/shared/api";
import { useIssueStore } from "@/features/issues/store";
import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";
import { useWorkspaceStore } from "@/features/workspace";
import { issueUrl } from "@/features/issues/utils/url";
import { PriorityIcon } from "./priority-icon";
import { StatusIcon } from "./status-icon";
import { AgentDispatchBadge } from "./agent-dispatch-badge";
import { RunningIndicatorRing } from "./running-indicator-ring";
import { PriorityPicker, AssigneePicker, DueDatePicker } from "./pickers";
import { PRIORITY_CONFIG } from "@/features/issues/config";
import type { CardProperties } from "@/features/issues/stores/view-store";
import { useViewStore } from "@/features/issues/stores/view-store-context";

/** Stops event from bubbling to Link/drag handlers */
function PickerWrapper({ children }: { children: React.ReactNode }) {
  const stop = (e: React.SyntheticEvent) => {
    e.stopPropagation();
    e.preventDefault();
  };
  return (
    <div onClick={stop} onMouseDown={stop} onPointerDown={stop}>
      {children}
    </div>
  );
}

export const BoardCardContent = memo(function BoardCardContent({
  issue,
  editable = false,
}: {
  issue: Issue;
  editable?: boolean;
}) {
  const storeProperties = useViewStore((s) => s.cardProperties);
  const priorityCfg = PRIORITY_CONFIG[issue.priority];
  const isWorking = useActiveTaskStore((s) => s.tasks.has(issue.id));

  const handleUpdate = useCallback(
    (updates: Partial<UpdateIssueRequest>) => {
      const prev = { ...issue };
      useIssueStore.getState().updateIssue(issue.id, updates);
      api.updateIssue(issue.id, updates).catch(() => {
        useIssueStore.getState().updateIssue(issue.id, prev);
        toast.error("Failed to update issue");
      });
    },
    [issue],
  );

  const showPriority = storeProperties.priority;
  const showDescription = storeProperties.description && issue.description;
  const showAssignee = storeProperties.assignee && issue.assignee_type && issue.assignee_id;
  const showDueDate = storeProperties.dueDate && issue.due_date;
  const showBottom = showAssignee || showDueDate;

  return (
    <div className={`rounded-lg border bg-card p-3.5 shadow-[0_1px_2px_0_rgba(0,0,0,0.03)] transition-shadow group-hover:shadow-sm ${isWorking ? "border-info/30 bg-info/[0.02]" : ""}`}>
      {/* Row 1: Status icon + identifier — the status icon is always shown so
          the running ring has a stable anchor regardless of which card
          properties the user has toggled off. */}
      <div className="flex items-center gap-1.5">
        <RunningIndicatorRing issueId={issue.id} className="size-3.5">
          <StatusIcon status={issue.status} className="size-3.5" />
        </RunningIndicatorRing>
        <p className="text-xs text-muted-foreground">{issue.identifier}</p>
      </div>

      {/* Row 2: Title */}
      <p className="mt-1 text-sm font-medium leading-snug line-clamp-2">
        {issue.title}
      </p>

      {/* Description */}
      {showDescription && (
        <p className="mt-1 text-xs text-muted-foreground line-clamp-1">
          {issue.description}
        </p>
      )}

      {/* Labels */}
      {storeProperties.labels && issue.labels.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {issue.labels.slice(0, 4).map((l) => (
            <span
              key={l.id}
              className="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px] border truncate max-w-[100px]"
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
          {issue.labels.length > 4 && (
            <span className="text-[10px] text-muted-foreground py-0.5">
              +{issue.labels.length - 4}
            </span>
          )}
        </div>
      )}

      {/* Row 3: Assignee, priority badge, due date */}
      {(showAssignee || showPriority || showDueDate) && (
        <div className="mt-3 flex items-center gap-2">
          {showAssignee && issue.assignee_type === "agent" ? (
            editable ? (
              <PickerWrapper>
                <AssigneePicker
                  assigneeType={issue.assignee_type}
                  assigneeId={issue.assignee_id}
                  onUpdate={handleUpdate}
                  trigger={<AgentDispatchBadge issue={issue} />}
                />
              </PickerWrapper>
            ) : (
              <AgentDispatchBadge issue={issue} />
            )
          ) : showAssignee ? (
            editable ? (
              <PickerWrapper>
                <AssigneePicker
                  assigneeType={issue.assignee_type}
                  assigneeId={issue.assignee_id}
                  onUpdate={handleUpdate}
                  trigger={
                    <ActorAvatar
                      actorType={issue.assignee_type!}
                      actorId={issue.assignee_id!}
                      size={22}
                    />
                  }
                />
              </PickerWrapper>
            ) : (
              <ActorAvatar
                actorType={issue.assignee_type!}
                actorId={issue.assignee_id!}
                size={22}
              />
            )
          ) : null}
          {showPriority &&
            (editable ? (
              <PickerWrapper>
                <PriorityPicker
                  priority={issue.priority}
                  onUpdate={handleUpdate}
                  trigger={
                    <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${priorityCfg.badgeBg} ${priorityCfg.badgeText}`}>
                      <PriorityIcon priority={issue.priority} className="h-3 w-3" inheritColor />
                      {priorityCfg.label}
                    </span>
                  }
                />
              </PickerWrapper>
            ) : (
              <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${priorityCfg.badgeBg} ${priorityCfg.badgeText}`}>
                <PriorityIcon priority={issue.priority} className="h-3 w-3" inheritColor />
                {priorityCfg.label}
              </span>
            ))}
          {showDueDate && (
            <div className="ml-auto">
              {editable ? (
                <PickerWrapper>
                  <DueDatePicker
                    dueDate={issue.due_date}
                    onUpdate={handleUpdate}
                    trigger={
                      <span
                        className={`flex items-center gap-1 text-xs ${
                          new Date(issue.due_date!) < new Date()
                            ? "text-destructive"
                            : "text-muted-foreground"
                        }`}
                      >
                        <CalendarDays className="size-3" />
                        <AbsoluteTime value={issue.due_date} style="shortDate" />
                      </span>
                    }
                  />
                </PickerWrapper>
              ) : (
                <span
                  className={`flex items-center gap-1 text-xs ${
                    new Date(issue.due_date!) < new Date()
                      ? "text-destructive"
                      : "text-muted-foreground"
                  }`}
                >
                  <CalendarDays className="size-3" />
                  <AbsoluteTime value={issue.due_date} style="shortDate" />
                </span>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
});

export const DraggableBoardCard = memo(function DraggableBoardCard({ issue }: { issue: Issue }) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: issue.id,
    data: { status: issue.status },
  });
  const workspaceSlug = useWorkspaceStore((s) => s.workspace?.slug ?? "");

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      className={isDragging ? "opacity-30" : ""}
    >
      <Link
        href={issueUrl(issue.id, workspaceSlug)}
        className={`group block transition-colors ${isDragging ? "pointer-events-none" : ""}`}
      >
        <BoardCardContent issue={issue} editable />
      </Link>
    </div>
  );
});
