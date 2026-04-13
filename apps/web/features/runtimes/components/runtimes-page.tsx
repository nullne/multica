"use client";

import { useEffect, useCallback } from "react";
import { Server, ChevronLeft } from "lucide-react";
import { useDefaultLayout } from "react-resizable-panels";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@/components/ui/resizable";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useIsMobile } from "@/hooks/use-mobile";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { useWSEvent } from "@/features/realtime";
import { useRuntimeStore } from "../store";
import { RuntimeList } from "./runtime-list";
import { DaemonDetail } from "./daemon-detail";

export default function RuntimesPage() {
  const isMobile = useIsMobile();
  const isLoading = useAuthStore((s) => s.isLoading);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const daemons = useRuntimeStore((s) => s.daemons);
  const runtimes = useRuntimeStore((s) => s.runtimes);
  const selectedDaemonId = useRuntimeStore((s) => s.selectedDaemonId);
  const fetching = useRuntimeStore((s) => s.fetching);
  const fetchAll = useRuntimeStore((s) => s.fetchAll);
  const setSelectedDaemonId = useRuntimeStore((s) => s.setSelectedDaemonId);

  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "multica_runtimes_layout",
  });

  useEffect(() => {
    if (workspace) fetchAll();
  }, [workspace, fetchAll]);

  const handleDaemonEvent = useCallback(() => {
    fetchAll();
  }, [fetchAll]);

  useWSEvent("daemon:register", handleDaemonEvent);

  const selected = daemons.find((d) => d.id === selectedDaemonId) ?? null;

  if (isLoading || fetching) {
    return (
      <div className="flex flex-1 min-h-0">
        <div className="w-72 border-r">
          <div className="flex h-12 items-center justify-between border-b px-4">
            <Skeleton className="h-4 w-20" />
          </div>
          <div className="divide-y">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 px-4 py-3">
                <Skeleton className="h-8 w-8 rounded-lg" />
                <div className="flex-1 space-y-1.5">
                  <Skeleton className="h-4 w-28" />
                  <Skeleton className="h-3 w-20" />
                </div>
              </div>
            ))}
          </div>
        </div>
        <div className="flex-1 p-6 space-y-6">
          <div className="flex items-center gap-3">
            <Skeleton className="h-7 w-7 rounded-md" />
            <Skeleton className="h-5 w-32" />
          </div>
          <div className="space-y-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-lg" />
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (isMobile) {
    if (selected) {
      return (
        <div className="flex flex-1 min-h-0 flex-col">
          <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
            <Button variant="ghost" size="icon-xs" onClick={() => setSelectedDaemonId("")}>
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <span className="text-sm font-medium truncate">Daemons</span>
          </div>
          <div className="flex-1 min-h-0 overflow-y-auto">
            <DaemonDetail key={selected.id} daemon={selected} runtimes={runtimes} />
          </div>
        </div>
      );
    }
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <RuntimeList
          daemons={daemons}
          selectedId={selectedDaemonId}
          onSelect={setSelectedDaemonId}
        />
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
      <ResizablePanel
        id="list"
        defaultSize={280}
        minSize={240}
        maxSize={400}
        groupResizeBehavior="preserve-pixel-size"
      >
        <RuntimeList
          daemons={daemons}
          selectedId={selectedDaemonId}
          onSelect={setSelectedDaemonId}
        />
      </ResizablePanel>

      <ResizableHandle />

      <ResizablePanel id="detail" minSize="50%">
        {selected ? (
          <DaemonDetail key={selected.id} daemon={selected} runtimes={runtimes} />
        ) : (
          <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
            <Server className="h-10 w-10 text-muted-foreground/30" />
            <p className="mt-3 text-sm">Select a daemon to view details</p>
          </div>
        )}
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
