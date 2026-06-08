"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";
import type { Routine } from "@/shared/types";

const logger = createLogger("routine-store");

interface RoutineState {
  routines: Routine[];
  loading: boolean;
  selectedId: string | null;
}

interface RoutineActions {
  fetch: () => Promise<void>;
  setRoutines: (routines: Routine[]) => void;
  select: (id: string | null) => void;
  patch: (id: string, updates: Partial<Routine>) => void;
  remove: (id: string) => void;
  reset: () => void;
}

type RoutineStore = RoutineState & RoutineActions;

export const useRoutineStore = create<RoutineStore>((set, get) => ({
  routines: [],
  loading: true,
  selectedId: null,

  fetch: async () => {
    const workspaceId = api.getWorkspaceId();
    const isInitialLoad = get().routines.length === 0;
    if (isInitialLoad) set({ loading: true });
    try {
      const routines = await api.listRoutines();
      if (api.getWorkspaceId() !== workspaceId) {
        logger.debug("skip stale fetch result", workspaceId);
        return;
      }
      set((state) => ({
        routines,
        loading: false,
        selectedId:
          state.selectedId && routines.some((routine) => routine.id === state.selectedId)
            ? state.selectedId
            : null,
      }));
    } catch (error) {
      if (api.getWorkspaceId() !== workspaceId) {
        logger.debug("skip stale fetch error", workspaceId);
        return;
      }
      logger.error("fetch failed", error);
      toast.error("Failed to load routines");
      if (isInitialLoad) set({ loading: false });
    }
  },

  setRoutines: (routines) =>
    set((state) => ({
      routines,
      selectedId:
        state.selectedId && routines.some((routine) => routine.id === state.selectedId)
          ? state.selectedId
          : null,
    })),

  select: (id) => set({ selectedId: id }),

  patch: (id, updates) =>
    set((state) => ({
      routines: state.routines.map((routine) =>
        routine.id === id ? { ...routine, ...updates } : routine,
      ),
    })),

  remove: (id) =>
    set((state) => ({
      routines: state.routines.filter((routine) => routine.id !== id),
      selectedId: state.selectedId === id ? null : state.selectedId,
    })),

  reset: () => set({ routines: [], loading: true, selectedId: null }),
}));
