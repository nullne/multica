"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

interface RecentsPrefsState {
  collapsedWorkspaceIds: string[];
  pinnedWorkspaceIds: string[];
  toggleCollapsed: (workspaceId: string) => void;
  togglePinned: (workspaceId: string) => void;
  movePinned: (workspaceId: string, direction: "up" | "down") => void;
  isCollapsed: (workspaceId: string) => boolean;
  isPinned: (workspaceId: string) => boolean;
}

export const useRecentsPrefsStore = create<RecentsPrefsState>()(
  persist(
    (set, get) => ({
      collapsedWorkspaceIds: [],
      pinnedWorkspaceIds: [],

      toggleCollapsed: (workspaceId) =>
        set((s) => ({
          collapsedWorkspaceIds: s.collapsedWorkspaceIds.includes(workspaceId)
            ? s.collapsedWorkspaceIds.filter((id) => id !== workspaceId)
            : [...s.collapsedWorkspaceIds, workspaceId],
        })),

      togglePinned: (workspaceId) =>
        set((s) => ({
          pinnedWorkspaceIds: s.pinnedWorkspaceIds.includes(workspaceId)
            ? s.pinnedWorkspaceIds.filter((id) => id !== workspaceId)
            : [...s.pinnedWorkspaceIds, workspaceId],
        })),

      movePinned: (workspaceId, direction) =>
        set((s) => {
          const index = s.pinnedWorkspaceIds.indexOf(workspaceId);
          if (index === -1) return s;
          const target = direction === "up" ? index - 1 : index + 1;
          if (target < 0 || target >= s.pinnedWorkspaceIds.length) return s;
          const next = [...s.pinnedWorkspaceIds];
          const moving = next[index]!;
          next[index] = next[target]!;
          next[target] = moving;
          return { pinnedWorkspaceIds: next };
        }),

      isCollapsed: (workspaceId) =>
        get().collapsedWorkspaceIds.includes(workspaceId),

      isPinned: (workspaceId) => get().pinnedWorkspaceIds.includes(workspaceId),
    }),
    { name: "multica_recents_prefs" },
  ),
);
