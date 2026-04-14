"use client";

import { Bot, Monitor } from "lucide-react";
import { useWorkspaceStore } from "@/features/workspace";
import { useRuntimeStore } from "@/features/runtimes";
import type { Issue } from "@/shared/types";

interface AgentDispatchBadgeProps {
  issue: Issue;
  layout?: "stack" | "inline";
}

export function AgentDispatchBadge({ issue, layout = "stack" }: AgentDispatchBadgeProps) {
  const agents = useWorkspaceStore((s) => s.agents);
  const daemons = useRuntimeStore((s) => s.daemons);

  const agent = issue.assignee_id
    ? agents.find((a) => a.id === issue.assignee_id)
    : null;

  const daemon = issue.dispatch_daemon_id
    ? daemons.find((d) => d.id === issue.dispatch_daemon_id)
    : null;

  const agentName = agent?.name ?? "Unknown Agent";
  const provider = issue.dispatch_provider;
  const daemonName = daemon?.device_name || daemon?.daemon_id || null;

  if (layout === "inline") {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs min-w-0">
        <Bot className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <span className="truncate font-medium">{agentName}</span>
        {(provider || daemonName) && (
          <span className="text-muted-foreground shrink-0">·</span>
        )}
        {provider && (
          <span className="capitalize text-muted-foreground shrink-0">{provider}</span>
        )}
        {provider && daemonName && (
          <span className="text-muted-foreground shrink-0">·</span>
        )}
        {daemonName && (
          <span className="inline-flex items-center gap-0.5 text-muted-foreground truncate">
            <Monitor className="h-3 w-3 shrink-0" />
            <span className="truncate">{daemonName}</span>
          </span>
        )}
      </span>
    );
  }

  return (
    <div className="flex items-start gap-1.5 min-w-0">
      <Bot className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
      <div className="min-w-0 flex flex-col">
        <span className="text-xs font-medium truncate">{agentName}</span>
        {(provider || daemonName) && (
          <span className="text-[10px] text-muted-foreground truncate leading-tight">
            {provider && <span className="capitalize">{provider}</span>}
            {provider && daemonName && " · "}
            {daemonName && daemonName}
          </span>
        )}
      </div>
    </div>
  );
}
