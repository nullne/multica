"use client";

import { create } from "zustand";
import type { Issue } from "@/shared/types";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";

const logger = createLogger("issue-store");

interface IssueState {
  issues: Issue[];
  loading: boolean;
  activeIssueId: string | null;
  fetch: () => Promise<void>;
  setIssues: (issues: Issue[]) => void;
  addIssue: (issue: Issue) => void;
  updateIssue: (id: string, updates: Partial<Issue>) => void;
  removeIssue: (id: string) => void;
  setActiveIssue: (id: string | null) => void;
}

export const useIssueStore = create<IssueState>((set, get) => ({
  issues: [],
  loading: true,
  activeIssueId: null,

  fetch: async () => {
    const workspaceId = api.getWorkspaceId();
    logger.debug("fetch start");
    const isInitialLoad = get().issues.length === 0;
    if (isInitialLoad) set({ loading: true });
    try {
      const res = await api.listIssues({ limit: 200 });
      if (api.getWorkspaceId() !== workspaceId) {
        logger.debug("skip stale fetch result", workspaceId);
        return;
      }
      logger.info("fetched", res.issues.length, "issues");
      set({ issues: res.issues, loading: false });
    } catch (err) {
      if (api.getWorkspaceId() !== workspaceId) {
        logger.debug("skip stale fetch error", workspaceId);
        return;
      }
      logger.error("fetch failed", err);
      toast.error("Failed to load issues");
      if (isInitialLoad) set({ loading: false });
    }
  },

  setIssues: (issues) => set({ issues }),
  addIssue: (issue) =>
    set((s) => ({
      issues: s.issues.some((i) => i.id === issue.id)
        ? s.issues
        : [...s.issues, issue],
    })),
  updateIssue: (id, updates) =>
    set((s) => ({
      issues: s.issues.map((i) => {
        if (i.id !== id) return i;
        const merged = { ...i, ...updates };
        // Preserve existing links if the incoming payload omits them.
        if (!updates.links?.length && i.links?.length) {
          merged.links = i.links;
        }
        return merged;
      }),
    })),
  removeIssue: (id) =>
    set((s) => ({ issues: s.issues.filter((i) => i.id !== id) })),
  setActiveIssue: (id) => set({ activeIssueId: id }),
}));
