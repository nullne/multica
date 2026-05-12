"use client";

import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, ChevronDown, Cpu, Lock, Monitor, UserMinus } from "lucide-react";
import type {
  Agent,
  AgentRuntime,
  Daemon,
  IssueAssigneeType,
  ProviderAuthStatus,
  UpdateIssueRequest,
} from "@/shared/types";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { useRuntimeStore } from "@/features/runtimes";
import { useIssueDefaultsStore } from "@/features/issues/stores/defaults-store";
import { getAgentDispatchDefaults, hasCompleteAgentDispatchDefaults, resolveAssigneeChange } from "@/features/issues/utils/dispatch";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { Button } from "@/components/ui/button";
import {
  PropertyPicker,
  PickerItem,
  PickerSection,
  PickerEmpty,
} from "./property-picker";

export function canAssignAgent(agent: Agent, userId: string | undefined, memberRole: string | undefined): boolean {
  if (agent.visibility !== "private") return true;
  if (agent.owner_id === userId) return true;
  if (memberRole === "owner" || memberRole === "admin") return true;
  return false;
}

function authBadge(status: ProviderAuthStatus) {
  if (status === "ready")
    return <span className="ml-auto h-1.5 w-1.5 shrink-0 rounded-full bg-green-500" />;
  if (status === "unauthenticated")
    return <span className="ml-auto text-[10px] text-amber-500">unauth</span>;
  if (status === "not_installed")
    return <span className="ml-auto text-[10px] text-muted-foreground">not installed</span>;
  return null;
}

function getProviderAuth(
  runtimes: AgentRuntime[],
  daemonId: string | null | undefined,
  provider: string | null | undefined,
): ProviderAuthStatus {
  if (!daemonId || !provider) return "unknown";
  const rt = runtimes.find(
    (r) => r.daemon_ref === daemonId && r.provider === provider,
  );
  return rt?.auth_status ?? "not_installed";
}

function classifyDaemons(
  daemons: Daemon[],
  runtimes: AgentRuntime[],
  provider: string | null | undefined,
) {
  if (!provider) return { compatible: daemons, incompatible: [] as Daemon[] };
  const compatible: Daemon[] = [];
  const incompatible: Daemon[] = [];
  for (const d of daemons) {
    const hasRuntime = runtimes.some(
      (r) => r.daemon_ref === d.id && r.provider === provider,
    );
    if (hasRuntime) compatible.push(d);
    else incompatible.push(d);
  }
  return { compatible, incompatible };
}

// ---------------------------------------------------------------------------
// AgentDispatchConfirm — in-popover confirmation step shown after the user
// selects an agent assignee. Lets them review/adjust provider + environment
// before committing.
// ---------------------------------------------------------------------------

