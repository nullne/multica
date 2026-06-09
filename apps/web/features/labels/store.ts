"use client";

import { create } from "zustand";
import type { Label } from "@/shared/types";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";

const logger = createLogger("label-store");

interface LabelState {
  labels: Label[];
  loading: boolean;
  fetch: () => Promise<void>;
  setLabels: (labels: Label[]) => void;
  addLabel: (label: Label) => void;
  updateLabel: (id: string, updates: Partial<Label>) => void;
  removeLabel: (id: string) => void;
}

export const useLabelStore = create<LabelState>((set, get) => ({
  labels: [],
  loading: true,

  fetch: async () => {
    const workspaceId = api.getWorkspaceId();
    const isInitialLoad = get().labels.length === 0;
    if (isInitialLoad) set({ loading: true });
    try {
      const labels = await api.listLabels();
      if (api.getWorkspaceId() !== workspaceId) {
        logger.debug("skip stale fetch result", workspaceId);
        return;
      }
      logger.info("fetched", labels.length, "labels");
      set({ labels, loading: false });
    } catch (err) {
      if (api.getWorkspaceId() !== workspaceId) {
        logger.debug("skip stale fetch error", workspaceId);
        return;
      }
      logger.error("fetch labels failed", err);
      toast.error("Failed to load labels");
      if (isInitialLoad) set({ loading: false });
    }
  },

  setLabels: (labels) => set({ labels }),

  addLabel: (label) =>
    set((s) => ({
      labels: s.labels.some((l) => l.id === label.id)
        ? s.labels
        : [...s.labels, label],
    })),

  updateLabel: (id, updates) =>
    set((s) => ({
      labels: s.labels.map((l) => (l.id === id ? { ...l, ...updates } : l)),
    })),

  removeLabel: (id) =>
    set((s) => ({ labels: s.labels.filter((l) => l.id !== id) })),
}));
