"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { StatusIcon, RunningIndicatorRing } from "@/features/issues/components";
import { issueUrl } from "@/features/issues/utils/url";
import { useWorkspaceStore } from "@/features/workspace";
import { Loader2 } from "lucide-react";
import type { IssueStatus, RoutineRun, RoutineRunSource } from "@/shared/types";

export type RoutineRunFilter = "all" | RoutineRunSource;

const RUN_FILTERS: { value: RoutineRunFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "scheduled", label: "Scheduled" },
  { value: "manual", label: "Manual" },
  { value: "api", label: "API" },
  { value: "webhook", label: "Webhook" },
];

const RUN_STATUS_TO_ISSUE_STATUS: Record<RoutineRun["status"], IssueStatus> = {
  processed: "done",
  filtered: "cancelled",
  deduped: "todo",
  error: "blocked",
};

export function RoutineRunList({
  runs,
  filter,
  onFilterChange,
  hasMore,
  loading,
  loadingMore,
  onLoadMore,
}: {
  runs: RoutineRun[];
  filter: RoutineRunFilter;
  onFilterChange: (filter: RoutineRunFilter) => void;
  hasMore: boolean;
  loading: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
}) {
  return (
    <section className="space-y-3">
      <div>
        <h2 className="text-sm font-semibold">Runs</h2>
        <p className="text-xs text-muted-foreground">
          Recent routine runs, including scheduled, manual, API, and webhook triggers.
        </p>
      </div>
      <div className="inline-flex rounded-xl bg-muted p-1">
        {RUN_FILTERS.map((option) => (
          <button
            key={option.value}
            type="button"
            aria-pressed={filter === option.value}
            onClick={() => onFilterChange(option.value)}
            className="rounded-lg px-2.5 py-1 text-sm text-muted-foreground transition-colors hover:text-foreground aria-pressed:bg-background aria-pressed:text-foreground aria-pressed:shadow-sm"
          >
            {option.label}
          </button>
        ))}
      </div>
      {loading ? (
        <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
          Loading runs...
        </div>
      ) : runs.length === 0 ? (
        <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
          No runs found
        </div>
      ) : (
        <div className="space-y-2">
          {runs.map((run) => (
            <RoutineRunRow key={run.id} run={run} />
          ))}
        </div>
      )}
      {hasMore && (
        <div className="flex justify-center">
          <Button type="button" variant="outline" onClick={onLoadMore} disabled={loadingMore}>
            {loadingMore && <Loader2 className="size-4 animate-spin" />}
            Load more
          </Button>
        </div>
      )}
    </section>
  );
}

function RoutineRunRow({ run }: { run: RoutineRun }) {
  const source = getRoutineRunSource(run);
  const workspaceSlug = useWorkspaceStore((s) => s.workspace?.slug ?? "");
  const content = (
    <>
      <span className="flex min-w-0 items-center gap-3">
        <RoutineRunStatusIcon run={run} />
        <span className="min-w-0">
          <span className="block truncate text-sm font-medium">
            {run.issue?.title ?? formatDateTime(run.created_at)}
          </span>
          <span className="block truncate text-xs text-muted-foreground">
            {formatRunSubtitle(run)}
          </span>
        </span>
      </span>
      <Tooltip>
        <TooltipTrigger
          render={
            <span className="shrink-0 cursor-default">
              <Badge variant="secondary" className="uppercase tracking-wide">
                {source.label}
              </Badge>
            </span>
          }
        />
        <TooltipContent side="left" align="end" className="max-w-md items-start bg-popover p-0 text-popover-foreground shadow-lg">
          <div className="space-y-2 p-3">
            <div>
              <div className="text-xs font-semibold">{source.tooltipTitle}</div>
              <div className="text-[11px] text-muted-foreground">{run.event_type || "routine"}</div>
            </div>
            <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-2 text-[11px] leading-relaxed text-muted-foreground">
              {formatRunPayload(run.payload)}
            </pre>
          </div>
        </TooltipContent>
      </Tooltip>
    </>
  );

  if (run.issue) {
    return (
      <a
        href={issueUrl(run.issue.id, workspaceSlug)}
        className="flex items-center justify-between gap-3 rounded-xl bg-muted/40 px-4 py-3 transition-colors hover:bg-accent/60"
      >
        {content}
      </a>
    );
  }

  return (
    <div className="flex items-center justify-between gap-3 rounded-xl bg-muted/40 px-4 py-3">
      {content}
    </div>
  );
}

function RoutineRunStatusIcon({ run }: { run: RoutineRun }) {
  const icon = (
    <StatusIcon
      status={RUN_STATUS_TO_ISSUE_STATUS[run.status]}
      className="size-4"
    />
  );
  if (!run.issue_id) return icon;
  return (
    <RunningIndicatorRing issueId={run.issue_id}>
      {icon}
    </RunningIndicatorRing>
  );
}

function getRoutineRunSource(run: RoutineRun): { filter: Exclude<RoutineRunFilter, "all">; label: string; tooltipTitle: string } {
  if (run.event_type === "schedule") return { filter: "scheduled", label: "Scheduled", tooltipTitle: "Scheduled run" };
  if (run.event_type === "manual") return { filter: "manual", label: "Manual", tooltipTitle: "Manual run" };
  if (run.event_type.startsWith("github.") || run.event_type.startsWith("alert.")) {
    return { filter: "webhook", label: "Webhook", tooltipTitle: run.event_type.startsWith("github.") ? "GitHub webhook event" : "Webhook event" };
  }
  return { filter: "api", label: "API", tooltipTitle: "API call payload" };
}

function formatRunSubtitle(run: RoutineRun): string {
  const parts = run.issue ? [run.issue.identifier, formatDateTime(run.created_at)] : [run.event_type || "routine"];
  const eventLabel = formatWebhookEventLabel(run.event_type);
  if (eventLabel) parts.push(eventLabel);
  return parts.join(" · ");
}

function formatWebhookEventLabel(eventType: string): string | null {
  if (eventType.startsWith("github.")) {
    return eventType.slice("github.".length);
  }
  if (eventType.startsWith("alert.")) {
    return eventType;
  }
  return null;
}

function formatRunPayload(payload: unknown): string {
  if (payload == null || (typeof payload === "object" && Object.keys(payload).length === 0)) {
    return "No payload captured";
  }
  if (typeof payload === "string") return payload;
  try {
    return JSON.stringify(payload, null, 2);
  } catch {
    return String(payload);
  }
}

function formatDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
