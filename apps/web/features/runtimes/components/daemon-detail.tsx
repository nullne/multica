import { useState } from "react";
import { Monitor, Plus, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import type { Daemon, AgentRuntime } from "@/shared/types";
import { api } from "@/shared/api";
import { formatLastSeen } from "../utils";
import { StatusBadge, InfoField } from "./shared";
import { UpdateSection } from "./update-section";
import { UsageSection } from "./usage-section";
import { PingSection } from "./ping-section";

function LabelsEditor({ daemon }: { daemon: Daemon }) {
  const [labels, setLabels] = useState<string[]>(daemon.labels ?? []);
  const [newLabel, setNewLabel] = useState("");

  const save = async (next: string[]) => {
    setLabels(next);
    try {
      await api.updateDaemon(daemon.id, { labels: next });
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

function ProviderCard({ runtime }: { runtime: AgentRuntime }) {
  const version =
    runtime.metadata && typeof runtime.metadata === "object" && "version" in runtime.metadata
      ? String((runtime.metadata as Record<string, unknown>).version)
      : null;

  return (
    <div className="rounded-lg border px-4 py-3 space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium capitalize">{runtime.provider}</span>
        {version && (
          <span className="text-xs text-muted-foreground font-mono">{version}</span>
        )}
      </div>

      <div>
        <h4 className="text-xs text-muted-foreground mb-2">Token Usage</h4>
        <UsageSection runtimeId={runtime.id} />
      </div>
    </div>
  );
}

export function DaemonDetail({
  daemon,
  runtimes,
}: {
  daemon: Daemon;
  runtimes: AgentRuntime[];
}) {
  const daemonRuntimes = runtimes.filter((r) => r.daemon_ref === daemon.id);
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
            <h2 className="text-sm font-semibold truncate">
              {daemon.device_name || daemon.daemon_id}
            </h2>
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
          {daemon.device_info && (
            <InfoField label="Device" value={daemon.device_info} />
          )}
          <InfoField
            label="Providers"
            value={
              daemonRuntimes.length > 0
                ? daemonRuntimes.map((r) => r.provider).join(", ")
                : "none"
            }
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

        {/* Providers */}
        {daemonRuntimes.length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              Providers ({daemonRuntimes.length})
            </h3>
            <div className="space-y-3">
              {daemonRuntimes.map((rt) => (
                <ProviderCard key={rt.id} runtime={rt} />
              ))}
            </div>
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
