"use client";

import { create } from "zustand";
import type { Issue } from "@/shared/types";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";

const logger = createLogger("recents-store");

const RECENT_LIMIT_PER_WORKSPACE = 50;

interface RecentsState {
  issues: Issue[];
  loading: boolean;
  loadedWorkspaceIds: string[];
  inFlightKey: string | null;
}

interface RecentsActions {
  fetch: (workspaceIds: string[]) => Promise<void>;
  upsertIssue: (issue: Issue) => void;
  removeIssue: (issueId: string) => void;
  clear: () => void;
}

type RecentsStore = RecentsState & RecentsActions;

function workspacesKey(ids: string[]): string {
  return [...ids].sort().join(",");
}

function sortByUpdatedDesc(issues: Issue[]): Issue[] {
  return [...issues].sort((a, b) => b.updated_at.localeCompare(a.updated_at));
}

/**
 * Owns the home sidebar Recents data. One fetch per unique set of
 * workspace IDs — issues update incrementally via upsertIssue from the
 * WebSocket sync, so a single issue change does not trigger a full refetch.
 */
export const useRecentsStore = create<RecentsStore>((set, get) => ({
  issues: [],
  loading: false,
  loadedWorkspaceIds: [],
  inFlightKey: null,

  fetch: async (workspaceIds) => {
    const key = workspacesKey(workspaceIds);
    const state = get();
    if (state.inFlightKey === key) {
      logger.debug("fetch already in flight for", key);
      return;
    }
    if (workspaceIds.length === 0) {
      set({
        issues: [],
        loading: false,
        loadedWorkspaceIds: [],
        inFlightKey: null,
      });
      return;
    }
    set({ loading: true, inFlightKey: key });
    try {
      const results = await Promise.all(
        workspaceIds.map((wsId) =>
          api
            .listRecentIssues({
              workspace_id: wsId,
              limit: RECENT_LIMIT_PER_WORKSPACE,
            })
            .then((res) => res.issues)
            .catch((err) => {
              logger.error("recents fetch failed for workspace", wsId, err);
              return [] as Issue[];
            }),
        ),
      );
      if (get().inFlightKey !== key) {
        logger.debug("ignoring stale recents result for", key);
        return;
      }
      set({
        issues: sortByUpdatedDesc(results.flat()),
        loading: false,
        loadedWorkspaceIds: workspaceIds,
        inFlightKey: null,
      });
    } catch (err) {
      logger.error("recents fetch failed", err);
      if (get().inFlightKey === key) {
        set({ loading: false, inFlightKey: null });
      }
    }
  },

  upsertIssue: (issue) =>
    set((state) => {
      // Only track issues whose workspace is part of the loaded set; otherwise
      // the result is incomplete and should not pollute the list.
      if (
        state.loadedWorkspaceIds.length > 0 &&
        !state.loadedWorkspaceIds.includes(issue.workspace_id)
      ) {
        return state;
      }
      const existing = state.issues.find((i) => i.id === issue.id);
      const merged: Issue = existing ? { ...existing, ...issue } : issue;
      const others = state.issues.filter((i) => i.id !== issue.id);
      return { issues: sortByUpdatedDesc([merged, ...others]) };
    }),

  removeIssue: (issueId) =>
    set((state) => ({ issues: state.issues.filter((i) => i.id !== issueId) })),

  clear: () =>
    set({
      issues: [],
      loading: false,
      loadedWorkspaceIds: [],
      inFlightKey: null,
    }),
}));