export function AgentDispatchConfirm({
  agent,
  initialDaemonId,
  initialProvider,
  daemonLocked,
  onBack,
  onConfirm,
}: {
  agent: Agent;
  initialDaemonId: string | null;
  initialProvider: string | null;
  daemonLocked: boolean;
  onBack: () => void;
  onConfirm: (daemonId: string | null, provider: string | null) => void;
}) {
  const fetchAllRuntimes = useRuntimeStore((s) => s.fetchAll);
  const daemons = useRuntimeStore((s) => s.daemons);
  const runtimes = useRuntimeStore((s) => s.runtimes);

  const [daemonId, setDaemonId] = useState<string | null>(initialDaemonId);
  const [provider, setProvider] = useState<string | null>(initialProvider);
  const [daemonOpen, setDaemonOpen] = useState(false);
  const [providerOpen, setProviderOpen] = useState(false);
  const [showAllDaemons, setShowAllDaemons] = useState(false);

  useEffect(() => {
    fetchAllRuntimes();
  }, [fetchAllRuntimes]);

  const providers = agent.providers ?? [];
  const { compatible, incompatible } = useMemo(
    () => classifyDaemons(daemons, runtimes, provider),
    [daemons, runtimes, provider],
  );

  const selectedDaemon = daemonId ? daemons.find((d) => d.id === daemonId) : null;

  return (
    <div className="flex flex-col">
      <div className="flex items-center gap-2 border-b px-2 py-1.5">
        <button
          type="button"
          onClick={onBack}
          className="flex h-5 w-5 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
          aria-label="Back"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
        </button>
        <ActorAvatar actorType="agent" actorId={agent.id} size={18} />
        <span className="truncate text-sm font-medium">{agent.name}</span>
      </div>

      <div className="space-y-2 p-2">
        <div>
          <div className="px-1 pb-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
            Environment
          </div>
          {daemonLocked ? (
            <div className="flex items-center gap-1.5 rounded-md border bg-muted/40 px-2 py-1.5 text-sm">
              <Monitor className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="truncate">
                {selectedDaemon?.device_name || selectedDaemon?.daemon_id || "Default"}
              </span>
              <Lock className="ml-auto h-3 w-3 text-muted-foreground" />
            </div>
          ) : (
            <PropertyPicker
              open={daemonOpen}
              onOpenChange={(v) => {
                setDaemonOpen(v);
                if (!v) setShowAllDaemons(false);
              }}
              align="start"
              width="w-52"
              triggerRender={
                <button
                  type="button"
                  className="flex w-full items-center gap-1.5 rounded-md border px-2 py-1.5 text-sm hover:bg-accent transition-colors"
                />
              }
              trigger={
                <>
                  <Monitor className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className={selectedDaemon ? "truncate" : "text-muted-foreground"}>
                    {selectedDaemon?.device_name || selectedDaemon?.daemon_id || "Pick environment"}
                  </span>
                  <ChevronDown className="ml-auto h-3 w-3 text-muted-foreground" />
                </>
              }
            >
              {compatible.length > 0 && (
                <PickerSection label={provider ? "Compatible" : "Daemons"}>
                  {compatible.map((d) => (
                    <PickerItem
                      key={d.id}
                      selected={daemonId === d.id}
                      onClick={() => {
                        setDaemonId(d.id);
                        setDaemonOpen(false);
                      }}
                    >
                      <span className="truncate">
                        {d.device_name || d.daemon_id}
                      </span>
                      <span
                        className={`ml-auto h-1.5 w-1.5 shrink-0 rounded-full ${d.status === "online" ? "bg-green-500" : "bg-muted-foreground/40"}`}
                      />
                    </PickerItem>
                  ))}
                </PickerSection>
              )}
              {provider && incompatible.length > 0 && !showAllDaemons && (
                <button
                  type="button"
                  onClick={() => setShowAllDaemons(true)}
                  className="flex w-full items-center gap-1.5 px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  <ChevronDown className="h-3 w-3" />
                  Show {incompatible.length} incompatible
                </button>
              )}
              {provider && showAllDaemons && incompatible.length > 0 && (
                <PickerSection label="Incompatible">
                  {incompatible.map((d) => (
                    <PickerItem
                      key={d.id}
                      selected={daemonId === d.id}
                      onClick={() => {
                        setDaemonId(d.id);
                        setDaemonOpen(false);
                      }}
                    >
                      <span className="truncate text-muted-foreground">
                        {d.device_name || d.daemon_id}
                      </span>
                      <span
                        className={`ml-auto h-1.5 w-1.5 shrink-0 rounded-full ${d.status === "online" ? "bg-green-500" : "bg-muted-foreground/40"}`}
                      />
                    </PickerItem>
                  ))}
                </PickerSection>
              )}
            </PropertyPicker>
          )}
        </div>

        <div>
          <div className="px-1 pb-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
            Provider
          </div>
          <PropertyPicker
            open={providerOpen}
            onOpenChange={setProviderOpen}
            align="start"
            width="w-52"
            triggerRender={
              <button
                type="button"
                className="flex w-full items-center gap-1.5 rounded-md border px-2 py-1.5 text-sm hover:bg-accent transition-colors"
              />
            }
            trigger={
              <>
                <Cpu className="h-3.5 w-3.5 text-muted-foreground" />
                <span className={provider ? "capitalize" : "text-muted-foreground"}>
                  {provider ?? "Pick provider"}
                </span>
                {provider && authBadge(getProviderAuth(runtimes, daemonId, provider))}
                <ChevronDown className="ml-auto h-3 w-3 text-muted-foreground" />
              </>
            }
          >
            {providers.length === 0 ? (
              <PickerEmpty />
            ) : (
              providers.map((p) => {
                const auth = getProviderAuth(runtimes, daemonId, p);
                return (
                  <PickerItem
                    key={p}
                    selected={provider === p}
                    onClick={() => {
                      setProvider(p);
                      setProviderOpen(false);
                    }}
                  >
                    <span className="capitalize flex-1 truncate">{p}</span>
                    {authBadge(auth)}
                  </PickerItem>
                );
              })
            )}
          </PropertyPicker>
        </div>
      </div>

      <div className="flex items-center justify-end gap-1.5 border-t p-2">
        <Button variant="ghost" size="xs" onClick={onBack}>
          Cancel
        </Button>
        <Button size="xs" onClick={() => onConfirm(daemonId, provider)}>
          Assign agent
        </Button>
      </div>
    </div>
  );
}

