"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Check, ChevronDown, Loader2, Sparkles } from "lucide-react";

import { api } from "@/shared/api";
import type {
  AgentModelConfig,
  AgentRuntime,
  ModelListRequest,
  RuntimeModel,
} from "@/shared/types";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Input } from "@/components/ui/input";
import { useRuntimeStore } from "../store";

// ---------------------------------------------------------------------------
// Discovery (initiate + poll, module-level cache)
// ---------------------------------------------------------------------------

const POLL_INTERVAL_MS = 500;
const POLL_TIMEOUT_MS = 30_000;
const CACHE_TTL_MS = 60_000;

const discoveryCache = new Map<string, { result: ModelListRequest; at: number }>();
const inflight = new Map<string, Promise<ModelListRequest>>();

function isTerminal(status: ModelListRequest["status"]): boolean {
  return status === "completed" || status === "failed" || status === "timeout";
}

async function discoverModels(runtimeId: string): Promise<ModelListRequest> {
  const cached = discoveryCache.get(runtimeId);
  if (cached && Date.now() - cached.at < CACHE_TTL_MS) {
    return cached.result;
  }
  const running = inflight.get(runtimeId);
  if (running) return running;

  const promise = (async () => {
    const req = await api.initiateListModels(runtimeId);
    let current = req;
    const deadline = Date.now() + POLL_TIMEOUT_MS;
    while (!isTerminal(current.status) && Date.now() < deadline) {
      await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
      current = await api.getModelListResult(runtimeId, req.id);
    }
    if (current.status === "completed") {
      discoveryCache.set(runtimeId, { result: current, at: Date.now() });
    }
    return current;
  })();

  inflight.set(runtimeId, promise);
  try {
    return await promise;
  } finally {
    inflight.delete(runtimeId);
  }
}

/** Picks the discovery target: the most recently seen online runtime for a provider. */
function pickRuntime(runtimes: AgentRuntime[], provider: string): AgentRuntime | null {
  const candidates = runtimes.filter(
    (r) => r.provider === provider && r.status === "online",
  );
  if (candidates.length === 0) return null;
  return candidates.reduce((best, r) =>
    (r.last_seen_at ?? "") > (best.last_seen_at ?? "") ? r : best,
  );
}

interface ModelsState {
  loading: boolean;
  models: RuntimeModel[];
  supported: boolean;
  error: string | null;
}

/**
 * useRuntimeModels discovers the model catalog for a provider by asking an
 * online runtime of that provider (via the server's pending-request flow).
 */
