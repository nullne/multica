"use client";

import { useEffect, useState, useCallback } from "react";
import { Webhook, Trash2, Copy, Check, Plus, Pause, Play, RefreshCw, ChevronDown, ChevronRight, Pencil } from "lucide-react";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import type { WebhookWithActions, CreateWebhookResponse, Agent, Daemon, CreateIssueActionConfig, WebhookAction, AdapterInfo } from "@/shared/types";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { useWorkspaceStore } from "@/features/workspace";

export function WebhooksTab() {
  const [webhooks, setWebhooks] = useState<WebhookWithActions[]>([]);
  const [daemons, setDaemons] = useState<Daemon[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [editingWebhook, setEditingWebhook] = useState<WebhookWithActions | null>(null);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [newTokenData, setNewTokenData] = useState<{ token: string; webhookId: string; url: string; sourceType: string } | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);
  const [urlCopied, setUrlCopied] = useState(false);
  const [exampleCopied, setExampleCopied] = useState(false);
  const agents = useWorkspaceStore((s) => s.agents);

  const loadWebhooks = useCallback(async () => {
    try {
      const [list, daemonList] = await Promise.all([
        api.listWebhooks(),
        api.listDaemons(),
      ]);
      setWebhooks(list);
      setDaemons(daemonList);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to load webhooks");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { loadWebhooks(); }, [loadWebhooks]);

  const handleDelete = async (id: string) => {
    try {
      await api.deleteWebhook(id);
      await loadWebhooks();
      toast.success("Webhook deleted");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete webhook");
    }
  };

  const handleToggleStatus = async (wh: WebhookWithActions) => {
    const newStatus = wh.webhook.status === "active" ? "paused" : "active";
    try {
      await api.updateWebhook(wh.webhook.id, { status: newStatus });
      await loadWebhooks();
      toast.success(newStatus === "active" ? "Webhook activated" : "Webhook paused");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update webhook");
    }
  };

  const handleRegenerateToken = async (id: string) => {
    try {
      const result = await api.regenerateWebhookToken(id);
      const baseUrl = typeof window !== "undefined" ? window.location.origin : "";
      const wh = webhooks.find((w) => w.webhook.id === id);
      setNewTokenData({ token: result.token, webhookId: id, url: `${baseUrl}/api/webhooks/${id}`, sourceType: wh?.webhook.source_type ?? "standard" });
      await loadWebhooks();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to regenerate token");
    }
  };

  const handleCopy = async (text: string, type: "token" | "url" | "example") => {
    await navigator.clipboard.writeText(text);
    const setters = { token: setTokenCopied, url: setUrlCopied, example: setExampleCopied };
    setters[type](true);
    setTimeout(() => setters[type](false), 2000);
  };

  const agentName = (agentId: string) => {
    const agent = agents.find((a: Agent) => a.id === agentId);
    return agent?.name ?? "Unknown Agent";
  };

  const envName = (daemonId: string) => {
    const d = daemons.find((daemon) => daemon.id === daemonId);
    return d?.device_name || d?.daemon_id || "Unknown";
  };

  const apiBaseUrl = typeof window !== "undefined"
    ? (process.env.NEXT_PUBLIC_API_URL ?? window.location.origin)
    : "";

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Webhook className="h-4 w-4 text-muted-foreground" />
            <h2 className="text-sm font-semibold">Webhooks</h2>
          </div>
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="h-3.5 w-3.5 mr-1" />
            Create Webhook
          </Button>
        </div>

        <p className="text-xs text-muted-foreground">
          Webhooks allow external systems (alerting, monitoring, CI) to send events that automatically create issues and trigger agent processing.
        </p>

        {loading ? (
          <div className="space-y-2">
            {Array.from({ length: 2 }).map((_, i) => (
              <Card key={i}>
                <CardContent className="flex items-center gap-3">
                  <div className="flex-1 space-y-1.5">
                    <Skeleton className="h-4 w-40" />
                    <Skeleton className="h-3 w-64" />
                  </div>
                  <Skeleton className="h-8 w-8 rounded" />
                </CardContent>
              </Card>
            ))}
          </div>
        ) : webhooks.length === 0 ? (
          <Card>
            <CardContent className="py-8 text-center text-sm text-muted-foreground">
              No webhooks configured. Create one to start receiving external events.
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-2">
            {webhooks.map((wh) => (
              <WebhookCard
                key={wh.webhook.id}
                wh={wh}
                agentName={agentName}
                envName={envName}
                apiBaseUrl={apiBaseUrl}
                onToggle={() => handleToggleStatus(wh)}
                onEdit={() => setEditingWebhook(wh)}
                onRegenerate={() => handleRegenerateToken(wh.webhook.id)}
                onDelete={() => setDeleteConfirmId(wh.webhook.id)}
              />
            ))}
          </div>
        )}
      </section>

      <CreateWebhookDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        agents={agents}
        daemons={daemons}
        apiBaseUrl={apiBaseUrl}
        onCreated={async (data) => {
          setNewTokenData(data);
          await loadWebhooks();
        }}
      />

      <EditWebhookDialog
        webhook={editingWebhook}
        onOpenChange={(v) => { if (!v) setEditingWebhook(null); }}
        agents={agents}
        daemons={daemons}
        onUpdated={loadWebhooks}
      />

      <AlertDialog open={!!deleteConfirmId} onOpenChange={(v) => { if (!v) setDeleteConfirmId(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete webhook</AlertDialogTitle>
            <AlertDialogDescription>
              This webhook and all its event history will be permanently deleted. External systems will receive errors when sending to this URL.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={async () => {
                if (deleteConfirmId) await handleDelete(deleteConfirmId);
                setDeleteConfirmId(null);
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={!!newTokenData} onOpenChange={(v) => { if (!v) { setNewTokenData(null); setTokenCopied(false); setUrlCopied(false); setExampleCopied(false); } }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Webhook created</DialogTitle>
            <DialogDescription>
              Copy the URL and token below. The token will not be shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 min-w-0">
            <div>
              <Label className="text-xs text-muted-foreground mb-1">Webhook URL</Label>
              <div className="flex items-center gap-2">
                <code className="flex-1 rounded-md border bg-muted/50 px-3 py-2 text-xs break-all select-all">
                  {newTokenData?.url}
                </code>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button variant="outline" size="icon" onClick={() => newTokenData && handleCopy(newTokenData.url, "url")}>
                        {urlCopied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                      </Button>
                    }
                  />
                  <TooltipContent>Copy URL</TooltipContent>
                </Tooltip>
              </div>
            </div>
            <div>
              <Label className="text-xs text-muted-foreground mb-1">Secret Token</Label>
              <div className="flex items-center gap-2">
                <code className="flex-1 rounded-md border bg-muted/50 px-3 py-2 text-xs break-all select-all">
                  {newTokenData?.token}
                </code>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button variant="outline" size="icon" onClick={() => newTokenData && handleCopy(newTokenData.token, "token")}>
                        {tokenCopied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                      </Button>
                    }
                  />
                  <TooltipContent>Copy token</TooltipContent>
                </Tooltip>
              </div>
            </div>
            <div>
              <Label className="text-xs text-muted-foreground mb-1">Usage Example</Label>
              <div className="flex items-start gap-2 min-w-0">
                <code className="flex-1 min-w-0 rounded-md border bg-muted/50 px-3 py-2 text-xs whitespace-pre overflow-x-auto select-all">
{`curl -X POST ${newTokenData?.url} \\
  -H "Authorization: Bearer ${newTokenData?.token}" \\
  -H "Content-Type: application/json" \\
  -d '${usageExamplePayload(newTokenData?.sourceType)}'`}
                </code>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button variant="outline" size="icon" className="shrink-0 mt-0.5" onClick={() => {
                        if (!newTokenData) return;
                        const cmd = `curl -X POST ${newTokenData.url} \\\n  -H "Authorization: Bearer ${newTokenData.token}" \\\n  -H "Content-Type: application/json" \\\n  -d '${usageExamplePayload(newTokenData.sourceType)}'`;
                        handleCopy(cmd, "example");
                      }}>
                        {exampleCopied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                      </Button>
                    }
                  />
                  <TooltipContent>Copy command</TooltipContent>
                </Tooltip>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => { setNewTokenData(null); setTokenCopied(false); setUrlCopied(false); }}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function getActionConfig(action: WebhookAction): CreateIssueActionConfig {
  return action.config as CreateIssueActionConfig;
}

function usageExamplePayload(sourceType?: string): string {
  switch (sourceType) {
    case "oss-alert":
      return JSON.stringify({
        alerts: [{
          labels: { alertname: "HighLatency", app: "api" },
          annotations: { value: "500ms" },
          startsAt: "2026-01-01T00:00:00Z",
        }],
      }, null, 2);
    default:
      return `{"title": "Test alert", "body": "Something happened"}`;
  }
}

function WebhookCard({ wh, agentName, envName, apiBaseUrl, onToggle, onEdit, onRegenerate, onDelete }: {
  wh: WebhookWithActions;
  agentName: (agentId: string) => string;
  envName: (daemonId: string) => string;
  apiBaseUrl: string;
  onToggle: () => void;
  onEdit: () => void;
  onRegenerate: () => void;
  onDelete: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const webhook = wh.webhook;
  const isActive = webhook.status === "active";
  const primaryAction = wh.actions[0];
  const primaryConfig = primaryAction ? getActionConfig(primaryAction) : null;

  return (
    <Card>
      <CardContent className="space-y-2">
        <div className="flex items-center gap-3">
          <button onClick={() => setExpanded(!expanded)} className="shrink-0 text-muted-foreground hover:text-foreground">
            {expanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          </button>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium truncate">{webhook.name}</span>
              <Badge variant={isActive ? "default" : "secondary"} className="text-[10px] px-1.5 py-0">
                {isActive ? "Active" : "Paused"}
              </Badge>
              <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                {webhook.source_type}
              </Badge>
            </div>
            <div className="text-xs text-muted-foreground">
              {primaryConfig ? `Agent: ${agentName(primaryConfig.agent_id)}` : "No action configured"}
              {" · "}{wh.event_count} events · {webhook.token_prefix}...
            </div>
          </div>
          <div className="flex items-center gap-1">
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button variant="ghost" size="icon-sm" onClick={onEdit}>
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                }
              />
              <TooltipContent>Edit</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button variant="ghost" size="icon-sm" onClick={onToggle}>
                    {isActive ? <Pause className="h-3.5 w-3.5" /> : <Play className="h-3.5 w-3.5" />}
                  </Button>
                }
              />
              <TooltipContent>{isActive ? "Pause" : "Activate"}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button variant="ghost" size="icon-sm" onClick={onRegenerate}>
                    <RefreshCw className="h-3.5 w-3.5" />
                  </Button>
                }
              />
              <TooltipContent>Regenerate token</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button variant="ghost" size="icon-sm" onClick={onDelete}>
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                }
              />
              <TooltipContent>Delete</TooltipContent>
            </Tooltip>
          </div>
        </div>
        {expanded && (
          <div className="ml-7 space-y-2 text-xs text-muted-foreground border-t pt-2">
            <div className="flex gap-2">
              <span className="font-medium w-28 shrink-0">URL:</span>
              <code className="break-all">{apiBaseUrl}/api/webhooks/{webhook.id}</code>
            </div>
            <div className="flex gap-2">
              <span className="font-medium w-28 shrink-0">Dedup window:</span>
              <span>{webhook.dedup_window_seconds}s</span>
            </div>
            {primaryConfig && (
              <>
                <div className="border-t pt-2 mt-1">
                  <span className="text-[10px] font-semibold uppercase tracking-wide">Action: Create Issue</span>
                </div>
                <div className="flex gap-2">
                  <span className="font-medium w-28 shrink-0">Agent:</span>
                  <span>{agentName(primaryConfig.agent_id)}</span>
                </div>
                <div className="flex gap-2">
                  <span className="font-medium w-28 shrink-0">Provider:</span>
                  <span className="capitalize">{primaryConfig.dispatch_provider || "Auto"}</span>
                </div>
                <div className="flex gap-2">
                  <span className="font-medium w-28 shrink-0">Environment:</span>
                  <span>{primaryConfig.dispatch_daemon_id ? envName(primaryConfig.dispatch_daemon_id) : "Any"}</span>
                </div>
                {primaryConfig.title_template && (
                  <div className="flex gap-2">
                    <span className="font-medium w-28 shrink-0">Issue title:</span>
                    <code>{primaryConfig.title_template}</code>
                  </div>
                )}
                {primaryConfig.description_template && (
                  <div className="flex gap-2">
                    <span className="font-medium w-28 shrink-0">Issue desc:</span>
                    <code className="break-all">{primaryConfig.description_template}</code>
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function CreateWebhookDialog({ open, onOpenChange, agents, daemons, apiBaseUrl, onCreated }: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  agents: Agent[];
  daemons: Daemon[];
  apiBaseUrl: string;
  onCreated: (data: { token: string; webhookId: string; url: string; sourceType: string }) => void;
}) {
  const [name, setName] = useState("");
  const [sourceType, setSourceType] = useState("standard");
  const [actionType, setActionType] = useState("create_issue");
  const [agentId, setAgentId] = useState("");
  const [dispatchProvider, setDispatchProvider] = useState("");
  const [dispatchDaemonId, setDispatchDaemonId] = useState("");
  const [titleTemplate, setTitleTemplate] = useState("");
  const [descriptionTemplate, setDescriptionTemplate] = useState("");
  const [creating, setCreating] = useState(false);
  const [adapters, setAdapters] = useState<AdapterInfo[]>([]);
  const [showSchema, setShowSchema] = useState(false);

  const activeAgents = agents.filter((a: Agent) => !a.archived_at);
  const selectedAgent = activeAgents.find((a) => a.id === agentId);

  useEffect(() => {
    if (open) {
      api.listWebhookAdapters().then(setAdapters).catch(() => {});
    }
  }, [open]);

  useEffect(() => {
    if (open && activeAgents.length > 0 && !agentId) {
      setAgentId(activeAgents[0]!.id);
    }
  }, [open, activeAgents, agentId]);

  const currentAdapter = adapters.find((a) => a.source_type === sourceType);

  const handleCreate = async () => {
    setCreating(true);
    try {
      const result: CreateWebhookResponse = await api.createWebhook({
        name,
        source_type: sourceType as "standard" | "oss-alert",
        agent_id: agentId,
        title_template: titleTemplate || undefined,
        description_template: descriptionTemplate || undefined,
        dispatch_provider: dispatchProvider || undefined,
        dispatch_daemon_id: dispatchDaemonId || undefined,
      });
      onCreated({ token: result.token, webhookId: result.webhook.id, url: `${apiBaseUrl}/api/webhooks/${result.webhook.id}`, sourceType });
      onOpenChange(false);
      setName("");
      setSourceType("standard");
      setActionType("create_issue");
      setAgentId("");
      setDispatchProvider("");
      setDispatchDaemonId("");
      setTitleTemplate("");
      setDescriptionTemplate("");
      setShowSchema(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to create webhook");
    } finally {
      setCreating(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create Webhook</DialogTitle>
          <DialogDescription>
            Create an endpoint for external systems to send events.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-3">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Endpoint</h3>
            <div className="space-y-1.5">
              <Label htmlFor="wh-name">Name</Label>
              <Input id="wh-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Grafana Alerts" />
            </div>
            <div className="space-y-1.5">
              <Label>Source Type</Label>
              <Select value={sourceType} onValueChange={(v) => { if (v) { setSourceType(v); setShowSchema(false); } }}>
                <SelectTrigger size="sm"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {adapters.map((a) => (
                    <SelectItem key={a.source_type} value={a.source_type}>
                      {a.source_type}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {currentAdapter && (
              <div className="rounded-md border bg-muted/30 p-3 space-y-2">
                <p className="text-xs text-muted-foreground">{currentAdapter.description}</p>
                <button
                  type="button"
                  onClick={() => setShowSchema(!showSchema)}
                  className="text-xs text-primary hover:underline flex items-center gap-1"
                >
                  {showSchema ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                  {showSchema ? "Hide" : "Show"} payload schema & example
                </button>
                {showSchema && (
                  <div className="space-y-2 pt-1">
                    <div>
                      <p className="text-[10px] font-medium text-muted-foreground mb-1">Available template variables:</p>
                      <div className="grid gap-0.5">
                        {currentAdapter.keys.map((k) => (
                          <div key={k.key} className="flex items-start gap-2 text-[10px]">
                            <code className="font-mono text-primary shrink-0">{`{{.${k.key}}}`}</code>
                            <span className="text-muted-foreground">{k.description}</span>
                            {k.required && <Badge variant="outline" className="text-[8px] px-1 py-0 shrink-0">required</Badge>}
                          </div>
                        ))}
                      </div>
                    </div>
                    {currentAdapter.example && (
                      <div>
                        <p className="text-[10px] font-medium text-muted-foreground mb-1">Example payload:</p>
                        <pre className="rounded bg-muted p-2 text-[10px] overflow-x-auto whitespace-pre">
                          {currentAdapter.example}
                        </pre>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="border-t pt-4 space-y-3">
            <div>
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Action</h3>
              <p className="text-[11px] text-muted-foreground mt-1">
                When an event is received, execute the following action.
              </p>
            </div>
            <div className="flex items-center gap-3">
              <Label className="w-24 shrink-0 text-right">Action Type</Label>
              <Select value={actionType} onValueChange={(v) => { if (v) setActionType(v); }}>
                <SelectTrigger size="sm" className="w-auto"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="create_issue">Create Issue</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {actionType === "create_issue" && (
              <>
                <div className="flex items-center gap-3">
                  <Label className="w-24 shrink-0 text-right">Agent</Label>
                  <Select value={agentId} onValueChange={(v) => { if (v) setAgentId(v); }}>
                    <SelectTrigger size="sm" className="w-auto">
                      <SelectValue placeholder="Select agent">
                        {activeAgents.find((a) => a.id === agentId)?.name}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      {activeAgents.map((a: Agent) => (
                        <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                {selectedAgent && selectedAgent.providers.length > 0 && (
                  <div className="flex items-center gap-3">
                    <Label className="w-24 shrink-0 text-right">Provider</Label>
                    <Select value={dispatchProvider} onValueChange={(v) => setDispatchProvider(v ?? "")}>
                      <SelectTrigger size="sm" className="w-auto">
                        <SelectValue placeholder="Auto">
                          {dispatchProvider ? <span className="capitalize">{dispatchProvider}</span> : "Auto"}
                        </SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="">Auto</SelectItem>
                        {selectedAgent.providers.map((p) => (
                          <SelectItem key={p} value={p}>
                            <span className="capitalize">{p}</span>
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}
                <div className="flex items-center gap-3">
                  <Label className="w-24 shrink-0 text-right">Environment</Label>
                  <Select value={dispatchDaemonId} onValueChange={(v) => setDispatchDaemonId(v ?? "")}>
                    <SelectTrigger size="sm" className="w-auto">
                      <SelectValue placeholder="Any environment">
                        {dispatchDaemonId
                          ? (daemons.find((d) => d.id === dispatchDaemonId)?.device_name || daemons.find((d) => d.id === dispatchDaemonId)?.daemon_id)
                          : "Any environment"}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="">Any environment</SelectItem>
                      {daemons.map((d) => (
                        <SelectItem key={d.id} value={d.id}>
                          <span className="flex items-center gap-2">
                            <span className={`h-1.5 w-1.5 rounded-full ${d.status === "online" ? "bg-green-500" : "bg-muted-foreground/40"}`} />
                            {d.device_name || d.daemon_id}
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="wh-title-tmpl">Issue Title Template <span className="text-muted-foreground">(optional, defaults to event title)</span></Label>
                  <Input id="wh-title-tmpl" value={titleTemplate} onChange={(e) => setTitleTemplate(e.target.value)} placeholder="e.g. [Alert] {{.title}}" />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="wh-desc-tmpl">Issue Description Template <span className="text-muted-foreground">(optional, defaults to event body)</span></Label>
                  <Textarea id="wh-desc-tmpl" value={descriptionTemplate} onChange={(e) => setDescriptionTemplate(e.target.value)} placeholder="e.g. {{.body}}" rows={2} />
                </div>
              </>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleCreate} disabled={creating || !name.trim() || !agentId}>
            {creating ? "Creating..." : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function EditWebhookDialog({ webhook, onOpenChange, agents, daemons, onUpdated }: {
  webhook: WebhookWithActions | null;
  onOpenChange: (v: boolean) => void;
  agents: Agent[];
  daemons: Daemon[];
  onUpdated: () => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [sourceType, setSourceType] = useState("standard");
  const [actionType, setActionType] = useState("create_issue");
  const [agentId, setAgentId] = useState("");
  const [dispatchProvider, setDispatchProvider] = useState("");
  const [dispatchDaemonId, setDispatchDaemonId] = useState("");
  const [titleTemplate, setTitleTemplate] = useState("");
  const [descriptionTemplate, setDescriptionTemplate] = useState("");
  const [saving, setSaving] = useState(false);
  const [adapters, setAdapters] = useState<AdapterInfo[]>([]);
  const [showSchema, setShowSchema] = useState(false);

  const activeAgents = agents.filter((a: Agent) => !a.archived_at);
  const selectedAgent = activeAgents.find((a) => a.id === agentId);

  useEffect(() => {
    if (webhook) {
      setName(webhook.webhook.name);
      setSourceType(webhook.webhook.source_type);
      const action = webhook.actions[0];
      if (action) {
        setActionType(action.action_type);
        const cfg = action.config as CreateIssueActionConfig;
        setAgentId(cfg.agent_id || "");
        setDispatchProvider(cfg.dispatch_provider || "");
        setDispatchDaemonId(cfg.dispatch_daemon_id || "");
        setTitleTemplate(cfg.title_template || "");
        setDescriptionTemplate(cfg.description_template || "");
      }
      api.listWebhookAdapters().then(setAdapters).catch(() => {});
    }
  }, [webhook]);

  const currentAdapter = adapters.find((a) => a.source_type === sourceType);

  const handleSave = async () => {
    if (!webhook) return;
    setSaving(true);
    try {
      await api.updateWebhook(webhook.webhook.id, {
        name,
        source_type: sourceType as "standard" | "oss-alert",
      });

      const action = webhook.actions[0];
      if (action) {
        const config: CreateIssueActionConfig = {
          agent_id: agentId,
          title_template: titleTemplate,
          description_template: descriptionTemplate,
          labels: (action.config as CreateIssueActionConfig).labels || [],
          dispatch_provider: dispatchProvider || undefined,
          dispatch_daemon_id: dispatchDaemonId || undefined,
          dispatch_daemon_label: undefined,
        };
        await api.updateWebhookAction(webhook.webhook.id, action.id, { config });
      }

      toast.success("Webhook updated");
      onOpenChange(false);
      await onUpdated();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update webhook");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={!!webhook} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Edit Webhook</DialogTitle>
          <DialogDescription>
            Update the webhook endpoint and action configuration.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-3">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Endpoint</h3>
            <div className="space-y-1.5">
              <Label htmlFor="edit-wh-name">Name</Label>
              <Input id="edit-wh-name" value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label>Source Type</Label>
              <Select value={sourceType} onValueChange={(v) => { if (v) { setSourceType(v); setShowSchema(false); } }}>
                <SelectTrigger size="sm"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {adapters.map((a) => (
                    <SelectItem key={a.source_type} value={a.source_type}>
                      {a.source_type}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {currentAdapter && (
              <div className="rounded-md border bg-muted/30 p-3 space-y-2">
                <p className="text-xs text-muted-foreground">{currentAdapter.description}</p>
                <button
                  type="button"
                  onClick={() => setShowSchema(!showSchema)}
                  className="text-xs text-primary hover:underline flex items-center gap-1"
                >
                  {showSchema ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                  {showSchema ? "Hide" : "Show"} payload schema & example
                </button>
                {showSchema && (
                  <div className="space-y-2 pt-1">
                    <div>
                      <p className="text-[10px] font-medium text-muted-foreground mb-1">Available template variables:</p>
                      <div className="grid gap-0.5">
                        {currentAdapter.keys.map((k) => (
                          <div key={k.key} className="flex items-start gap-2 text-[10px]">
                            <code className="font-mono text-primary shrink-0">{`{{.${k.key}}}`}</code>
                            <span className="text-muted-foreground">{k.description}</span>
                            {k.required && <Badge variant="outline" className="text-[8px] px-1 py-0 shrink-0">required</Badge>}
                          </div>
                        ))}
                      </div>
                    </div>
                    {currentAdapter.example && (
                      <div>
                        <p className="text-[10px] font-medium text-muted-foreground mb-1">Example payload:</p>
                        <pre className="rounded bg-muted p-2 text-[10px] overflow-x-auto whitespace-pre">
                          {currentAdapter.example}
                        </pre>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="border-t pt-4 space-y-3">
            <div>
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Action</h3>
              <p className="text-[11px] text-muted-foreground mt-1">
                When an event is received, execute the following action.
              </p>
            </div>
            <div className="flex items-center gap-3">
              <Label className="w-24 shrink-0 text-right">Action Type</Label>
              <Select value={actionType} onValueChange={(v) => { if (v) setActionType(v); }}>
                <SelectTrigger size="sm" className="w-auto"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="create_issue">Create Issue</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {actionType === "create_issue" && (
              <>
                <div className="flex items-center gap-3">
                  <Label className="w-24 shrink-0 text-right">Agent</Label>
                  <Select value={agentId} onValueChange={(v) => { if (v) setAgentId(v); }}>
                    <SelectTrigger size="sm" className="w-auto">
                      <SelectValue placeholder="Select agent">
                        {activeAgents.find((a) => a.id === agentId)?.name}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      {activeAgents.map((a: Agent) => (
                        <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                {selectedAgent && selectedAgent.providers.length > 0 && (
                  <div className="flex items-center gap-3">
                    <Label className="w-24 shrink-0 text-right">Provider</Label>
                    <Select value={dispatchProvider} onValueChange={(v) => setDispatchProvider(v ?? "")}>
                      <SelectTrigger size="sm" className="w-auto">
                        <SelectValue placeholder="Auto">
                          {dispatchProvider ? <span className="capitalize">{dispatchProvider}</span> : "Auto"}
                        </SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="">Auto</SelectItem>
                        {selectedAgent.providers.map((p) => (
                          <SelectItem key={p} value={p}>
                            <span className="capitalize">{p}</span>
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}
                <div className="flex items-center gap-3">
                  <Label className="w-24 shrink-0 text-right">Environment</Label>
                  <Select value={dispatchDaemonId} onValueChange={(v) => setDispatchDaemonId(v ?? "")}>
                    <SelectTrigger size="sm" className="w-auto">
                      <SelectValue placeholder="Any environment">
                        {dispatchDaemonId
                          ? (daemons.find((d) => d.id === dispatchDaemonId)?.device_name || daemons.find((d) => d.id === dispatchDaemonId)?.daemon_id)
                          : "Any environment"}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="">Any environment</SelectItem>
                      {daemons.map((d) => (
                        <SelectItem key={d.id} value={d.id}>
                          <span className="flex items-center gap-2">
                            <span className={`h-1.5 w-1.5 rounded-full ${d.status === "online" ? "bg-green-500" : "bg-muted-foreground/40"}`} />
                            {d.device_name || d.daemon_id}
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="edit-wh-title-tmpl">Issue Title Template <span className="text-muted-foreground">(optional, defaults to event title)</span></Label>
                  <Input id="edit-wh-title-tmpl" value={titleTemplate} onChange={(e) => setTitleTemplate(e.target.value)} placeholder="e.g. [Alert] {{.title}}" />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="edit-wh-desc-tmpl">Issue Description Template <span className="text-muted-foreground">(optional, defaults to event body)</span></Label>
                  <Textarea id="edit-wh-desc-tmpl" value={descriptionTemplate} onChange={(e) => setDescriptionTemplate(e.target.value)} placeholder="e.g. {{.body}}" rows={2} />
                </div>
              </>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSave} disabled={saving || !name.trim() || !agentId}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
