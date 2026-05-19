"use client";

import { create } from "zustand";
import type { Issue } from "@/shared/types";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";

const logger = createLogger("recents-store");

const RECENT_LIMIT_PER_WORKSPACE = 50;

export interface RecentsFetchOptions {
  mine?: boolean;
  userId?: string | null;
}

interface RecentsState {
  issues: Issue[];
  loading: boolean;
  loadedWorkspaceIds: string[];
  loadedMine: boolean;
  loadedUserId: string | null;
  inFlightKey: string | null;
}

interface RecentsActions {
  fetch: (
    workspaceIds: string[],
    options?: RecentsFetchOptions,
  ) => Promise<void>;
  upsertIssue: (issue: Issue) => void;
  removeIssue: (issueId: string) => void;
  clear: () => void;
}

type RecentsStore = RecentsState & RecentsActions;

function makeKey(
  workspaceIds: string[],
  mine: boolean,
  userId: string | null,
): string {
  return `${mine ? "mine" : "all"}:${userId ?? ""}:${[...workspaceIds].sort().join(",")}`;
}

function sortByUpdatedDesc(issues: Issue[]): Issue[] {
  return [...issues].sort((a, b) => b.updated_at.localeCompare(a.updated_at));
}

function issueInvolvesUser(issue: Issue, userId: string | null): boolean {
  if (!userId) return false;
  const created = issue.creator_type === "member" && issue.creator_id === userId;
  const assigned =
    issue.assignee_type === "member" && issue.assignee_id === userId;
  return created || assigned;
}

/**
 * Owns the home sidebar Recents data. Fetches deduped by the combined
 * (workspace set, mine, userId) key, so toggling "Only my issues"
 * triggers a fresh server-side query rather than client-only filtering —
 * critical when the user's involved issues are outside the global top 50
 * for a busy workspace. WS issue events upsert/remove in place and
 * respect the current mine filter.
 */
export const useRecentsStore = create<RecentsStore>((set, get) => ({
  issues: [],
  loading: false,
  loadedWorkspaceIds: [],
  loadedMine: false,
  loadedUserId: null,
  inFlightKey: null,

  fetch: async (workspaceIds, options) => {
    const mine = !!options?.mine;
    const userId = options?.userId ?? null;
    const key = makeKey(workspaceIds, mine, userId);
    const state = get();
    if (state.inFlightKey === key) {
      logger.debug("fetch already in flight for", key);
      return;
    }
    if (mine && !userId) {
      logger.debug("skip mine fetch without user id");
      set({
        issues: [],
        loading: false,
        loadedWorkspaceIds: workspaceIds,
        loadedMine: mine,
        loadedUserId: userId,
        inFlightKey: null,
      });
      return;
    }
    if (workspaceIds.length === 0) {
      set({
        issues: [],
        loading: false,
        loadedWorkspaceIds: [],
        loadedMine: mine,
        loadedUserId: userId,
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
              mine: mine || undefined,
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
        loadedMine: mine,
        loadedUserId: userId,
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
      // In "Only my issues" mode, drop the issue if it no longer involves
      // the user — covers events like an issue being reassigned away.
      if (state.loadedMine && !issueInvolvesUser(issue, state.loadedUserId)) {
        if (!state.issues.some((i) => i.id === issue.id)) return state;
        return { issues: state.issues.filter((i) => i.id !== issue.id) };
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
      loadedMine: false,
      loadedUserId: null,
      inFlightKey: null,
    }),
}));
