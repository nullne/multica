"use client";

import { create } from "zustand";
import type { AgentRuntime, Daemon } from "@/shared/types";
import { api } from "@/shared/api";
import { useWorkspaceStore } from "@/features/workspace";

interface RuntimeState {
  runtimes: AgentRuntime[];
  daemons: Daemon[];
  selectedDaemonId: string;
  fetching: boolean;
}

interface RuntimeActions {
  fetchAll: () => Promise<void>;
  setSelectedDaemonId: (id: string) => void;
  patchRuntime: (id: string, updates: Partial<AgentRuntime>) => void;
  setRuntimes: (runtimes: AgentRuntime[]) => void;
  /** @deprecated use fetchAll */
  fetchRuntimes: () => Promise<void>;
  /** @deprecated use selectedDaemonId */
  selectedId: string;
  setSelectedId: (id: string) => void;
}

type RuntimeStore = RuntimeState & RuntimeActions;

export const useRuntimeStore = create<RuntimeStore>((set, get) => ({
  runtimes: [],
  daemons: [],
  selectedDaemonId: "",
  selectedId: "",
  fetching: true,

  fetchAll: async () => {
    const workspace = useWorkspaceStore.getState().workspace;
    if (!workspace) return;
    try {
      const [runtimes, daemons] = await Promise.all([
        api.listRuntimes({ workspace_id: workspace.id }),
        api.listDaemons(),
      ]);
      const { selectedDaemonId } = get();
      set({
        runtimes,
        daemons,
        fetching: false,
        selectedDaemonId:
          selectedDaemonId && daemons.some((d) => d.id === selectedDaemonId)
            ? selectedDaemonId
            : daemons[0]?.id ?? "",
        selectedId: runtimes[0]?.id ?? "",
      });
    } catch {
      set({ fetching: false });
    }
  },

  fetchRuntimes: async () => {
    return get().fetchAll();
  },

  setSelectedDaemonId: (id) => set({ selectedDaemonId: id }),
  setSelectedId: (id) => set({ selectedId: id }),

  patchRuntime: (id, updates) => {
    set((state) => ({
      runtimes: state.runtimes.map((r) =>
        r.id === id ? { ...r, ...updates } : r,
      ),
    }));
  },

  setRuntimes: (runtimes) => {
    const { selectedId } = get();
    set({
      runtimes,
      selectedId:
        selectedId && runtimes.some((r) => r.id === selectedId)
          ? selectedId
          : runtimes[0]?.id ?? "",
    });
  },
}));
