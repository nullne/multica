"use client";

import { create } from "zustand";
import type { AgentRuntime, Daemon, WorkspaceProviderSettings } from "@/shared/types";
import { api } from "@/shared/api";
import { useWorkspaceStore } from "@/features/workspace";

interface RuntimeState {
  runtimes: AgentRuntime[];
  daemons: Daemon[];
  enabledProviders: string[];
  selectedDaemonId: string;
  fetching: boolean;
}

interface RuntimeActions {
  fetchAll: () => Promise<void>;
  setSelectedDaemonId: (id: string) => void;
  patchDaemon: (id: string, updates: Partial<Daemon>) => void;
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
  enabledProviders: [],
  selectedDaemonId: "",
  selectedId: "",
  fetching: true,

  fetchAll: async () => {
    const workspace = useWorkspaceStore.getState().workspace;
    if (!workspace) return;
    try {
      const [runtimes, daemons, providerSettings] = await Promise.all([
        api.listRuntimes({ workspace_id: workspace.id }),
        api.listDaemons(),
        api.getProviderConfig(workspace.id).catch((): WorkspaceProviderSettings => ({})),
      ]);
      const enabled = providerSettings.providers
        ? Object.entries(providerSettings.providers)
            .filter(([, c]) => c.enabled)
            .map(([name]) => name)
        : [];
      const { selectedDaemonId } = get();
      // Default selection only opens workspace-visible daemons. Personal
      // daemons must remain hidden until the user explicitly expands the
      // collapsible Personal section and clicks one, so we never auto-open
      // them on the detail pane.
      const defaultDaemon = daemons.find(
        (d) => d.workspace_visible && !d.archived_at,
      );
      set({
        runtimes,
        daemons,
        enabledProviders: enabled,
        fetching: false,
        selectedDaemonId:
          selectedDaemonId && daemons.some((d) => d.id === selectedDaemonId)
            ? selectedDaemonId
            : defaultDaemon?.id ?? "",
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

  patchDaemon: (id, updates) => {
    set((state) => ({
      daemons: state.daemons.map((d) =>
        d.id === id ? { ...d, ...updates } : d,
      ),
    }));
  },

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