export function useRuntimeModels(provider: string): ModelsState & { runtimeOnline: boolean } {
  const runtimes = useRuntimeStore((s) => s.runtimes);
  const fetching = useRuntimeStore((s) => s.fetching);
  const fetchAll = useRuntimeStore((s) => s.fetchAll);

  // Hydrate the runtimes store if nothing loaded it yet (e.g. the picker is
  // rendered inside the create-agent dialog before visiting the runtimes page).
  useEffect(() => {
    if (fetching && runtimes.length === 0) {
      void fetchAll();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const runtime = useMemo(() => pickRuntime(runtimes, provider), [runtimes, provider]);
  const runtimeId = runtime?.id ?? null;

  const [state, setState] = useState<ModelsState>({
    loading: false,
    models: [],
    supported: true,
    error: null,
  });

  useEffect(() => {
    if (!runtimeId) {
      setState({ loading: false, models: [], supported: true, error: null });
      return;
    }
    let alive = true;
    setState((s) => ({ ...s, loading: true, error: null }));
    discoverModels(runtimeId)
      .then((result) => {
        if (!alive) return;
        if (result.status === "completed") {
          setState({
            loading: false,
            models: result.models ?? [],
            supported: result.supported,
            error: null,
          });
        } else {
          setState({
            loading: false,
            models: [],
            supported: true,
            error: result.error || "model discovery failed",
          });
        }
      })
      .catch((err: unknown) => {
        if (!alive) return;
        setState({
          loading: false,
          models: [],
          supported: true,
          error: err instanceof Error ? err.message : "model discovery failed",
        });
      });
    return () => {
      alive = false;
    };
  }, [runtimeId]);

  return { ...state, runtimeOnline: runtimeId != null };
}

// ---------------------------------------------------------------------------
// ModelPicker
// ---------------------------------------------------------------------------

export function ModelPicker({
  provider,
  value,
  onChange,
  autoSelectDefault = false,
}: {
  provider: string;
  /** Current selection for this provider; empty object = runtime defaults. */
  value: AgentModelConfig;
  onChange: (next: AgentModelConfig) => void;
  /**
   * When true and no model is set yet, the catalog's default (latest) model
   * is auto-selected once discovery completes. Used on agent creation so new
   * agents pick up the latest model out of the box.
   */
  autoSelectDefault?: boolean;
}) {
  const { loading, models, supported, error, runtimeOnline } = useRuntimeModels(provider);
  const autoSelected = useRef(false);

  const selectedModel = value.model ?? "";
  const selectedEntry = models.find((m) => m.id === selectedModel);
  const defaultEntry = models.find((m) => m.default);

  // Auto-select the catalog default (latest flagship) on creation flows.
  useEffect(() => {
    if (!autoSelectDefault || autoSelected.current) return;
    if (selectedModel !== "" || !defaultEntry) return;
    autoSelected.current = true;
    onChange({ ...value, model: defaultEntry.id });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoSelectDefault, defaultEntry?.id, selectedModel]);

  const selectModel = (modelId: string) => {
    const next: AgentModelConfig = { ...value, model: modelId || undefined };
    // Keep thinking_level only when the new model still supports it.
    if (next.thinking_level) {
      const entry = models.find((m) => m.id === modelId);
      const levels = entry?.thinking?.supported_levels ?? [];
      if (modelId !== "" && entry && !levels.some((l) => l.value === next.thinking_level)) {
        next.thinking_level = undefined;
      }
    }
    onChange(next);
  };

  if (!supported) {
    return (
      <div className="text-xs italic text-muted-foreground py-1.5">
        Model is managed by the runtime
      </div>
    );
  }

  // No discovered catalog (offline runtime or failed discovery): fall back
  // to manual entry so power users can still pin a model ID.
  const manualEntry = !loading && models.length === 0;

  const thinkingLevels = selectedEntry?.thinking?.supported_levels
    ?? (selectedModel === "" ? defaultEntry?.thinking?.supported_levels : undefined)
    ?? [];

  return (
    <div className="flex flex-wrap items-center gap-2">
      {loading ? (
        <div className="flex items-center gap-2 rounded-lg border border-dashed px-3 py-2 text-sm text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          <span>Discovering models…</span>
        </div>
      ) : manualEntry ? (
        <div className="flex flex-col gap-1">
          <Input
            value={selectedModel}
            onChange={(e) => selectModel(e.target.value.trim())}
            placeholder="Model ID (optional — empty = default)"
            className="h-8 w-64 text-sm"
          />
          <span className="text-xs text-muted-foreground">
            {runtimeOnline
              ? error
                ? `Model discovery failed: ${error}`
                : "No models discovered — enter a model ID manually"
              : "No online runtime for this provider — enter a model ID manually"}
          </span>
        </div>
      ) : (
        <Popover>
          <PopoverTrigger
            render={
              <button
                type="button"
                className="flex items-center gap-2 rounded-lg border px-3 py-2 text-sm hover:bg-muted transition-colors"
              >
                <Sparkles className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="font-medium">
                  {selectedEntry?.label ?? (selectedModel || "Default")}
                </span>
                {selectedModel === "" && defaultEntry && (
                  <span className="text-xs text-muted-foreground">({defaultEntry.label})</span>
                )}
                <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
              </button>
            }
          />
          <PopoverContent align="start" className="max-h-72 w-72 overflow-y-auto p-1">
            <button
              type="button"
              onClick={() => selectModel("")}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
            >
              <span className="flex-1 text-left">Default</span>
              {selectedModel === "" && <Check className="h-3.5 w-3.5 text-muted-foreground" />}
            </button>
            {models.map((m) => (
              <button
                key={m.id}
                type="button"
                onClick={() => selectModel(m.id)}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
              >
                <span className="flex-1 truncate text-left">{m.label}</span>
                {m.default && (
                  <span className="rounded bg-primary/10 px-1 py-0.5 text-[10px] text-primary">
                    latest
                  </span>
                )}
                {m.id === selectedModel && <Check className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />}
              </button>
            ))}
          </PopoverContent>
        </Popover>
      )}

      {thinkingLevels.length > 0 && (
        <Popover>
          <PopoverTrigger
            render={
              <button
                type="button"
                className="flex items-center gap-1.5 rounded-lg border px-2.5 py-2 text-sm text-muted-foreground hover:bg-muted transition-colors"
              >
                <span>
                  Thinking:{" "}
                  <span className="font-medium text-foreground">
                    {thinkingLevels.find((l) => l.value === value.thinking_level)?.label ?? "Default"}
                  </span>
                </span>
                <ChevronDown className="h-3.5 w-3.5" />
              </button>
            }
          />
          <PopoverContent align="start" className="w-56 p-1">
            <button
              type="button"
              onClick={() => onChange({ ...value, thinking_level: undefined })}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
            >
              <span className="flex-1 text-left">Default</span>
              {!value.thinking_level && <Check className="h-3.5 w-3.5 text-muted-foreground" />}
            </button>
            {thinkingLevels.map((l) => (
              <button
                key={l.value}
                type="button"
                onClick={() => onChange({ ...value, thinking_level: l.value })}
                className="flex w-full items-start gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
              >
                <span className="flex-1 text-left">
                  <span className="block">{l.label}</span>
                  {l.description && (
                    <span className="block text-xs text-muted-foreground">{l.description}</span>
                  )}
                </span>
                {l.value === value.thinking_level && (
                  <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                )}
              </button>
            ))}
          </PopoverContent>
        </Popover>
      )}
    </div>
  );
}
