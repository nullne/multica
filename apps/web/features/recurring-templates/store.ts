"use client";

import { create } from "zustand";
import type { RecurringTemplate } from "@/shared/types";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";

const logger = createLogger("recurring-template-store");

interface RecurringTemplateState {
  templates: RecurringTemplate[];
  loading: boolean;
  fetch: () => Promise<void>;
  addTemplate: (template: RecurringTemplate) => void;
  updateTemplate: (id: string, updates: Partial<RecurringTemplate>) => void;
  removeTemplate: (id: string) => void;
}

export const useRecurringTemplateStore = create<RecurringTemplateState>((set, get) => ({
  templates: [],
  loading: true,

  fetch: async () => {
    const isInitialLoad = get().templates.length === 0;
    if (isInitialLoad) set({ loading: true });
    try {
      const templates = await api.listRecurringTemplates();
      logger.info("fetched", templates.length, "recurring templates");
      set({ templates, loading: false });
    } catch (err) {
      logger.error("fetch recurring templates failed", err);
      toast.error("Failed to load recurring templates");
      if (isInitialLoad) set({ loading: false });
    }
  },

  addTemplate: (template) =>
    set((s) => ({
      templates: s.templates.some((t) => t.id === template.id)
        ? s.templates
        : [...s.templates, template],
    })),

  updateTemplate: (id, updates) =>
    set((s) => ({
      templates: s.templates.map((t) => (t.id === id ? { ...t, ...updates } : t)),
    })),

  removeTemplate: (id) =>
    set((s) => ({ templates: s.templates.filter((t) => t.id !== id) })),
}));
