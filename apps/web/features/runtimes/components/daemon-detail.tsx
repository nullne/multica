import { useState, useCallback } from "react";
import { Monitor, Archive, ArchiveRestore, MoreHorizontal, Terminal, Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "sonner";
import type { Daemon, AgentRuntime } from "@/shared/types";
import { api } from "@/shared/api";
import { useRuntimeStore } from "../store";
import { formatLastSeen } from "../utils";
import { StatusBadge, AuthStatusBadge, InfoField } from "./shared";
import { UpdateSection } from "./update-section";
import { UsageSection } from "./usage-section";
import { PingSection } from "./ping-section";

function ProviderTabContent({ runtime }: { runtime: AgentRuntime }) {
  return (
    <div className="space-y-4 pt-3">
      <UsageSection runtimeId={runtime.id} />
    </div>
  );
}

function EditableDeviceName({ daemon }: { daemon: Daemon }) {
  const patchDaemon = useRuntimeStore((s) => s.patchDaemon);
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(daemon.device_name || daemon.daemon_id);

  const save = async () => {
    const trimmed = value.trim();
    if (!trimmed || trimmed === (daemon.device_name || daemon.daemon_id)) {
      setEditing(false);
      return;
    }
    try {
      await api.updateDaemon(daemon.id, { device_name: trimmed });
      patchDaemon(daemon.id, { device_name: trimmed });
      setEditing(false);
    } catch {
      toast.error("Failed to update name");
    }
  };

  if (editing) {
    return (
      <Input
        autoFocus
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onBlur={save}
        onKeyDown={(e) => { if (e.key === "Enter") save(); if (e.key === "Escape") setEditing(false); }}
        className="h-7 text-sm font-semibold"
      />
    );
  }

  return (
    <button
      onClick={() => setEditing(true)}
      className="text-sm font-semibold truncate hover:underline decoration-dashed underline-offset-4 cursor-pointer text-left"
      title="Click to rename"
    >
      {daemon.device_name || daemon.daemon_id}
    </button>
  );
}

function EnvDialog({ daemonId, open, onOpenChange }: { daemonId: string; open: boolean; onOpenChange: (v: boolean) => void }) {
  const [envVars, setEnvVars] = useState<Record<string, string> | null>(null);
  const [loading, setLoading] = useState(false);
  const [filter, setFilter] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.getDaemonEnv(daemonId);
      setEnvVars(data);
    } catch {
      toast.error("Failed to load environment variables");
    } finally {
      setLoading(false);
    }
  }, [daemonId]);

  const handleOpenChange = useCallback((v: boolean) => {
    onOpenChange(v);
    if (v) {
      setFilter("");
      load();
    }
  }, [onOpenChange, load]);

  const entries = envVars
    ? Object.entries(envVars)
        .sort(([a], [b]) => a.localeCompare(b))
        .filter(([k, v]) => {
          if (!filter) return true;
          const lower = filter.toLowerCase();
          return k.toLowerCase().includes(lower) || v.toLowerCase().includes(lower);
        })
    : [];

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>Environment Variables</DialogTitle>
        </DialogHeader>
        <div className="relative">
          <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Filter variables..."
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="pl-8 h-8 text-sm"
          />
        </div>
        <div className="flex-1 overflow-y-auto min-h-0 border rounded-md">
          {loading ? (
            <div className="p-4 text-sm text-muted-foreground text-center">Loading...</div>
          ) : entries.length === 0 ? (
            <div className="p-4 text-sm text-muted-foreground text-center">
              {envVars && Object.keys(envVars).length === 0
                ? "No environment variables reported. Re-register the daemon to populate."
                : "No matches"}
            </div>
          ) : (
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-muted/80 backdrop-blur-sm">
                <tr>
                  <th className="text-left font-medium px-3 py-1.5 text-muted-foreground">Name</th>
                  <th className="text-left font-medium px-3 py-1.5 text-muted-foreground">Value</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {entries.map(([key, value]) => (
                  <tr key={key} className="hover:bg-muted/40">
                    <td className="px-3 py-1.5 font-mono whitespace-nowrap font-medium">{key}</td>
                    <td className="px-3 py-1.5 font-mono break-all text-muted-foreground">{value}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
        {envVars && Object.keys(envVars).length > 0 && (
          <div className="text-xs text-muted-foreground">
            {entries.length} of {Object.keys(envVars).length} variables
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

interface ProviderEntry {
  provider: string;
  runtime: AgentRuntime | null;
}

function buildProviderEntries(
  daemonRuntimes: AgentRuntime[],
  enabledProviders: string[],
): ProviderEntry[] {
  const entries: ProviderEntry[] = daemonRuntimes.map((rt) => ({
    provider: rt.provider,
    runtime: rt,
  }));
  const installedSet = new Set(daemonRuntimes.map((r) => r.provider));
  for (const name of enabledProviders) {
    if (!installedSet.has(name)) {
      entries.push({ provider: name, runtime: null });
    }
  }
  return entries;
}

export function DaemonDetail({
  daemon,
  runtimes,
}: {
  daemon: Daemon;
  runtimes: AgentRuntime[];
}) {
  const enabledProviders = useRuntimeStore((s) => s.enabledProviders);
  const patchDaemon = useRuntimeStore((s) => s.patchDaemon);
  const daemonRuntimes = runtimes.filter((r) => r.daemon_ref === daemon.id);
  const providerEntries = buildProviderEntries(daemonRuntimes, enabledProviders);
  const firstRuntimeId = daemonRuntimes[0]?.id;
  const isArchived = !!daemon.archived_at;
  const [envOpen, setEnvOpen] = useState(false);

  const handleArchiveToggle = useCallback(async () => {
    try {
      if (isArchived) {
        const updated = await api.restoreDaemon(daemon.id);
        patchDaemon(daemon.id, { archived_at: updated.archived_at });
        toast.success("Daemon restored");
      } else {
        const updated = await api.archiveDaemon(daemon.id);
        patchDaemon(daemon.id, { archived_at: updated.archived_at });
        toast.success("Daemon archived");
      }
    } catch {
      toast.error(isArchived ? "Failed to restore daemon" : "Failed to archive daemon");
    }
  }, [daemon.id, isArchived, patchDaemon]);

  return (
    <div className="flex h-full flex-col">
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex min-w-0 items-center gap-2">
          <div
            className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md ${
              daemon.status === "online" ? "bg-success/10" : daemon.status === "updating" ? "bg-info/10" : "bg-muted"
            }`}
          >
            <Monitor className="h-3.5 w-3.5" />
          </div>
          <div className="min-w-0">
            <EditableDeviceName daemon={daemon} />
          </div>
        </div>
        <div className="flex items-center gap-2">
          <StatusBadge status={daemon.status} />
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="ghost" size="icon-xs">
                  <MoreHorizontal className="h-3.5 w-3.5" />
                </Button>
              }
            />
            <DropdownMenuContent align="end" className="w-36">
              <DropdownMenuItem onClick={handleArchiveToggle}>
                {isArchived ? (
                  <><ArchiveRestore className="h-3.5 w-3.5" /> Restore</>
                ) : (
                  <><Archive className="h-3.5 w-3.5" /> Archive</>
                )}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        <div className="grid grid-cols-2 gap-4">
          <InfoField label="Daemon ID" value={daemon.daemon_id} mono />
          <InfoField label="Status" value={daemon.status} />
          <InfoField label="CLI Version" value={daemon.cli_version || "unknown"} />
          <InfoField
            label="Last Seen"
            value={formatLastSeen(daemon.last_seen_at)}
          />
        </div>

        {/* CLI Update */}
        {firstRuntimeId && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              CLI Update
            </h3>
            <UpdateSection
              runtimeId={firstRuntimeId}
              currentVersion={daemon.cli_version || null}
              isOnline={daemon.status === "online"}
            />
          </div>
        )}

        {/* Connection Test */}
        {firstRuntimeId && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              Connection Test
            </h3>
            <PingSection runtimeId={firstRuntimeId} />
          </div>
        )}

        {/* Environment Variables */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Environment
          </h3>
          <Button variant="outline" size="sm" onClick={() => setEnvOpen(true)}>
            <Terminal className="h-3.5 w-3.5" />
            View Environment Variables
          </Button>
          <EnvDialog daemonId={daemon.id} open={envOpen} onOpenChange={setEnvOpen} />
        </div>

        {/* Providers with tabs */}
        {providerEntries.length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              Providers ({providerEntries.length})
            </h3>
            {providerEntries.length === 1 && providerEntries[0] ? (
              providerEntries[0].runtime ? (
                <div className="space-y-3">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium capitalize">{providerEntries[0].provider}</span>
                    <AuthStatusBadge authStatus={providerEntries[0].runtime.auth_status} />
                  </div>
                  <ProviderTabContent runtime={providerEntries[0].runtime} />
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium capitalize">{providerEntries[0].provider}</span>
                  <AuthStatusBadge authStatus="not_installed" />
                </div>
              )
            ) : (
              <Tabs defaultValue={providerEntries[0]?.provider ?? ""}>
                <TabsList>
                  {providerEntries.map((entry) => (
                    <TabsTrigger key={entry.provider} value={entry.provider} className="gap-1.5">
                      <span className="capitalize">{entry.provider}</span>
                      <AuthStatusBadge authStatus={entry.runtime?.auth_status ?? "not_installed"} />
                    </TabsTrigger>
                  ))}
                </TabsList>
                {providerEntries.map((entry) => (
                  <TabsContent key={entry.provider} value={entry.provider}>
                    {entry.runtime && <ProviderTabContent runtime={entry.runtime} />}
                  </TabsContent>
                ))}
              </Tabs>
            )}
          </div>
        )}

        {/* Timestamps */}
        <div className="grid grid-cols-2 gap-4 border-t pt-4">
          <InfoField
            label="Registered"
            value={new Date(daemon.created_at).toLocaleString()}
          />
          <InfoField
            label="Updated"
            value={new Date(daemon.updated_at).toLocaleString()}
          />
        </div>
      </div>
    </div>
  );
}
