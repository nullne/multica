"use client";

import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useSearchParams } from "next/navigation";
import { useDefaultLayout } from "react-resizable-panels";
import { ChevronRight, Plus, PanelLeft } from "lucide-react";
import { Accordion } from "@base-ui/react/accordion";
import type { Issue, IssueStatus } from "@/shared/types";
import { STATUS_CONFIG } from "@/features/issues/config";
import { useViewStore } from "@/features/issues/stores/view-store-context";
import { useModalStore } from "@/features/modals";
import { sortIssues } from "@/features/issues/utils/sort";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@/components/ui/resizable";
import { IssueDetail } from "./issue-detail";
import { PriorityIcon } from "./priority-icon";
import { StatusIcon } from "./status-icon";
import { AgentDispatchBadge } from "./agent-dispatch-badge";

// ---------------------------------------------------------------------------
// SplitListRow
// ---------------------------------------------------------------------------

function SplitListRow({
  issue,
  isSelected,
  onClick,
}: {
  issue: Issue;
  isSelected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`group/row flex h-9 w-full items-center gap-2 px-2 md:px-4 text-sm transition-colors hover:bg-accent/50 ${
        isSelected ? "bg-accent" : ""
      }`}
    >
      <PriorityIcon priority={issue.priority} className="shrink-0" />
      <span className="hidden md:inline w-16 shrink-0 text-xs text-muted-foreground text-left">
        {issue.identifier}
      </span>
      <span className="min-w-0 flex-1 truncate text-left">{issue.title}</span>
      {issue.due_date && (
        <span className="hidden sm:inline shrink-0 text-xs text-muted-foreground">
          {new Date(issue.due_date).toLocaleDateString("en-US", {
            month: "short",
            day: "numeric",
          })}
        </span>
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
    </button>
  );
}

// ---------------------------------------------------------------------------
// SplitView
// ---------------------------------------------------------------------------

export function SplitView({
  issues,
  visibleStatuses,
}: {
  issues: Issue[];
  visibleStatuses: IssueStatus[];
}) {
  const searchParams = useSearchParams();
  const urlIssue = searchParams.get("issue") ?? "";

  const [selectedIssueId, setSelectedIssueIdState] = useState(() => urlIssue);

  useEffect(() => {
    setSelectedIssueIdState(urlIssue);
  }, [urlIssue]);

  const setSelectedIssueId = useCallback((id: string) => {
    setSelectedIssueIdState(id);
    const url = id
      ? `${window.location.pathname}?issue=${id}`
      : window.location.pathname;
    window.history.replaceState(null, "", url);
  }, []);

  const sortBy = useViewStore((s) => s.sortBy);
  const sortDirection = useViewStore((s) => s.sortDirection);

  const issuesByStatus = useMemo(() => {
    const map = new Map<IssueStatus, Issue[]>();
    for (const status of visibleStatuses) {
      const filtered = issues.filter((i) => i.status === status);
      map.set(status, sortIssues(filtered, sortBy, sortDirection));
    }
    return map;
  }, [issues, visibleStatuses, sortBy, sortDirection]);

  const [expandedStatuses, setExpandedStatuses] = useState<string[]>(
    () => [...visibleStatuses],
  );
  const prevVisibleRef = useRef(visibleStatuses);
  useEffect(() => {
    const prev = prevVisibleRef.current;
    prevVisibleRef.current = visibleStatuses;
    const newStatuses = visibleStatuses.filter((s) => !prev.includes(s));
    if (newStatuses.length > 0) {
      setExpandedStatuses((e) => [...e, ...newStatuses]);
    }
  }, [visibleStatuses]);

  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "multica_issues_split_layout",
  });

  const listPane = (
    <div className="flex flex-col border-r h-full overflow-hidden">
      <div className="flex-1 min-h-0 overflow-y-auto p-2">
        <Accordion.Root
          multiple
          className="space-y-1"
          value={expandedStatuses}
          onValueChange={(value: string[]) => setExpandedStatuses(value)}
        >
          {visibleStatuses.map((status) => {
            const cfg = STATUS_CONFIG[status];
            const statusIssues = issuesByStatus.get(status) ?? [];

            return (
              <Accordion.Item key={status} value={status}>
                <Accordion.Header className="group/header flex h-10 items-center rounded-lg bg-muted/40 transition-colors hover:bg-accent/30">
                  <Accordion.Trigger className="group/trigger flex flex-1 items-center gap-2 px-2 h-full text-left outline-none">
                    <ChevronRight className="size-3.5 shrink-0 text-muted-foreground transition-transform group-aria-expanded/trigger:rotate-90" />
                    <span
                      className={`inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-xs font-semibold ${cfg.badgeBg} ${cfg.badgeText}`}
                    >
                      <StatusIcon status={status} className="h-3 w-3" inheritColor />
                      {cfg.label}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {statusIssues.length}
                    </span>
                  </Accordion.Trigger>
                  <div className="pr-2">
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            className="rounded-full text-muted-foreground opacity-0 group-hover/header:opacity-100 transition-opacity"
                            onClick={() =>
                              useModalStore
                                .getState()
                                .open("create-issue", { status })
                            }
                          />
                        }
                      >
                        <Plus className="size-3.5" />
                      </TooltipTrigger>
                      <TooltipContent>Add issue</TooltipContent>
                    </Tooltip>
                  </div>
                </Accordion.Header>
                <Accordion.Panel className="pt-1">
                  {statusIssues.length > 0 ? (
                    statusIssues.map((issue) => (
                      <SplitListRow
                        key={issue.id}
                        issue={issue}
                        isSelected={issue.id === selectedIssueId}
                        onClick={() => setSelectedIssueId(issue.id)}
                      />
                    ))
                  ) : (
                    <p className="py-6 text-center text-xs text-muted-foreground">
                      No issues
                    </p>
                  )}
                </Accordion.Panel>
              </Accordion.Item>
            );
          })}
        </Accordion.Root>
      </div>
    </div>
  );

  const detailPane = (
    <div className="flex flex-col min-h-0 h-full">
      {selectedIssueId ? (
        <IssueDetail
          key={selectedIssueId}
          issueId={selectedIssueId}
          defaultSidebarOpen={false}
          layoutId="multica_issues_split_issue_detail_layout"
          onBack={() => setSelectedIssueId("")}
        />
      ) : (
        <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
          <PanelLeft className="mb-3 h-10 w-10 text-muted-foreground/30" />
          <p className="text-sm">Select an issue to view details</p>
        </div>
      )}
    </div>
  );

  return (
    <ResizablePanelGroup
      orientation="horizontal"
      className="flex-1 min-h-0"
      defaultLayout={defaultLayout}
      onLayoutChanged={onLayoutChanged}
    >
      <ResizablePanel
        id="list"
        defaultSize={320}
        minSize={240}
        maxSize={480}
        groupResizeBehavior="preserve-pixel-size"
      >
        {listPane}
      </ResizablePanel>
      <ResizableHandle />
      <ResizablePanel id="detail" minSize="40%">
        {detailPane}
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
