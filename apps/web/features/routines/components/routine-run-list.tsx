"use client";

import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { StatusIcon, RunningIndicatorRing } from "@/features/issues/components";
import { issueUrl } from "@/features/issues/utils/url";
import { useWorkspaceStore } from "@/features/workspace";
import type { IssueStatus, RoutineRun } from "@/shared/types";

type RoutineRunFilter = "all" | "scheduled" | "manual" | "api" | "webhook";

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

export function RoutineRunList({ runs }: { runs: RoutineRun[] }) {
  const [filter, setFilter] = useState<RoutineRunFilter>("all");
  const visibleRuns = useMemo(
    () => runs.filter((run) => filter === "all" || getRoutineRunSource(run).filter === filter),
    [filter, runs],
  );

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
            onClick={() => setFilter(option.value)}
            className="rounded-lg px-2.5 py-1 text-sm text-muted-foreground transition-colors hover:text-foreground aria-pressed:bg-background aria-pressed:text-foreground aria-pressed:shadow-sm"
          >
            {option.label}
          </button>
        ))}
      </div>
      {visibleRuns.length === 0 ? (
        <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
          No runs found
        </div>
      ) : (
        <div className="space-y-2">
          {visibleRuns.map((run) => (
            <RoutineRunRow key={run.id} run={run} />
          ))}
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
            {run.issue
              ? `${run.issue.identifier} · ${formatDateTime(run.created_at)}`
              : run.event_type || "routine"}
          </span>
        </span>
      </span>
      <Badge variant="secondary" className="shrink-0 uppercase tracking-wide">
        {source.label}
      </Badge>
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

function getRoutineRunSource(run: RoutineRun): { filter: Exclude<RoutineRunFilter, "all">; label: string } {
  if (run.event_type === "schedule") return { filter: "scheduled", label: "Scheduled" };
  if (run.event_type === "manual") return { filter: "manual", label: "Manual" };
  if (run.event_type.startsWith("github.") || run.event_type.startsWith("alert.")) {
    return { filter: "webhook", label: "Webhook" };
  }
  return { filter: "api", label: "API" };
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
