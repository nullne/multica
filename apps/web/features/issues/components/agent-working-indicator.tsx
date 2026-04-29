"use client";

import { Loader2 } from "lucide-react";
import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";

interface AgentWorkingIndicatorProps {
  issueId: string;
  layout?: "card" | "inline";
}

/**
 * Shows an animated working badge when an agent task is active for the issue.
 * Reads from the page-level activeTaskStore (no per-card API calls).
 */
export function AgentWorkingIndicator({ issueId, layout = "card" }: AgentWorkingIndicatorProps) {
  const task = useActiveTaskStore((s) => s.tasks.get(issueId));

  if (!task) return null;

  if (layout === "inline") {
    return (
      <span
        className="inline-flex items-center gap-1 rounded-full bg-info/10 px-1.5 py-0.5 text-[10px] font-medium text-info shrink-0"
        title="Agent is working"
      >
        <Loader2 className="h-2.5 w-2.5 animate-spin" />
        Working
      </span>
    );
  }

  return (
    <div className="mt-2 flex items-center gap-1.5 rounded-md bg-info/8 px-2 py-1">
      <Loader2 className="h-3 w-3 animate-spin text-info shrink-0" />
      <span className="text-[11px] font-medium text-info">Agent working</span>
    </div>
  );
}
