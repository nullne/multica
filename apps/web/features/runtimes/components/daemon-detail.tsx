import { useState } from "react";
import { Monitor, Plus, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
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

function LabelsEditor({ daemon }: { daemon: Daemon }) {
  const patchDaemon = useRuntimeStore((s) => s.patchDaemon);
  const [labels, setLabels] = useState<string[]>(daemon.labels ?? []);
  const [newLabel, setNewLabel] = useState("");

  const save = async (next: string[]) => {
    setLabels(next);
    try {
      await api.updateDaemon(daemon.id, { labels: next });
      patchDaemon(daemon.id, { labels: next });
    } catch {
      toast.error("Failed to update labels");
      setLabels(daemon.labels ?? []);
    }
  };

  const addLabel = () => {
    const val = newLabel.trim().toLowerCase();
    if (!val || labels.includes(val)) return;
    save([...labels, val]);
    setNewLabel("");
  };

  const removeLabel = (label: string) => {
    save(labels.filter((l) => l !== label));
  };

  return (
    <div>
      <div className="flex flex-wrap gap-1.5 mb-2">
        {labels.map((label) => (
          <span
            key={label}
            className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-xs font-medium"
          >
            {label}
            <button
              type="button"
              onClick={() => removeLabel(label)}
              className="text-muted-foreground hover:text-foreground"
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        ))}
        {labels.length === 0 && (
          <span className="text-xs text-muted-foreground">No labels</span>
        )}
      </div>
      <div className="flex gap-1.5">
        <Input
          value={newLabel}
          onChange={(e) => setNewLabel(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && addLabel()}
          placeholder="Add label..."
          className="h-7 text-xs flex-1"
        />
        <Button
          variant="ghost"
          size="xs"
          onClick={addLabel}
          disabled={!newLabel.trim()}
        >
          <Plus className="h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}

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
  const daemonRuntimes = runtimes.filter((r) => r.daemon_ref === daemon.id);
  const providerEntries = buildProviderEntries(daemonRuntimes, enabledProviders);
  const firstRuntimeId = daemonRuntimes[0]?.id;

  return (
    <div className="flex h-full flex-col">
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex min-w-0 items-center gap-2">
          <div
            className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md ${
              daemon.status === "online" ? "bg-success/10" : "bg-muted"
            }`}
          >
            <Monitor className="h-3.5 w-3.5" />
          </div>
          <div className="min-w-0">
            <EditableDeviceName daemon={daemon} />
          </div>
        </div>
        <StatusBadge status={daemon.status} />
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

        {/* Labels */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Labels
          </h3>
          <LabelsEditor daemon={daemon} />
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
