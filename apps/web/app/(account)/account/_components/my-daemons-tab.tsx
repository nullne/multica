"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Monitor, Server, Wifi, WifiOff } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { api } from "@/shared/api";
import type { Daemon, DaemonWorkspaceAssignment } from "@/shared/types";

function formatLastSeen(value: string | null): string {
  if (!value) return "Never seen";
  const timestamp = new Date(value).getTime();
  if (Number.isNaN(timestamp)) return "Unknown";
  const seconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
  if (seconds < 60) return "Just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function DaemonStatusBadge({ daemon }: { daemon: Daemon }) {
  const online = daemon.status === "online";
  return (
    <Badge variant={online ? "default" : "secondary"} className="capitalize">
      {online ? <Wifi className="h-3 w-3" /> : <WifiOff className="h-3 w-3" />}
      {daemon.status}
    </Badge>
  );
}

export function MyDaemonsTab() {
  const [daemons, setDaemons] = useState<Daemon[]>([]);
  const [selectedDaemonId, setSelectedDaemonId] = useState("");
  const [assignments, setAssignments] = useState<DaemonWorkspaceAssignment[]>([]);
  const [loadingDaemons, setLoadingDaemons] = useState(true);
  const [loadingAssignments, setLoadingAssignments] = useState(false);
  const [updatingWorkspaceId, setUpdatingWorkspaceId] = useState<string | null>(null);

  const selectedDaemon = useMemo(
    () => daemons.find((daemon) => daemon.id === selectedDaemonId) ?? null,
    [daemons, selectedDaemonId],
  );

  const loadAssignments = useCallback(async (daemonId: string) => {
    setLoadingAssignments(true);
    try {
      setAssignments(await api.listMyDaemonWorkspaces(daemonId));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to load daemon workspaces");
    } finally {
      setLoadingAssignments(false);
    }
  }, []);

  const loadDaemons = useCallback(async () => {
    setLoadingDaemons(true);
    try {
      const list = await api.listMyDaemons();
      setDaemons(list);
      setSelectedDaemonId((current) =>
        current && list.some((daemon) => daemon.id === current)
          ? current
          : list[0]?.id ?? "",
      );
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to load daemons");
    } finally {
      setLoadingDaemons(false);
    }
  }, []);

  useEffect(() => {
    loadDaemons();
  }, [loadDaemons]);

  useEffect(() => {
    if (!selectedDaemonId) {
      setAssignments([]);
      return;
    }
    loadAssignments(selectedDaemonId);
  }, [selectedDaemonId, loadAssignments]);

  const handleToggleWorkspace = async (
    assignment: DaemonWorkspaceAssignment,
    enabled: boolean,
  ) => {
    if (!selectedDaemon) return;
    setUpdatingWorkspaceId(assignment.workspace_id);
    try {
      await api.setMyDaemonWorkspaceEnabled(
        selectedDaemon.id,
        assignment.workspace_id,
        enabled,
      );
      await loadAssignments(selectedDaemon.id);
      toast.success(
        enabled
          ? `Enabled ${selectedDaemon.device_name || selectedDaemon.daemon_id} for ${assignment.workspace_name}`
          : `Disabled ${selectedDaemon.device_name || selectedDaemon.daemon_id} for ${assignment.workspace_name}`,
      );
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update workspace assignment");
    } finally {
      setUpdatingWorkspaceId(null);
    }
  };

  return (
    <div className="space-y-6">
      <section className="space-y-2">
        <div className="flex items-center gap-2">
          <Monitor className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">My Daemons</h2>
        </div>
        <p className="text-xs text-muted-foreground">
          Manage local daemon machines owned by your account and choose which workspaces
          each daemon is allowed to serve.
        </p>
      </section>

      {loadingDaemons ? (
        <div className="grid gap-3 md:grid-cols-[240px_1fr]">
          <Skeleton className="h-32 rounded-xl" />
          <Skeleton className="h-32 rounded-xl" />
        </div>
      ) : daemons.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-10 text-center">
            <Server className="h-8 w-8 text-muted-foreground/40" />
            <p className="mt-3 text-sm font-medium">No daemons registered</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Run <code className="rounded bg-muted px-1 py-0.5">multica daemon start</code>{" "}
              after logging in to register this machine to your account.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-[240px_1fr]">
          <div className="space-y-2">
            {daemons.map((daemon) => {
              const selected = daemon.id === selectedDaemonId;
              return (
                <button
                  key={daemon.id}
                  type="button"
                  onClick={() => setSelectedDaemonId(daemon.id)}
                  className={cn(
                    "flex w-full items-start gap-3 rounded-xl border p-3 text-left transition-colors",
                    selected ? "border-primary bg-primary/5" : "hover:bg-accent",
                    daemon.archived_at && "opacity-60",
                  )}
                >
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-muted">
                    <Monitor className="h-4 w-4 text-muted-foreground" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">
                      {daemon.device_name || daemon.daemon_id}
                    </span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {daemon.daemon_id}
                    </span>
                    <span className="mt-2 flex items-center gap-2">
                      <DaemonStatusBadge daemon={daemon} />
                    </span>
                  </span>
                </button>
              );
            })}
          </div>

          <Card>
            <CardContent className="space-y-5">
              {selectedDaemon && (
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <h3 className="truncate text-sm font-semibold">
                      {selectedDaemon.device_name || selectedDaemon.daemon_id}
                    </h3>
                    <p className="text-xs text-muted-foreground">
                      multica {selectedDaemon.cli_version || "unknown"} · {formatLastSeen(selectedDaemon.last_seen_at)}
                    </p>
                  </div>
                  <DaemonStatusBadge daemon={selectedDaemon} />
                </div>
              )}

              <div className="space-y-2">
                <div>
                  <h4 className="text-xs font-medium text-muted-foreground">
                    Workspace Access
                  </h4>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Enabled workspaces can dispatch local agent work to this daemon.
                  </p>
                </div>

                {loadingAssignments ? (
                  <div className="space-y-2">
                    {Array.from({ length: 2 }).map((_, index) => (
                      <Skeleton key={index} className="h-14 rounded-lg" />
                    ))}
                  </div>
                ) : assignments.length === 0 ? (
                  <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                    This daemon has no eligible workspaces. Join or create a workspace,
                    then re-register the daemon.
                  </div>
                ) : (
                  <div className="divide-y rounded-lg border">
                    {assignments.map((assignment) => (
                      <div
                        key={assignment.workspace_id}
                        className="flex items-center justify-between gap-3 px-3 py-3"
                      >
                        <div className="min-w-0">
                          <div className="truncate text-sm font-medium">
                            {assignment.workspace_name}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {assignment.enabled ? "Enabled for dispatch" : "Disabled"}
                          </div>
                        </div>
                        <Switch
                          aria-label={`Enable ${assignment.workspace_name}`}
                          checked={assignment.enabled}
                          disabled={updatingWorkspaceId === assignment.workspace_id}
                          onCheckedChange={(enabled) =>
                            handleToggleWorkspace(assignment, enabled)
                          }
                        />
                      </div>
                    ))}
                  </div>
                )}
              </div>

              <div className="rounded-lg bg-muted/50 px-3 py-2 text-xs text-muted-foreground">
                CLI: use <code>multica daemon assignments</code> to inspect the same
                workspace access from your terminal.
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
