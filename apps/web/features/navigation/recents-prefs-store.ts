"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

interface RecentsPrefsState {
  collapsedWorkspaceIds: string[];
  pinnedIssueIds: string[];
  toggleCollapsed: (workspaceId: string) => void;
  togglePinnedIssue: (issueId: string) => void;
  isCollapsed: (workspaceId: string) => boolean;
  isIssuePinned: (issueId: string) => boolean;
}

export const useRecentsPrefsStore = create<RecentsPrefsState>()(
  persist(
    (set, get) => ({
      collapsedWorkspaceIds: [],
      pinnedIssueIds: [],

      toggleCollapsed: (workspaceId) =>
        set((s) => ({
          collapsedWorkspaceIds: s.collapsedWorkspaceIds.includes(workspaceId)
            ? s.collapsedWorkspaceIds.filter((id) => id !== workspaceId)
            : [...s.collapsedWorkspaceIds, workspaceId],
        })),

      togglePinnedIssue: (issueId) =>
        set((s) => ({
          pinnedIssueIds: s.pinnedIssueIds.includes(issueId)
            ? s.pinnedIssueIds.filter((id) => id !== issueId)
            : [...s.pinnedIssueIds, issueId],
        })),

      isCollapsed: (workspaceId) =>
        get().collapsedWorkspaceIds.includes(workspaceId),

      isIssuePinned: (issueId) => get().pinnedIssueIds.includes(issueId),
    }),
    { name: "multica_recents_prefs" },
  ),
);
