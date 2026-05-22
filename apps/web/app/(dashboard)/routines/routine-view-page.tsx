"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Pencil, Sparkles } from "lucide-react";
import { api } from "@/shared/api";
import type { Routine, RoutineRun } from "@/shared/types";
import { toast } from "sonner";
export function RoutineViewPage({ routineID }: { routineID: string }) {
  const [routine, setRoutine] = useState<Routine | null>(null);
  const [runs, setRuns] = useState<RoutineRun[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    Promise.all([api.getRoutine(routineID), api.listRoutineRuns(routineID)])
      .then(([routineResult, runResult]) => {
        if (cancelled) return;
        setRoutine(routineResult);
        setRuns(runResult);
      })
      .catch((error) => {
        if (!cancelled) toast.error(error instanceof Error ? error.message : "Failed to load routine");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [routineID]);

  return (
    <main className="h-full min-h-0 overflow-y-auto bg-background">
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-5 px-6 py-5">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Sparkles className="size-4" />
              <a href="/routines" className="hover:text-foreground">Routines</a>
              <span>/</span>
              <span>{routine?.name ?? "Routine"}</span>
            </div>
            <h1 className="text-2xl font-semibold tracking-tight">{routine?.name ?? "Routine"}</h1>
          </div>
          {routine && (
            <a href={`/routines?new=1&id=${routine.id}`} className={buttonVariants({ variant: "outline" })}>
              <Pencil className="size-4" />
              Edit routine
            </a>
          )}
        </div>

        {loading ? (
          <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
            Loading routine...
          </div>
        ) : !routine ? (
          <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
            Routine not found
          </div>
        ) : (
          <>
            <section className="grid gap-3 rounded-xl border bg-muted/20 p-4 md:grid-cols-3">
              <OverviewItem label="Status" value={routine.enabled ? "Enabled" : "Paused"} />
              <OverviewItem label="Priority" value={routine.priority} />
              <OverviewItem label="Triggers" value={`${routine.triggers.length}`} />
              <OverviewItem label="Assignee" value={routine.assignee_id ? `${routine.assignee_type}:${routine.assignee_id}` : "Unassigned"} />
              <OverviewItem label="Subscribers" value={`${routine.subscriber_ids.length}`} />
              <OverviewItem label="Labels" value={`${routine.label_ids.length}`} />
            </section>

            <section className="space-y-3">
              <div>
                <h2 className="text-sm font-semibold">Triggered issues</h2>
                <p className="text-xs text-muted-foreground">
                  Issues created by this routine, ordered by trigger time.
                </p>
              </div>
              {runs.filter((run) => run.issue).length === 0 ? (
                <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
                  No issues triggered yet
                </div>
              ) : (
                <div className="space-y-2">
                  {runs.filter((run) => run.issue).map((run) => (
                    <a
                      key={run.id}
                      href={`/issues/${run.issue!.id}`}
                      className="flex items-center justify-between rounded-xl border px-4 py-3 transition-colors hover:bg-accent/50"
                    >
                      <span className="min-w-0">
                        <span className="block truncate text-sm font-medium">{run.issue!.title}</span>
                        <span className="block truncate text-xs text-muted-foreground">
                          {run.issue!.identifier} · {run.event_type || "routine"} · {formatDateTime(run.created_at)}
                        </span>
                      </span>
                      <Badge variant="secondary">{run.issue!.status}</Badge>
                    </a>
                  ))}
                </div>
              )}
            </section>
          </>
        )}
      </div>
    </main>
  );
}

function OverviewItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-sm font-medium">{value}</div>
    </div>
  );
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


