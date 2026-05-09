import type {
  Agent,
  AgentRuntime,
  IssueAssigneeType,
  UpdateIssueRequest,
} from "@/shared/types";

export function pickBestProvider(
  agentProviders: string[],
  runtimes: AgentRuntime[],
  daemonId: string | undefined,
): string | undefined {
  if (agentProviders.length === 0) return undefined;
  if (!daemonId) return agentProviders[0];
  const ready = agentProviders.find((p) => {
    const rt = runtimes.find(
      (r) => r.daemon_ref === daemonId && r.provider === p,
    );
    return rt?.auth_status === "ready";
  });
  return ready ?? agentProviders[0];
}

export interface ResolveAssigneeChangeOpts {
  type?: IssueAssigneeType | null;
  id?: string | null;
  agents: Agent[];
  runtimes: AgentRuntime[];
  /** Optional saved defaults for the chosen agent (daemon/provider) */
  savedDispatch?: { daemonId?: string; provider?: string };
  /** Currently set verifier on the issue, if any */
  currentVerifierId?: string | null;
}

/**
 * Computes the full set of fields that must move together when the assignee
 * changes: assignee, dispatch hints (provider/daemon/label) and verifier.
 *
 * Agent → resolves dispatch from agent.default_daemon_id / saved defaults /
 * the agent's first compatible provider. Drops the verifier when it equals
 * the new assignee.
 *
 * Member or unassigned → clears all agent-only dispatch and verifier fields.
 */
export function resolveAssigneeChange(
  opts: ResolveAssigneeChangeOpts,
): Partial<UpdateIssueRequest> {
  const { type, id, agents, runtimes, savedDispatch, currentVerifierId } = opts;

  if (type === "agent" && id) {
    const agent = agents.find((a) => a.id === id);
    const daemonId =
      agent?.default_daemon_id ?? savedDispatch?.daemonId ?? null;
    const provider =
      savedDispatch?.provider ??
      pickBestProvider(agent?.providers ?? [], runtimes, daemonId ?? undefined) ??
      null;

    const patch: Partial<UpdateIssueRequest> = {
      assignee_type: "agent",
      assignee_id: id,
      dispatch_provider: provider,
      dispatch_daemon_id: daemonId,
      dispatch_daemon_label: null,
    };
    if (currentVerifierId === id) {
      patch.verifier_agent_id = null;
      patch.max_verification_rounds = null;
    }
    return patch;
  }

  return {
    assignee_type: type ?? null,
    assignee_id: id ?? null,
    dispatch_provider: null,
    dispatch_daemon_id: null,
    dispatch_daemon_label: null,
    verifier_agent_id: null,
    max_verification_rounds: null,
  };
}
