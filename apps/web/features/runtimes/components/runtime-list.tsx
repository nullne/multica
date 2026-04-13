import { useState, useCallback } from "react";
import { Server, Monitor, ArrowUpCircle, ShieldCheck, ShieldAlert, ShieldX } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { Daemon, AgentRuntime, WorkspaceProviderSettings, ProviderAuthStatus } from "@/shared/types";
import { useWorkspaceStore } from "@/features/workspace";
import { useRuntimeStore } from "../store";
import { api } from "@/shared/api";
import { toast } from "sonner";
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
  onClick,
}: {
  daemon: Daemon;
  daemonRuntimes: AgentRuntime[];
  enabledProviders: string[];
  isSelected: boolean;
  onClick: () => void;
}) {
  const installedSet = new Set(daemonRuntimes.map((r) => r.provider));
  const missingProviders = enabledProviders.filter((p) => !installedSet.has(p));

  return (
    <button
      onClick={onClick}
      className={`flex w-full items-center gap-3 px-4 py-3 text-left transition-colors ${
        isSelected ? "bg-accent" : "hover:bg-accent/50"
      }`}
    >
      <div
        className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${
          daemon.status === "online" ? "bg-success/10" : "bg-muted"
        }`}
      >
        <Monitor className="h-3.5 w-3.5" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium">
          {daemon.device_name || daemon.daemon_id}
        </div>
        <div className="mt-0.5 truncate text-xs text-muted-foreground">
          {daemon.cli_version && `multica ${daemon.cli_version}`}
          {daemon.cli_version && daemon.last_seen_at && " · "}
          {daemon.last_seen_at && formatLastSeen(daemon.last_seen_at)}
        </div>
        {(daemonRuntimes.length > 0 || missingProviders.length > 0) && (
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
      <span
        className={`h-2 w-2 shrink-0 rounded-full ${
          daemon.status === "online" ? "bg-success" : "bg-muted-foreground/40"
        }`}
      />
    </button>
  );
}

function DaemonListItemWrapper({
  daemon,
  runtimes,
  isSelected,
  onClick,
}: {
  daemon: Daemon;
  runtimes: AgentRuntime[];
  isSelected: boolean;
  onClick: () => void;
}) {
  const enabledProviders = useRuntimeStore((s) => s.enabledProviders);
  return (
    <DaemonListItem
      daemon={daemon}
      daemonRuntimes={runtimes.filter((r) => r.daemon_ref === daemon.id)}
      enabledProviders={enabledProviders}
      isSelected={isSelected}
      onClick={onClick}
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
  const [updating, setUpdating] = useState(false);

  const handleUpdateAll = useCallback(async () => {
    if (!workspace) return;
    setUpdating(true);
    try {
      const config: WorkspaceProviderSettings = await api.getProviderConfig(workspace.id);
      const targets: { target: string; version: string }[] = [];
      if (config.multica_target_version) {
        targets.push({ target: "multica", version: config.multica_target_version });
      }
      if (config.providers) {
        for (const [provider, pc] of Object.entries(config.providers)) {
          if (pc.enabled && pc.target_version) {
            targets.push({ target: provider, version: pc.target_version });
          }
        }
      }
      if (targets.length === 0) {
        toast.error("No target versions configured. Set versions in Settings > Providers.");
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

  const hasOnline = daemons.some((d) => d.status === "online");
  const onlineCount = daemons.filter((d) => d.status === "online").length;

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
            {onlineCount}/{daemons.length} online
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
        <div className="divide-y">
          {daemons.map((daemon) => (
            <DaemonListItemWrapper
              key={daemon.id}
              daemon={daemon}
              runtimes={runtimes}
              isSelected={daemon.id === selectedId}
              onClick={() => onSelect(daemon.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
