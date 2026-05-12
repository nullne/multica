"use client";

import { useState, useMemo } from "react";
import { ChevronDown } from "lucide-react";
import type { UpdateIssueRequest, Daemon, AgentRuntime, ProviderAuthStatus } from "@/shared/types";
import { useRuntimeStore } from "@/features/runtimes";
import {
  PropertyPicker,
  PickerItem,
  PickerSection,
} from "./property-picker";

function authBadge(status: ProviderAuthStatus) {
  if (status === "ready") return null;
  if (status === "unauthenticated")
    return <span className="ml-auto text-[10px] text-amber-500">unauth</span>;
  if (status === "not_installed")
    return <span className="ml-auto text-[10px] text-muted-foreground">not installed</span>;
  return null;
}

function statusDot(daemon: Daemon) {
  return (
    <span
      className={`h-1.5 w-1.5 shrink-0 rounded-full ${daemon.status === "online" ? "bg-green-500" : "bg-muted-foreground/40"}`}
    />
  );
}

function classifyDaemons(
  daemons: Daemon[],
  runtimes: AgentRuntime[],
  provider: string | null,
) {
  const workspaceDaemons = daemons.filter((d) => d.workspace_visible);
  const personalDaemons = daemons.filter((d) => !d.workspace_visible);
  if (!provider) {
    return {
      compatible: workspaceDaemons,
      incompatible: [] as Daemon[],
      personal: personalDaemons,
    };
  }

  const compatible: Daemon[] = [];
  const incompatible: Daemon[] = [];
  for (const d of workspaceDaemons) {
    const hasRuntime = runtimes.some(
      (r) => r.daemon_ref === d.id && r.provider === provider,
    );
    if (hasRuntime) compatible.push(d);
    else incompatible.push(d);
  }
  return { compatible, incompatible, personal: personalDaemons };
}

function getRuntimeAuth(
  runtimes: AgentRuntime[],
  daemonId: string,
  provider: string | null,
): ProviderAuthStatus {
  if (!provider) return "unknown";
  const rt = runtimes.find(
    (r) => r.daemon_ref === daemonId && r.provider === provider,
  );
  return rt?.auth_status ?? "unknown";
}

export function DaemonPicker({
  daemonId,
  provider,
  onUpdate,
  onSelect,
  align,
  trigger: customTrigger,
  triggerRender,
}: {
  daemonId: string | null;
  provider: string | null;
  /** Called with UpdateIssueRequest-shaped updates (issue detail page). */
  onUpdate?: (updates: Partial<UpdateIssueRequest>) => void;
  /** Called with raw values (create-issue modal or other contexts). */
  onSelect?: (daemonId: string | null) => void;
  align?: "start" | "center" | "end";
  trigger?: React.ReactNode;
  triggerRender?: React.ReactElement;
}) {
  const [open, setOpen] = useState(false);
  const [showAll, setShowAll] = useState(false);
  const daemons = useRuntimeStore((s) => s.daemons);
  const runtimes = useRuntimeStore((s) => s.runtimes);

  const { compatible, incompatible, personal } = useMemo(
    () => classifyDaemons(daemons, runtimes, provider),
    [daemons, runtimes, provider],
  );

  const selectedName = daemonId
    ? (() => {
        const d = daemons.find((d) => d.id === daemonId);
        return d?.device_name || d?.daemon_id || null;
      })()
    : null;

  const select = (id: string | null) => {
    onUpdate?.({ dispatch_daemon_id: id, dispatch_daemon_label: null });
    onSelect?.(id);
    setOpen(false);
  };

  const renderDaemonItem = (d: Daemon, dimmed?: boolean) => {
    const auth = getRuntimeAuth(runtimes, d.id, provider);
    return (
      <PickerItem
        key={d.id}
        selected={daemonId === d.id}
        onClick={() => select(d.id)}
      >
        <span className={dimmed ? "text-muted-foreground" : ""}>
          {d.device_name || d.daemon_id}
        </span>
        {authBadge(auth)}
        {statusDot(d)}
      </PickerItem>
    );
  };

  return (
    <PropertyPicker
      open={open}
      onOpenChange={(v) => {
        setOpen(v);
        if (!v) setShowAll(false);
      }}
      width="w-52"
      align={align}
      triggerRender={triggerRender}
      trigger={
        customTrigger ?? (
          <span className={daemonId ? "" : "text-muted-foreground"}>
            {selectedName ?? "Environment"}
          </span>
        )
      }
    >
      {/* Compatible daemons */}
      {compatible.length > 0 && (
        <PickerSection label={provider ? "Compatible" : "Daemons"}>
          {compatible.map((d) => renderDaemonItem(d))}
        </PickerSection>
      )}

      {/* Toggle for incompatible */}
      {provider && incompatible.length > 0 && !showAll && (
        <button
          type="button"
          onClick={() => setShowAll(true)}
          className="flex w-full items-center gap-1.5 px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          <ChevronDown className="h-3 w-3" />
          Show {incompatible.length} incompatible
        </button>
      )}

      {/* Incompatible daemons */}
      {provider && showAll && incompatible.length > 0 && (
        <PickerSection label="Incompatible">
          {incompatible.map((d) => renderDaemonItem(d, true))}
        </PickerSection>
      )}

      {personal.length > 0 && (
        <PickerSection label="Your Daemons">
          {personal.map((d) => renderDaemonItem(d))}
        </PickerSection>
      )}
    </PropertyPicker>
  );
}