export function AssigneePicker({
  assigneeType,
  assigneeId,
  verifierAgentId,
  onUpdate,
  trigger: customTrigger,
  triggerRender,
  open: controlledOpen,
  onOpenChange: controlledOnOpenChange,
  pendingAgentId: controlledPendingAgentId,
  onPendingAgentIdChange,
  align,
}: {
  assigneeType: IssueAssigneeType | null;
  assigneeId: string | null;
  /** Current verifier on the issue — used to clear it if the new assignee is the same agent. */
  verifierAgentId?: string | null;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
  trigger?: React.ReactNode;
  triggerRender?: React.ReactElement;
  open?: boolean;
  onOpenChange?: (v: boolean) => void;
  /**
   * Optional controlled value for the in-popover agent confirmation step.
   * Set to an agent ID to open the picker straight into the dispatch
   * confirmation view for that agent (used when an external trigger like
   * a dropdown menu picks the agent).
   */
  pendingAgentId?: string | null;
  onPendingAgentIdChange?: (id: string | null) => void;
  align?: "start" | "center" | "end";
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const open = controlledOpen ?? internalOpen;
  const setOpen = controlledOnOpenChange ?? setInternalOpen;
  const [filter, setFilter] = useState("");
  const [internalPendingAgentId, setInternalPendingAgentId] = useState<string | null>(null);
  const pendingAgentId =
    controlledPendingAgentId !== undefined
      ? controlledPendingAgentId
      : internalPendingAgentId;
  const setPendingAgentId = (id: string | null) => {
    if (controlledPendingAgentId === undefined) {
      setInternalPendingAgentId(id);
    }
    onPendingAgentIdChange?.(id);
  };
  const user = useAuthStore((s) => s.user);
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);
  const { getActorName } = useActorName();

  const currentMember = members.find((m) => m.user_id === user?.id);
  const memberRole = currentMember?.role;

  const query = filter.toLowerCase();
  const filteredMembers = members.filter((m) =>
    m.kind !== "bot" && m.name.toLowerCase().includes(query),
  );
  const filteredAgents = agents.filter((a) =>
    !a.archived_at && a.name.toLowerCase().includes(query),
  );

  const isSelected = (type: string, id: string) =>
    assigneeType === type && assigneeId === id;

  const triggerLabel =
    assigneeType && assigneeId
      ? getActorName(assigneeType, assigneeId)
      : "Unassigned";

  const pendingAgent =
    pendingAgentId ? agents.find((a) => a.id === pendingAgentId) ?? null : null;

  const initialConfirmDispatch = useMemo(() => {
    if (!pendingAgent) return { daemonId: null as string | null, provider: null as string | null };
    const saved = useIssueDefaultsStore.getState().getAgentDispatch(pendingAgent.id);
    const runtimes = useRuntimeStore.getState().runtimes;
    return getAgentDispatchDefaults(pendingAgent, runtimes, saved);
  }, [pendingAgent]);

  const emitMemberOrUnassigned = (type: IssueAssigneeType | null, id: string | null) => {
    onUpdate(
      resolveAssigneeChange({
        type,
        id,
        agents,
        runtimes: useRuntimeStore.getState().runtimes,
        currentVerifierId: verifierAgentId ?? null,
      }),
    );
  };

  const confirmAgent = (daemonId: string | null, provider: string | null) => {
    if (!pendingAgent) return;
    const patch: Partial<UpdateIssueRequest> = {
      assignee_type: "agent",
      assignee_id: pendingAgent.id,
      dispatch_provider: provider,
      dispatch_daemon_id: daemonId,
      dispatch_daemon_label: null,
    };
    if (verifierAgentId === pendingAgent.id) {
      patch.verifier_agent_id = null;
      patch.max_verification_rounds = null;
    }
    useIssueDefaultsStore.getState().setAgentDispatch(pendingAgent.id, {
      daemonId: daemonId ?? undefined,
      provider: provider ?? undefined,
    });
    onUpdate(patch);
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
      searchPlaceholder="Assign to..."
      onSearchChange={setFilter}
      triggerRender={triggerRender}
      trigger={
        customTrigger ? customTrigger : assigneeType && assigneeId ? (
          <>
            <ActorAvatar actorType={assigneeType} actorId={assigneeId} size={18} />
            <span className="truncate">{triggerLabel}</span>
          </>
        ) : (
          <span className="text-muted-foreground">Unassigned</span>
        )
      }
    >
      {pendingAgent ? (
        <AgentDispatchConfirm
          key={pendingAgent.id}
          agent={pendingAgent}
          initialDaemonId={initialConfirmDispatch.daemonId}
          initialProvider={initialConfirmDispatch.provider}
          daemonLocked={!!pendingAgent.default_daemon_id}
          onBack={() => setPendingAgentId(null)}
          onConfirm={confirmAgent}
        />
      ) : (
        <>
          {/* Unassigned option */}
          <PickerItem
            selected={!assigneeType && !assigneeId}
            onClick={() => {
              emitMemberOrUnassigned(null, null);
              setOpen(false);
            }}
          >
            <UserMinus className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-muted-foreground">Unassigned</span>
          </PickerItem>

          {/* Members */}
          {filteredMembers.length > 0 && (
            <PickerSection label="Members">
              {filteredMembers.map((m) => (
                <PickerItem
                  key={m.user_id}
                  selected={isSelected("member", m.user_id)}
                  onClick={() => {
                    emitMemberOrUnassigned("member", m.user_id);
                    setOpen(false);
                  }}
                >
                  <ActorAvatar actorType="member" actorId={m.user_id} size={18} />
                  <span>{m.name}</span>
                </PickerItem>
              ))}
            </PickerSection>
          )}

          {/* Agents — clicking switches to the dispatch confirmation step */}
          {filteredAgents.length > 0 && (
            <PickerSection label="Agents">
              {filteredAgents.map((a) => {
                const allowed = canAssignAgent(a, user?.id, memberRole);
                const completeDefaults = hasCompleteAgentDispatchDefaults(a);
                return (
                  <PickerItem
                    key={a.id}
                    selected={isSelected("agent", a.id)}
                    disabled={!allowed || !completeDefaults}
                    onClick={() => {
                      if (!allowed || !completeDefaults) return;
                      setPendingAgentId(a.id);
                    }}
                  >
                    <ActorAvatar actorType="agent" actorId={a.id} size={18} />
                    <span className={allowed && completeDefaults ? "" : "text-muted-foreground"}>{a.name}</span>
                    {!completeDefaults && (
                      <span className="ml-auto text-[10px] text-muted-foreground">
                        Configure defaults
                      </span>
                    )}
                    {a.visibility === "private" && (
                      <Lock className="ml-auto h-3 w-3 text-muted-foreground" />
                    )}
                  </PickerItem>
                );
              })}
            </PickerSection>
          )}

          {filteredMembers.length === 0 &&
            filteredAgents.length === 0 &&
            filter && <PickerEmpty />}
        </>
      )}
    </PropertyPicker>
  );
}
