"use client";

import { useState } from "react";
import { Lock, ShieldCheck, ShieldOff } from "lucide-react";
import type { UpdateIssueRequest } from "@/shared/types";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { canAssignAgent } from "./assignee-picker";
import {
  PropertyPicker,
  PickerItem,
  PickerSection,
  PickerEmpty,
} from "./property-picker";

export function VerifierPicker({
  verifierAgentId,
  assigneeId,
  onUpdate,
  trigger: customTrigger,
  triggerRender,
  open: controlledOpen,
  onOpenChange: controlledOnOpenChange,
  align,
}: {
  verifierAgentId: string | null;
  assigneeId: string | null;
  onUpdate: (updates: Partial<UpdateIssueRequest>) => void;
  trigger?: React.ReactNode;
  triggerRender?: React.ReactElement;
  open?: boolean;
  onOpenChange?: (v: boolean) => void;
  align?: "start" | "center" | "end";
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const open = controlledOpen ?? internalOpen;
  const setOpen = controlledOnOpenChange ?? setInternalOpen;
  const [filter, setFilter] = useState("");
  const user = useAuthStore((s) => s.user);
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);
  const { getActorName } = useActorName();

  const currentMember = members.find((m) => m.user_id === user?.id);
  const memberRole = currentMember?.role;

  const query = filter.toLowerCase();
  const filteredAgents = agents.filter(
    (a) => !a.archived_at && a.name.toLowerCase().includes(query) && a.id !== assigneeId,
  );

  const triggerLabel = verifierAgentId
    ? getActorName("agent", verifierAgentId)
    : "No verifier";

  return (
    <PropertyPicker
      open={open}
      onOpenChange={(v: boolean) => {
        setOpen(v);
        if (!v) setFilter("");
      }}
      width="w-52"
      align={align}
      searchable
      searchPlaceholder="Select verifier..."
      onSearchChange={setFilter}
      triggerRender={triggerRender}
      trigger={
        customTrigger ? customTrigger : verifierAgentId ? (
          <>
            <ActorAvatar actorType="agent" actorId={verifierAgentId} size={18} />
            <span className="truncate">{triggerLabel}</span>
          </>
        ) : (
          <span className="text-muted-foreground">No verifier</span>
        )
      }
    >
      <PickerItem
        selected={!verifierAgentId}
        onClick={() => {
          onUpdate({ verifier_agent_id: null });
          setOpen(false);
        }}
      >
        <ShieldOff className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-muted-foreground">No verifier</span>
      </PickerItem>

      {filteredAgents.length > 0 && (
        <PickerSection label="Agents">
          {filteredAgents.map((a) => {
            const allowed = canAssignAgent(a, user?.id, memberRole);
            return (
              <PickerItem
                key={a.id}
                selected={verifierAgentId === a.id}
                disabled={!allowed}
                onClick={() => {
                  if (!allowed) return;
                  onUpdate({ verifier_agent_id: a.id });
                  setOpen(false);
                }}
              >
                <ActorAvatar actorType="agent" actorId={a.id} size={18} />
                <span className={allowed ? "" : "text-muted-foreground"}>{a.name}</span>
                {a.visibility === "private" && (
                  <Lock className="ml-auto h-3 w-3 text-muted-foreground" />
                )}
              </PickerItem>
            );
          })}
        </PickerSection>
      )}

      {filteredAgents.length === 0 && filter && <PickerEmpty />}
    </PropertyPicker>
  );
}

export function VerifierIcon({ className }: { className?: string }) {
  return <ShieldCheck className={className} />;
}
