import { useState, useCallback } from "react";
import { Server, Monitor, ArrowUpCircle, ShieldCheck, ShieldAlert, ShieldX, MoreHorizontal, Archive, ArchiveRestore, ChevronRight, User } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import type { Daemon, AgentRuntime, WorkspaceProviderSettings, ProviderAuthStatus } from "@/shared/types";
import { useWorkspaceStore } from "@/features/workspace";
import { useRuntimeStore } from "../store";
import { api } from "@/shared/api";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { formatLastSeen } from "../utils";

const authDotColor: Record<ProviderAuthStatus, string> = {
  ready: "text-success",
  unauthenticated: "text-warning",
  not_installed: "text-destructive",
  unknown: "text-muted-foreground",
};

const AuthIcon = ({ status }: { status: ProviderAuthStatus }) => {
  const cls = `h-3 w-3 ${authDotColor[status] ?? authDotColor.unknown}`;
  if (status === "ready") return <ShieldCheck className={cls} />;
  if (status === "not_installed") return <ShieldX className={cls} />;
  return <ShieldAlert className={cls} />;
};

function DaemonListItem({
  daemon,
  daemonRuntimes,
  enabledProviders,
  isSelected,
  isPersonal,
  onClick,
  onArchive,
  onRestore,
}: {
  daemon: Daemon;
  daemonRuntimes: AgentRuntime[];
  enabledProviders: string[];
  isSelected: boolean;
  isPersonal?: boolean;
  onClick: () => void;
  onArchive: () => void;
  onRestore: () => void;
}) {
  const isArchived = !!daemon.archived_at;
  const installedSet = new Set(daemonRuntimes.map((r) => r.provider));
  // Missing providers are workspace-scoped; only relevant for workspace-visible daemons.
  const missingProviders = isPersonal
    ? []
    : enabledProviders.filter((p) => !installedSet.has(p));

  return (
    <div
      className={cn(
        "group flex w-full items-start gap-3 px-4 py-3 text-left transition-colors",
        isSelected ? "bg-accent" : "hover:bg-accent/50",
        isArchived && "opacity-60",
      )}
    >
      <button onClick={onClick} className="flex min-w-0 flex-1 items-start gap-3">
        <div
          className={cn(
            "flex h-8 w-8 shrink-0 items-center justify-center rounded-lg",
            daemon.status === "online" ? "bg-success/10" : daemon.status === "updating" ? "bg-info/10" : "bg-muted",
          )}
        >
          <Monitor className="h-3.5 w-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <span className="truncate text-sm font-medium">
              {daemon.device_name || daemon.daemon_id}
            </span>
            {isPersonal && (
              <Badge
                variant="outline"
                className="h-4 px-1.5 text-[10px] font-medium text-muted-foreground"
                title="Personal daemon — not enabled in this workspace"
              >
                <User className="h-2.5 w-2.5" />
                Personal
              </Badge>
            )}
          </div>
          <div className="mt-0.5 truncate text-xs text-muted-foreground">
            {daemon.cli_version && `multica ${daemon.cli_version}`}
            {daemon.cli_version && daemon.last_seen_at && " · "}
            {daemon.last_seen_at && formatLastSeen(daemon.last_seen_at)}
          </div>
          {!isArchived && (daemonRuntimes.length > 0 || missingProviders.length > 0) && (
            <div className="mt-1 flex items-center gap-2 flex-wrap">
              {daemonRuntimes.map((rt) => (
                <span key={rt.id} className="inline-flex items-center gap-0.5 text-[11px] text-muted-foreground">
                  <AuthIcon status={rt.auth_status} />
                  <span className="capitalize">{rt.provider}</span>
                </span>
              ))}
              {missingProviders.map((p) => (
                <span key={p} className="inline-flex items-center gap-0.5 text-[11px] text-muted-foreground">
                  <AuthIcon status="not_installed" />
                  <span className="capitalize">{p}</span>
                </span>
              ))}
            </div>
          )}
        </div>
      </button>

      <div className="flex shrink-0 items-center gap-1.5 mt-2.5">
        <span
          className={cn(
            "h-2 w-2 shrink-0 rounded-full",
            daemon.status === "online" ? "bg-success" : daemon.status === "updating" ? "bg-info animate-pulse" : "bg-muted-foreground/40",
          )}
        />
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                variant="ghost"
                size="icon-xs"
                className="opacity-0 group-hover:opacity-100 transition-opacity"
              >
                <MoreHorizontal className="h-3.5 w-3.5" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-36">
            {isArchived ? (
              <DropdownMenuItem onClick={onRestore}>
                <ArchiveRestore className="h-3.5 w-3.5" />
                Restore
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem onClick={onArchive}>
                <Archive className="h-3.5 w-3.5" />
                Archive
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}

function DaemonListItemWrapper({
  daemon,
  runtimes,
  isSelected,
  isPersonal,
  onClick,
  onArchive,
  onRestore,
}: {
  daemon: Daemon;
  runtimes: AgentRuntime[];
  isSelected: boolean;
  isPersonal?: boolean;
  onClick: () => void;
  onArchive: () => void;
  onRestore: () => void;
}) {
  const enabledProviders = useRuntimeStore((s) => s.enabledProviders);
  return (
    <DaemonListItem
      daemon={daemon}
      daemonRuntimes={runtimes.filter((r) => r.daemon_ref === daemon.id)}
      enabledProviders={enabledProviders}
      isSelected={isSelected}
      isPersonal={isPersonal}
      onClick={onClick}
      onArchive={onArchive}
      onRestore={onRestore}
    />
  );
}

export function RuntimeList({
  daemons,
  runtimes,
  selectedId,
  onSelect,
}: {
  daemons: Daemon[];
  runtimes: AgentRuntime[];
  selectedId: string;
  onSelect: (id: string) => void;
}) {
  const workspace = useWorkspaceStore((s) => s.workspace);
  const patchDaemon = useRuntimeStore((s) => s.patchDaemon);
  const [updating, setUpdating] = useState(false);
  const [archivedOpen, setArchivedOpen] = useState(false);
  const [personalOpen, setPersonalOpen] = useState(false);

  // Workspace daemons are surfaced by default; personal (non-workspace-visible)
  // daemons live in a collapsible section so they aren't mistaken for
  // workspace-enabled ones.
  const activeWorkspaceDaemons = daemons.filter(
    (d) => !d.archived_at && d.workspace_visible,
  );
  const personalDaemons = daemons.filter(
    (d) => !d.archived_at && !d.workspace_visible,
  );
  const archivedDaemons = daemons.filter((d) => !!d.archived_at);

  const handleArchive = useCallback(async (id: string) => {
    try {
      const updated = await api.archiveDaemon(id);
      patchDaemon(id, { archived_at: updated.archived_at });
      toast.success("Daemon archived");
    } catch {
      toast.error("Failed to archive daemon");
    }
  }, [patchDaemon]);

  const handleRestore = useCallback(async (id: string) => {
    try {
      const updated = await api.restoreDaemon(id);
      patchDaemon(id, { archived_at: updated.archived_at });
      toast.success("Daemon restored");
    } catch {
      toast.error("Failed to restore daemon");
    }
  }, [patchDaemon]);

  const handleUpdateAll = useCallback(async () => {
    if (!workspace) return;
    setUpdating(true);
    try {
      const config: WorkspaceProviderSettings = await api.getProviderConfig(workspace.id);
      const targets: { target: string; version: string }[] = [];
      if (config.providers) {
        for (const [provider, pc] of Object.entries(config.providers)) {
          if (pc.enabled && pc.target_version) {
            targets.push({ target: provider, version: pc.target_version });
          }
        }
      }
      if (targets.length === 0) {
        toast.error("No managed provider versions available for enabled providers.");
        return;
      }
      const result = await api.updateAllDaemons(workspace.id, targets);
      toast.success(`Updating ${result.daemons_count} daemon(s) — ${result.updates_queued} update(s) queued`);
    } catch {
      toast.error("Failed to trigger daemon updates");
    } finally {
      setUpdating(false);
    }
  }, [workspace]);

  // Online stats reflect what's available to this workspace; personal daemons
  // are not counted.
  const hasOnline = activeWorkspaceDaemons.some((d) => d.status === "online");
  const onlineCount = activeWorkspaceDaemons.filter(
    (d) => d.status === "online",
  ).length;

  return (
    <div className="overflow-y-auto h-full border-r">
      <div className="flex h-12 items-center justify-between border-b px-4">
        <h1 className="text-sm font-semibold">Daemons</h1>
        <div className="flex items-center gap-2">
          {hasOnline && (
            <Button
              variant="ghost"
              size="xs"
              onClick={handleUpdateAll}
              disabled={updating}
              title="Update all daemons to target versions"
            >
              <ArrowUpCircle className="h-3.5 w-3.5" />
            </Button>
          )}
          <span className="text-xs text-muted-foreground">
            {onlineCount}/{activeWorkspaceDaemons.length} online
          </span>
        </div>
      </div>
      {daemons.length === 0 ? (
        <div className="flex flex-col items-center justify-center px-4 py-12">
          <Server className="h-8 w-8 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">
            No daemons registered
          </p>
          <p className="mt-1 text-xs text-muted-foreground text-center">
            Run{" "}
            <code className="rounded bg-muted px-1 py-0.5">
              multica daemon start
            </code>{" "}
            to register a daemon.
          </p>
        </div>
      ) : (
        <>
          {activeWorkspaceDaemons.length === 0 ? (
            <div className="flex flex-col items-center justify-center px-4 py-10">
              <Server className="h-8 w-8 text-muted-foreground/40" />
              <p className="mt-3 text-sm text-muted-foreground">
                No daemons enabled in this workspace
              </p>
              {personalDaemons.length > 0 && (
                <p className="mt-1 text-xs text-muted-foreground text-center">
                  Expand <span className="font-medium">Personal</span> below to
                  see your own daemons.
                </p>
              )}
            </div>
          ) : (
            <div className="divide-y">
              {activeWorkspaceDaemons.map((daemon) => (
                <DaemonListItemWrapper
                  key={daemon.id}
                  daemon={daemon}
                  runtimes={runtimes}
                  isSelected={daemon.id === selectedId}
                  onClick={() => onSelect(daemon.id)}
                  onArchive={() => handleArchive(daemon.id)}
                  onRestore={() => handleRestore(daemon.id)}
                />
              ))}
            </div>
          )}

          {personalDaemons.length > 0 && (
            <Collapsible open={personalOpen} onOpenChange={setPersonalOpen}>
              <CollapsibleTrigger className="flex w-full items-center gap-1.5 border-t px-4 py-2 text-xs text-muted-foreground hover:text-foreground transition-colors">
                <ChevronRight className={cn("h-3 w-3 transition-transform", personalOpen && "rotate-90")} />
                <User className="h-3 w-3" />
                <span>Personal ({personalDaemons.length})</span>
                <span className="ml-auto text-[10px] text-muted-foreground/70">
                  Not in workspace
                </span>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <div className="divide-y">
                  {personalDaemons.map((daemon) => (
                    <DaemonListItemWrapper
                      key={daemon.id}
                      daemon={daemon}
                      runtimes={runtimes}
                      isSelected={daemon.id === selectedId}
                      isPersonal
                      onClick={() => onSelect(daemon.id)}
                      onArchive={() => handleArchive(daemon.id)}
                      onRestore={() => handleRestore(daemon.id)}
                    />
                  ))}
                </div>
              </CollapsibleContent>
            </Collapsible>
          )}

          {archivedDaemons.length > 0 && (
            <Collapsible open={archivedOpen} onOpenChange={setArchivedOpen}>
              <CollapsibleTrigger className="flex w-full items-center gap-1.5 border-t px-4 py-2 text-xs text-muted-foreground hover:text-foreground transition-colors">
                <ChevronRight className={cn("h-3 w-3 transition-transform", archivedOpen && "rotate-90")} />
                <Archive className="h-3 w-3" />
                <span>Archived ({archivedDaemons.length})</span>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <div className="divide-y">
                  {archivedDaemons.map((daemon) => (
                    <DaemonListItemWrapper
                      key={daemon.id}
                      daemon={daemon}
                      runtimes={runtimes}
                      isSelected={daemon.id === selectedId}
                      isPersonal={!daemon.workspace_visible}
                      onClick={() => onSelect(daemon.id)}
                      onArchive={() => handleArchive(daemon.id)}
                      onRestore={() => handleRestore(daemon.id)}
                    />
                  ))}
                </div>
              </CollapsibleContent>
            </Collapsible>
          )}
        </>
      )}
    </div>
  );
}
