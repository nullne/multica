"use client";

import { create } from "zustand";
import type { AgentTask } from "@/shared/types/agent";

interface ActiveTaskState {
  /** Map from issue ID to its active AgentTask (null means no active task) */
  tasks: Map<string, AgentTask>;
  setTask: (issueId: string, task: AgentTask) => void;
  clearTask: (issueId: string) => void;
  setTasks: (tasks: AgentTask[]) => void;
}

export const useActiveTaskStore = create<ActiveTaskState>((set) => ({
  tasks: new Map(),

  setTask: (issueId, task) =>
    set((s) => {
      const next = new Map(s.tasks);
      next.set(issueId, task);
      return { tasks: next };
    }),

  clearTask: (issueId) =>
    set((s) => {
      const next = new Map(s.tasks);
      next.delete(issueId);
      return { tasks: next };
    }),

  setTasks: (tasks) =>
    set(() => {
      const next = new Map<string, AgentTask>();
      for (const t of tasks) next.set(t.issue_id, t);
      return { tasks: next };
    }),
}));
