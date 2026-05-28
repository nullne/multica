"use client";

import { type ReactNode, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { ActorAvatar } from "@/components/common/actor-avatar";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { RoutineRunList } from "@/features/routines";
import { useAuthStore } from "@/features/auth";
import { useActorName, useWorkspaceStore } from "@/features/workspace";
import { MoreHorizontal, Pencil, Play, Sparkles, Trash2 } from "lucide-react";
import { api } from "@/shared/api";
import type { Routine, RoutineRun } from "@/shared/types";
import { toast } from "sonner";
export function RoutineViewPage({ routineID }: { routineID: string }) {
  const router = useRouter();
  const currentUser = useAuthStore((s) => s.user);
  const { getActorName } = useActorName();
  const members = useWorkspaceStore((s) => s.members);
  const [routine, setRoutine] = useState<Routine | null>(null);
  const [runs, setRuns] = useState<RoutineRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [triggering, setTriggering] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [updatingStatus, setUpdatingStatus] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const currentMember = members.find((member) => member.user_id === currentUser?.id);
  const canManageRoutines = currentMember?.role === "owner" || currentMember?.role === "admin";

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

  async function handleTrigger() {
    if (!routine) return;
    setTriggering(true);
    try {
      await api.triggerRoutine(routine.id);
      setRuns(await api.listRoutineRuns(routine.id));
      toast.success("Routine triggered");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to trigger routine");
    } finally {
      setTriggering(false);
    }
  }

  async function handleDelete() {
    if (!routine) return;
    setDeleting(true);
    try {
      await api.deleteRoutine(routine.id);
      toast.success("Routine deleted");
      router.push("/routines");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to delete routine");
      setDeleting(false);
    }
  }

  async function handleToggleStatus() {
    if (!routine) return;
    const nextEnabled = !routine.enabled;
    setUpdatingStatus(true);
    try {
      await api.updateRoutine(routine.id, routineToUpdatePayload(routine, nextEnabled));
      setRoutine({ ...routine, enabled: nextEnabled });
      toast.success(nextEnabled ? "Routine enabled" : "Routine paused");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update routine");
    } finally {
      setUpdatingStatus(false);
    }
  }

  return (
    <>
      <main className="h-full min-h-0 overflow-y-auto bg-background">
        <div className="mx-auto flex w-full max-w-5xl flex-col gap-5 px-6 py-5">
          <div className="flex items-start justify-between gap-4 md:items-center">
            <div className="space-y-1">
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Sparkles className="size-4" />
                <a href="/routines" className="hover:text-foreground">Routines</a>
                <span>/</span>
                <span>{routine?.name ?? "Routine"}</span>
              </div>
            </div>
            {routine && canManageRoutines && (
              <div className="flex flex-wrap items-center justify-end gap-2">
                <Button variant="outline" onClick={handleTrigger} disabled={!routine.enabled || triggering || deleting}>
                  <Play className="size-4" />
                  {triggering ? "Running..." : "Run now"}
                </Button>
                <a href={`/routines?new=1&id=${routine.id}`} className={buttonVariants({ variant: "outline" })}>
                  <Pencil className="size-4" />
                  Edit routine
                </a>
                <DropdownMenu>
                  <DropdownMenuTrigger
                    render={
                      <Button
                        variant="outline"
                        size="icon"
                        aria-label="More actions"
                        disabled={triggering || deleting}
                      />
                    }
                  >
                    <MoreHorizontal className="size-4" />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem
                      variant="destructive"
                      onClick={() => setDeleteOpen(true)}
                    >
                      <Trash2 className="size-4" />
                      Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
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
              {!canManageRoutines && (
                <div className="rounded-xl border bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
                  <span className="font-medium text-foreground">Read-only</span>
                  {" · "}
                  Ask an owner or admin to change or run this routine.
                </div>
              )}
              <section className="grid gap-3 rounded-xl border bg-muted/20 p-4 md:grid-cols-3">
                <OverviewItem
                  label="Status"
                  value={
                    <span className="flex items-center gap-2">
                      <span>{routine.enabled ? "Enabled" : "Paused"}</span>
                      {canManageRoutines && (
                        <Button
                          type="button"
                          variant="outline"
                          size="xs"
                          onClick={handleToggleStatus}
                          disabled={updatingStatus}
                        >
                          {routine.enabled ? "Pause routine" : "Enable routine"}
                        </Button>
                      )}
                    </span>
                  }
                />
                <OverviewItem label="Priority" value={formatLabel(routine.priority)} />
                <OverviewItem
                  label="Assignee"
                  value={routine.assignee_type && routine.assignee_id ? (
                    <span className="flex min-w-0 items-center gap-1.5">
                      <ActorAvatar actorType={routine.assignee_type} actorId={routine.assignee_id} size={16} />
                      <span className="truncate">{getActorName(routine.assignee_type, routine.assignee_id)}</span>
                    </span>
                  ) : "Unassigned"}
                />
                <OverviewItem
                  label="Subscribers"
                  value={summarizeSelectedMembers(routine.subscriber_ids, members, "No subscribers")}
                />
                <OverviewItem label="Labels" value={`${routine.label_ids.length}`} />
              </section>

              {routine.instructions && (
                <section className="space-y-2 rounded-xl border bg-muted/20 p-4">
                  <h2 className="text-sm font-semibold">Instructions</h2>
                  <p className="whitespace-pre-wrap text-sm text-muted-foreground">{routine.instructions}</p>
                </section>
              )}

              <section className="space-y-3">
                <div>
                  <h2 className="text-sm font-semibold">Triggers</h2>
                  <p className="text-xs text-muted-foreground">
                    Sources that can start this routine.
                  </p>
                </div>
                <div className="space-y-2">
                  {routine.triggers.map((trigger) => (
                    <div key={trigger.id} className="rounded-xl border bg-muted/20 px-4 py-3">
                      <div className="text-sm font-medium">{formatTriggerTitle(trigger.trigger_type)}</div>
                      <div className="mt-1 text-xs text-muted-foreground">{formatTriggerDetail(trigger)}</div>
                    </div>
                  ))}
                </div>
              </section>

              <RoutineRunList runs={runs} />
            </>
          )}
        </div>
      </main>
      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete routine</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete {routine?.name ?? "this routine"}. Existing issues created by the routine are kept.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              disabled={deleting}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete routine
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function OverviewItem({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="min-w-0">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-sm font-medium">{value}</div>
    </div>
  );
}

function formatLabel(value: string) {
  if (!value) return "";
  return value.charAt(0).toUpperCase() + value.slice(1).replaceAll("_", " ");
}

function summarizeSelectedMembers(
  selectedIds: string[],
  members: { user_id: string; name: string }[],
  emptyLabel: string,
) {
  if (selectedIds.length === 0) return emptyLabel;
  const names = selectedIds.map((id) => members.find((member) => member.user_id === id)?.name ?? "Unknown");
  const visibleNames = names.slice(0, 3).join(", ");
  const overflowCount = names.length - 3;
  return overflowCount > 0 ? `${visibleNames} +${overflowCount} more` : visibleNames;
}

function formatTriggerTitle(type: string) {
  if (type === "api") return "API";
  if (type === "github") return "GitHub";
  if (type === "schedule") return "Schedule";
  return formatLabel(type);
}

function formatTriggerDetail(trigger: Routine["triggers"][number]) {
  if (trigger.trigger_type === "api") {
    return getRoutineTriggerURL(trigger.id);
  }
  if (trigger.trigger_type === "github") {
    const eventTypes = trigger.config?.event_types;
    if (Array.isArray(eventTypes) && eventTypes.length > 0) {
      return eventTypes.filter((event): event is string => typeof event === "string").join(", ");
    }
    return "GitHub events";
  }
  if (trigger.trigger_type === "schedule") {
    if (trigger.schedule) return `Cron ${trigger.schedule} · ${trigger.timezone}`;
    if (trigger.run_at) return `Runs once at ${trigger.run_at}`;
    return `Scheduled · ${trigger.timezone}`;
  }
  return trigger.trigger_type;
}

function getRoutineTriggerURL(triggerID: string) {
  const baseURL = process.env.NEXT_PUBLIC_API_URL || (typeof window !== "undefined" ? window.location.origin : "");
  return `${baseURL}/api/routine-triggers/${triggerID}`;
}

function routineToUpdatePayload(routine: Routine, enabled: boolean) {
  return {
    name: routine.name,
    instructions: routine.instructions ?? null,
    priority: routine.priority,
    assignee_type: routine.assignee_type ?? null,
    assignee_id: routine.assignee_id ?? null,
    due_date_offset_hours: routine.due_date_offset_hours ?? null,
    dispatch_provider: routine.dispatch_provider ?? null,
    dispatch_daemon_id: routine.dispatch_daemon_id ?? null,
    dispatch_daemon_label: routine.dispatch_daemon_label ?? null,
    enabled,
    subscriber_ids: routine.subscriber_ids,
    label_ids: routine.label_ids,
    triggers: routine.triggers.map((trigger) => ({
      id: trigger.id,
      trigger_type: trigger.trigger_type,
      source_type: trigger.source_type ?? null,
      schedule: trigger.schedule ?? null,
      timezone: trigger.timezone,
      run_at: trigger.run_at ?? null,
      dedup_window_seconds: trigger.dedup_window_seconds,
      max_runs: trigger.max_runs ?? null,
      config: trigger.config ?? {},
      enabled: trigger.enabled,
    })),
    actions: routine.actions.map((action) => ({
      action_type: action.action_type,
      config: action.config ?? {},
      enabled: action.enabled,
      position: action.position,
    })),
  };
}

