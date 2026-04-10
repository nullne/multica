import { useState, useEffect, useCallback } from "react";
import { Server, ArrowUpCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { AgentRuntime, WorkspaceProviderSettings } from "@/shared/types";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import { toast } from "sonner";
import { RuntimeModeIcon } from "./shared";

function RuntimeListItem({
  runtime,
  isSelected,
  onClick,
}: {
  runtime: AgentRuntime;
  isSelected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`flex w-full items-center gap-3 px-4 py-3 text-left transition-colors ${
        isSelected ? "bg-accent" : "hover:bg-accent/50"
      }`}
    >
      <div
        className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${
          runtime.status === "online" ? "bg-success/10" : "bg-muted"
        }`}
      >
        <RuntimeModeIcon mode={runtime.runtime_mode} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium">{runtime.name}</div>
        <div className="mt-0.5 truncate text-xs text-muted-foreground">
          {runtime.provider} &middot; {runtime.runtime_mode}
        </div>
      </div>
      <div
        className={`h-2 w-2 shrink-0 rounded-full ${
          runtime.status === "online" ? "bg-success" : "bg-muted-foreground/40"
        }`}
      />
    </button>
  );
}

export function RuntimeList({
  runtimes,
  selectedId,
  onSelect,
}: {
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
      // Fetch provider config to get target versions.
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

  const hasOnlineDaemons = runtimes.some((r) => r.status === "online");

  return (
    <div className="overflow-y-auto h-full border-r">
      <div className="flex h-12 items-center justify-between border-b px-4">
        <h1 className="text-sm font-semibold">Runtimes</h1>
        <div className="flex items-center gap-2">
          {hasOnlineDaemons && (
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
            {runtimes.filter((r) => r.status === "online").length}/
            {runtimes.length} online
          </span>
        </div>
      </div>
      {runtimes.length === 0 ? (
        <div className="flex flex-col items-center justify-center px-4 py-12">
          <Server className="h-8 w-8 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">
            No runtimes registered
          </p>
          <p className="mt-1 text-xs text-muted-foreground text-center">
            Run{" "}
            <code className="rounded bg-muted px-1 py-0.5">
              multica daemon start
            </code>{" "}
            to register a local runtime.
          </p>
        </div>
      ) : (
        <div className="divide-y">
          {runtimes.map((runtime) => (
            <RuntimeListItem
              key={runtime.id}
              runtime={runtime}
              isSelected={runtime.id === selectedId}
              onClick={() => onSelect(runtime.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
