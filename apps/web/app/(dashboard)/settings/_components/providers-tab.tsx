"use client";

import { useEffect, useState, useCallback } from "react";
import { CheckCircle2, CircleAlert, Eye, EyeOff, Loader2, Save } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import type { ProviderConfig, ProviderValidationResult, WorkspaceProviderSettings } from "@/shared/types";

const PROVIDERS = [
  { key: "claude", label: "Claude Code", envHint: "ANTHROPIC_API_KEY" },
  { key: "codex", label: "Codex", envHint: "OPENAI_API_KEY" },
  { key: "opencode", label: "OpenCode", envHint: "OPENAI_API_KEY" },
  { key: "cursor", label: "Cursor", envHint: "" },
] as const;

const emptyConfig = (): ProviderConfig => ({
  enabled: false,
  api_key: "",
  target_version: "",
});

const validationTone: Record<ProviderValidationResult["status"], string> = {
  valid: "text-primary",
  invalid: "text-destructive",
  unsupported: "text-muted-foreground",
  temporarily_unavailable: "text-foreground",
};

function normalizeProviders(raw: Record<string, ProviderConfig> | undefined): Record<string, ProviderConfig> {
  if (!raw) return {};
  const out: Record<string, ProviderConfig> = {};
  for (const [k, v] of Object.entries(raw)) {
    out[k] = {
      enabled: v.enabled ?? false,
      api_key: v.api_key ?? "",
      target_version: v.target_version ?? "",
    };
  }
  return out;
}

export function ProvidersTab() {
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);
  const user = useWorkspaceStore.getState;

  const [providers, setProviders] = useState<Record<string, ProviderConfig>>({});
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [visibleKeys, setVisibleKeys] = useState<Set<string>>(new Set());
  const [validating, setValidating] = useState<Set<string>>(new Set());
  const [validationResults, setValidationResults] = useState<Record<string, ProviderValidationResult>>({});

  const loadConfig = useCallback(async () => {
    if (!workspace) return;
    try {
      const config = await api.getProviderConfig(workspace.id);
      setProviders(normalizeProviders(config.providers));
    } catch {
      toast.error("Failed to load provider configuration");
    } finally {
      setLoading(false);
    }
  }, [workspace]);

  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  const getConfig = (key: string): ProviderConfig =>
    providers[key] ?? emptyConfig();

  const updateProvider = (key: string, update: Partial<ProviderConfig>) => {
    setProviders((prev) => ({
      ...prev,
      [key]: { ...emptyConfig(), ...prev[key], ...update },
    }));
    setValidationResults((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
  };

  const toggleKeyVisibility = (key: string) => {
    setVisibleKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const handleSave = async () => {
    if (!workspace) return;
    setSaving(true);
    try {
      const providersForSave = Object.fromEntries(
        Object.entries(providers).map(([key, config]) => [
          key,
          {
            enabled: config.enabled,
            api_key: config.api_key,
          },
        ]),
      );
      const data: WorkspaceProviderSettings = {
        providers: providersForSave,
      };
      const result = await api.updateProviderConfig(workspace.id, data);
      setProviders(normalizeProviders(result.providers));
      toast.success("Provider configuration saved");
    } catch {
      toast.error("Failed to save provider configuration");
    } finally {
      setSaving(false);
    }
  };

  const handleValidate = async (key: string) => {
    if (!workspace) return;
    const config = getConfig(key);
    setValidating((prev) => new Set(prev).add(key));
    try {
      const result = await api.validateProviderAPIKey(workspace.id, key, config.api_key);
      setValidationResults((prev) => ({ ...prev, [key]: result }));
    } catch {
      toast.error("Failed to validate provider API key");
    } finally {
      setValidating((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <div>
          <h2 className="text-lg font-semibold">Providers</h2>
          <p className="text-sm text-muted-foreground">Loading...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">Providers</h2>
        <p className="text-sm text-muted-foreground">
          Configure code agent providers for this workspace. When API keys are set,
          environments use them automatically — no per-user login required.
        </p>
      </div>

      <div className="space-y-4">
        {PROVIDERS.map(({ key, label, envHint }) => {
          const config = getConfig(key);
          const isKeyVisible = visibleKeys.has(key);
          const isValidating = validating.has(key);
          const validationResult = validationResults[key];
          const validationIcon = validationResult?.status === "valid"
            ? <CheckCircle2 className="h-4 w-4" />
            : <CircleAlert className="h-4 w-4" />;
          return (
            <Card key={key}>
              <CardContent className="pt-4 space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium">{label}</p>
                    {envHint && (
                      <p className="text-xs text-muted-foreground">
                        Env: {envHint}
                      </p>
                    )}
                  </div>
                  <Switch
                    checked={config.enabled}
                    onCheckedChange={(checked) =>
                      updateProvider(key, { enabled: checked })
                    }
                  />
                </div>

                {config.enabled && (
                  <div className="space-y-3 pt-2 border-t">
                    <div className="space-y-1.5">
                      <Label className="text-xs">API Key</Label>
                      <div className="flex gap-2">
                        <Input
                          type={isKeyVisible ? "text" : "password"}
                          placeholder="sk-..."
                          value={config.api_key}
                          onChange={(e) =>
                            updateProvider(key, { api_key: e.target.value })
                          }
                          className="font-mono text-xs"
                        />
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => toggleKeyVisibility(key)}
                          className="shrink-0"
                          aria-label={isKeyVisible ? `Hide ${label} API key` : `Show ${label} API key`}
                        >
                          {isKeyVisible ? (
                            <EyeOff className="h-4 w-4" />
                          ) : (
                            <Eye className="h-4 w-4" />
                          )}
                        </Button>
                        <Button
                          variant="outline"
                          onClick={() => handleValidate(key)}
                          disabled={isValidating || !config.api_key.trim()}
                          className="shrink-0"
                        >
                          {isValidating ? (
                            <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                          ) : (
                            <CheckCircle2 className="h-4 w-4 mr-1" />
                          )}
                          {isValidating ? "Validating..." : "Validate"}
                        </Button>
                      </div>
                      {validationResult && (
                        <p className={`flex items-center gap-1 text-xs ${validationTone[validationResult.status]}`}>
                          {validationIcon}
                          <span>{validationResult.message}</span>
                        </p>
                      )}
                      <p className="text-xs text-muted-foreground">
                        Leave empty to use per-user authentication
                      </p>
                    </div>

                    <div className="space-y-1.5">
                      <Label className="text-xs">Tested Version</Label>
                      <p className="rounded-md border bg-muted/40 px-3 py-2 font-mono text-xs text-muted-foreground">
                        {config.target_version || "Not managed"}
                      </p>
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
          );
        })}
      </div>

      <Button onClick={handleSave} disabled={saving}>
        <Save className="h-4 w-4 mr-1" />
        {saving ? "Saving..." : "Save"}
      </Button>
    </div>
  );
}
