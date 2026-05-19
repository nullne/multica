"use client";

import { useState, useEffect, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import { useDefaultLayout } from "react-resizable-panels";
import { ChevronLeft, Columns2 } from "lucide-react";
import type { Issue } from "@/shared/types";
import { Button } from "@/components/ui/button";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@/components/ui/resizable";
import { useIsMobile } from "@/hooks/use-mobile";
import { IssueDetail } from "./issue-detail";
import { StatusIcon } from "./status-icon";
import { PriorityIcon } from "./priority-icon";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { RunningIndicatorRing } from "./running-indicator-ring";

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
      aria-current={isSelected ? "true" : undefined}
      data-active={isSelected || undefined}
      className={`group relative flex w-full items-center gap-2 px-4 py-2 text-left text-sm transition-colors ${
        isSelected
          ? "bg-accent text-accent-foreground before:absolute before:left-0 before:top-1 before:bottom-1 before:w-[3px] before:rounded-r before:bg-primary"
          : "hover:bg-accent/50"
      }`}
    >
      <RunningIndicatorRing issueId={issue.id} className="size-3.5">
        <PriorityIcon priority={issue.priority} className="size-3.5 text-muted-foreground" />
      </RunningIndicatorRing>
      <span className="hidden md:inline w-16 shrink-0 text-xs text-muted-foreground">
        {issue.identifier}
      </span>
      <span className="min-w-0 flex-1 truncate">{issue.title}</span>
      <StatusIcon status={issue.status} className="size-3.5 shrink-0 text-muted-foreground" />
      {issue.assignee_type && issue.assignee_id && (
        <ActorAvatar actorType={issue.assignee_type} actorId={issue.assignee_id} size={18} />
      )}
    </button>
  );
}

// ---------------------------------------------------------------------------
// SplitView
// ---------------------------------------------------------------------------

export function SplitView({ issues }: { issues: Issue[] }) {
  const isMobile = useIsMobile();
  const searchParams = useSearchParams();
  const urlIssue = searchParams.get("issue") ?? "";

  const [selectedId, setSelectedIdState] = useState(() => urlIssue);

  useEffect(() => {
    setSelectedIdState(urlIssue);
  }, [urlIssue]);

  const setSelectedId = useCallback((id: string) => {
    setSelectedIdState(id);
    const params = new URLSearchParams(window.location.search);
    if (id) {
      params.set("issue", id);
    } else {
      params.delete("issue");
    }
    const search = params.toString();
    window.history.replaceState(null, "", search ? `?${search}` : window.location.pathname);
  }, []);

  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "multica_issues_split_layout",
  });

  const listPane = (
    <div className="flex flex-col border-r h-full">
      <div className="flex-1 min-h-0 overflow-y-auto">
        {issues.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
            <p className="text-sm">No issues</p>
          </div>
        ) : (
          <div>
            {issues.map((issue) => (
              <SplitListRow
                key={issue.id}
                issue={issue}
                isSelected={issue.id === selectedId}
                onClick={() => setSelectedId(issue.id)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );

  const detailPane = (
    <div className="flex flex-col min-h-0 h-full">
      {selectedId ? (
        <IssueDetail
          key={selectedId}
          issueId={selectedId}
          defaultSidebarOpen={false}
          layoutId="multica_issues_split_detail_layout"
          onBack={isMobile ? () => setSelectedId("") : undefined}
        />
      ) : (
        <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
          <Columns2 className="mb-3 h-10 w-10 text-muted-foreground/30" />
          <p className="text-sm">Select an issue to view details</p>
        </div>
      )}
    </div>
  );

  if (isMobile) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        {selectedId ? (
          <>
            <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
              <Button variant="ghost" size="icon-xs" onClick={() => setSelectedId("")}>
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <span className="text-sm font-medium truncate">Issues</span>
            </div>
            <div className="flex-1 min-h-0 overflow-hidden">
              {detailPane}
            </div>
          </>
        ) : (
          listPane
        )}
      </div>
    );
  }

  return (
    <ResizablePanelGroup
      orientation="horizontal"
      className="flex-1 min-h-0"
      defaultLayout={defaultLayout}
      onLayoutChanged={onLayoutChanged}
    >
      <ResizablePanel id="list" defaultSize={320} minSize={240} maxSize={480} groupResizeBehavior="preserve-pixel-size">
        {listPane}
      </ResizablePanel>
      <ResizableHandle />
      <ResizablePanel id="detail" minSize="40%">
        {detailPane}
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
