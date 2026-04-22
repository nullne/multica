import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { IssueAssigneeType } from "@/shared/types";

interface AgentDispatchDefaults {
  daemonId?: string;
  provider?: string;
}

interface IssueDefaultsStore {
  assigneeType?: IssueAssigneeType;
  assigneeId?: string;
  agentDispatches: Record<string, AgentDispatchDefaults>;
  setAssigneeDefaults: (type?: IssueAssigneeType, id?: string) => void;
  setAgentDispatch: (agentId: string, dispatch: AgentDispatchDefaults) => void;
  getAgentDispatch: (agentId: string) => AgentDispatchDefaults | undefined;
}

export const useIssueDefaultsStore = create<IssueDefaultsStore>()(
  persist(
    (set, get) => ({
      assigneeType: undefined,
      assigneeId: undefined,
      agentDispatches: {},

      setAssigneeDefaults: (type, id) =>
        set({ assigneeType: type, assigneeId: id }),

      setAgentDispatch: (agentId, dispatch) =>
        set((s) => ({
          agentDispatches: { ...s.agentDispatches, [agentId]: dispatch },
        })),

      getAgentDispatch: (agentId) => get().agentDispatches[agentId],
    }),
    { name: "multica_issue_defaults" },
  ),
);
