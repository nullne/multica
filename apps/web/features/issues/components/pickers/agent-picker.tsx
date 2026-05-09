"use client";

import { useMemo, useState } from "react";
import { ChevronDown, Lock, Monitor } from "lucide-react";
import type { Agent } from "@/shared/types";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { useRuntimeStore } from "@/features/runtimes";
import { useIssueDefaultsStore } from "@/features/issues/stores/defaults-store";
import { pickBestProvider } from "@/features/issues/utils/dispatch";
import { ActorAvatar } from "@/components/common/actor-avatar";
import {
  PropertyPicker,
  PickerItem,
  PickerSection,
  PickerEmpty,
} from "./property-picker";
import { AgentDispatchConfirm, canAssignAgent } from "./assignee-picker";

export interface AgentPickerValue {
  agentId: string | null;
  daemonId: string | null;
  provider: string | null;
}

/**
 * Compact label for an agent dispatch — shows the agent name on the first
 * line and provider · environment as secondary text underneath. Use as the
 * trigger for `AgentPicker`/`AssigneePicker` in space-constrained surfaces
 * where separate provider/environment pills would feel crowded.
 */
export function AgentDispatchTriggerContent({
  value,
  placeholder = "Pick agent",
}: {
  value: AgentPickerValue;
  placeholder?: string;
}) {
  const agents = useWorkspaceStore((s) => s.agents);
  const daemons = useRuntimeStore((s) => s.daemons);

  const agent = value.agentId ? agents.find((a) => a.id === value.agentId) : null;
  const daemon = value.daemonId ? daemons.find((d) => d.id === value.daemonId) : null;
  const daemonName = daemon?.device_name || daemon?.daemon_id || null;
  const provider = value.provider ?? null;
  const secondary =
    [provider ? <span key="p" className="capitalize">{provider}</span> : null,
     daemonName
       ? (
          <span key="d" className="inline-flex items-center gap-0.5 min-w-0">
            <Monitor className="h-2.5 w-2.5 shrink-0" />
            <span className="truncate">{daemonName}</span>
          </span>
        )
       : null]
      .filter(Boolean);

  if (!agent) {
    return (
      <>
        <span className="text-muted-foreground">
          {value.agentId ? "(deleted agent)" : placeholder}
        </span>
        <ChevronDown className="ml-auto h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      </>
    );
  }

  return (
    <>
      <ActorAvatar actorType="agent" actorId={agent.id} size={18} />
      <span className="min-w-0 flex flex-col gap-0.5 leading-tight text-left">
        <span className="text-xs truncate">{agent.name}</span>
        {secondary.length > 0 && (
          <span className="flex items-center gap-1 text-[10px] text-muted-foreground leading-none min-w-0">
            {secondary.flatMap((node, i) =>
              i === 0
                ? [node]
                : [
                    <span key={`sep-${i}`} className="shrink-0">·</span>,
                    node,
                  ],
            )}
          </span>
        )}
      </span>
      <ChevronDown className="ml-auto h-3.5 w-3.5 shrink-0 text-muted-foreground" />
    </>
  );
}

/**
 * Agent-only variant of the assignee picker — used in automation entry points
 * (webhook actions) where assigning a non-agent is not meaningful. Shares the
 * same in-popover dispatch confirmation step as the full assignee picker so
 * the agent + provider + environment always travel together.
 */
export function AgentPicker({
  value,
  onChange,
  trigger,
  triggerRender,
  align = "start",
  open: controlledOpen,
  onOpenChange: controlledOnOpenChange,
}: {
  value: AgentPickerValue;
  onChange: (next: AgentPickerValue) => void;
  trigger?: React.ReactNode;
  triggerRender?: React.ReactElement;
  align?: "start" | "center" | "end";
  open?: boolean;
  onOpenChange?: (v: boolean) => void;
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const open = controlledOpen ?? internalOpen;
  const setOpen = controlledOnOpenChange ?? setInternalOpen;
  const [filter, setFilter] = useState("");
  const [pendingAgentId, setPendingAgentId] = useState<string | null>(null);

  const user = useAuthStore((s) => s.user);
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);

  const currentMember = members.find((m) => m.user_id === user?.id);
  const memberRole = currentMember?.role;

  const query = filter.toLowerCase();
  const filteredAgents = agents.filter(
    (a) => !a.archived_at && a.name.toLowerCase().includes(query),
  );

  const pendingAgent: Agent | null = pendingAgentId
    ? agents.find((a) => a.id === pendingAgentId) ?? null
    : null;

  const initialConfirmDispatch = useMemo(() => {
    if (!pendingAgent)
      return { daemonId: null as string | null, provider: null as string | null };
    const saved = useIssueDefaultsStore.getState().getAgentDispatch(pendingAgent.id);
    const runtimes = useRuntimeStore.getState().runtimes;
    const daemonId =
      pendingAgent.default_daemon_id ?? saved?.daemonId ?? null;
    const provider =
      saved?.provider ??
      pickBestProvider(pendingAgent.providers ?? [], runtimes, daemonId ?? undefined) ??
      null;
    return { daemonId, provider };
  }, [pendingAgent]);

  const handleConfirm = (daemonId: string | null, provider: string | null) => {
    if (!pendingAgent) return;
    useIssueDefaultsStore.getState().setAgentDispatch(pendingAgent.id, {
      daemonId: daemonId ?? undefined,
      provider: provider ?? undefined,
    });
    onChange({ agentId: pendingAgent.id, daemonId, provider });
    setPendingAgentId(null);
    setOpen(false);
  };

  return (
    <PropertyPicker
      open={open}
      onOpenChange={(v: boolean) => {
        setOpen(v);
        if (!v) {
          setFilter("");
          setPendingAgentId(null);
        }
      }}
      width={pendingAgent ? "w-64" : "w-52"}
      align={align}
      searchable={!pendingAgent}
      searchPlaceholder="Pick agent..."
      onSearchChange={setFilter}
      triggerRender={triggerRender}
      trigger={trigger}
    >
      {pendingAgent ? (
        <AgentDispatchConfirm
          key={pendingAgent.id}
          agent={pendingAgent}
          initialDaemonId={initialConfirmDispatch.daemonId}
          initialProvider={initialConfirmDispatch.provider}
          daemonLocked={!!pendingAgent.default_daemon_id}
          onBack={() => setPendingAgentId(null)}
          onConfirm={handleConfirm}
        />
      ) : filteredAgents.length === 0 ? (
        <PickerEmpty />
      ) : (
        <PickerSection label="Agents">
          {filteredAgents.map((a) => {
            const allowed = canAssignAgent(a, user?.id, memberRole);
            return (
              <PickerItem
                key={a.id}
                selected={value.agentId === a.id}
                disabled={!allowed}
                onClick={() => {
                  if (!allowed) return;
                  setPendingAgentId(a.id);
                }}
              >
                <ActorAvatar actorType="agent" actorId={a.id} size={18} />
                <span className={allowed ? "" : "text-muted-foreground"}>
                  {a.name}
                </span>
                {a.visibility === "private" && (
                  <Lock className="ml-auto h-3 w-3 text-muted-foreground" />
                )}
              </PickerItem>
            );
          })}
        </PickerSection>
      )}
    </PropertyPicker>
  );
}
