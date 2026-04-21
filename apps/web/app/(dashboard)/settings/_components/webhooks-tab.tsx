"use client";

import { useEffect, useState, useCallback } from "react";
import {
  Webhook,
  Trash2,
  Copy,
  Check,
  Plus,
  Pause,
  Play,
  RefreshCw,
  ChevronDown,
  ChevronRight,
  Pencil,
  Bot,
  GitBranch as Github,
} from "lucide-react";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import type {
  WebhookWithActions,
  CreateWebhookResponse,
  Agent,
  Daemon,
  CreateIssueActionConfig,
  CommentIssueActionConfig,
  WebhookAction,
  WebhookActionType,
  WebhookSourceType,
  AdapterInfo,
  BotUser,
} from "@/shared/types";
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
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { useWorkspaceStore } from "@/features/workspace";
import { useLabelStore } from "@/features/labels";
import type { Label as IssueLabel } from "@/shared/types";

const GITHUB_EVENT_TYPES: { value: string; label: string }[] = [
  { value: "github.push", label: "Push" },
  { value: "github.pull_request", label: "Pull request" },
  { value: "github.issues", label: "Issue" },
  { value: "github.issue_comment", label: "Issue / PR comment" },
];

export function WebhooksTab() {
  const workspaceId = useWorkspaceStore((s) => s.workspace?.id ?? null);
  const [webhooks, setWebhooks] = useState<WebhookWithActions[]>([]);
  const [bots, setBots] = useState<BotUser[]>([]);
  const [daemons, setDaemons] = useState<Daemon[]>([]);
  const [adapters, setAdapters] = useState<AdapterInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [editingWebhook, setEditingWebhook] = useState<WebhookWithActions | null>(null);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [newTokenData, setNewTokenData] = useState<{ token: string; webhookId: string; url: string; sourceType: string } | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);
  const [urlCopied, setUrlCopied] = useState(false);
  const [exampleCopied, setExampleCopied] = useState(false);
  const agents = useWorkspaceStore((s) => s.agents);

  const reload = useCallback(async () => {
    if (!workspaceId) return;
    try {
      const [webhookList, daemonList, botList, adapterList] = await Promise.all([
        api.listWebhooks(),
        api.listDaemons(),
        api.listBotUsers(workspaceId),
        api.listWebhookAdapters(),
      ]);
      setWebhooks(webhookList);
      setDaemons(daemonList);
      setBots(botList);
      setAdapters(adapterList);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to load webhooks");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => { reload(); }, [reload]);

  const handleDelete = async (id: string) => {
    try {
      await api.deleteWebhook(id);
      await reload();
      toast.success("Webhook deleted");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete webhook");
    }
  };

  const handleToggleStatus = async (wh: WebhookWithActions) => {
    const newStatus = wh.webhook.status === "active" ? "paused" : "active";
    try {
      await api.updateWebhook(wh.webhook.id, { status: newStatus });
      await reload();
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
      await reload();
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
      <BotUsersSection
        workspaceId={workspaceId}
        bots={bots}
        loading={loading}
        onChanged={reload}
      />

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
          Webhooks let external systems (alerting, monitoring, CI, GitHub) push events that
          create issues or comment on existing ones.
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
                bots={bots}
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
        adapters={adapters}
        apiBaseUrl={apiBaseUrl}
        onCreated={async (data) => {
          setNewTokenData(data);
          await reload();
        }}
      />

      <EditWebhookDialog
        webhook={editingWebhook}
        bots={bots}
        agents={agents}
        daemons={daemons}
        adapters={adapters}
        onOpenChange={(v) => { if (!v) setEditingWebhook(null); }}
        onUpdated={reload}
      />

      <AlertDialog open={!!deleteConfirmId} onOpenChange={(v) => { if (!v) setDeleteConfirmId(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete webhook</AlertDialogTitle>
            <AlertDialogDescription>
              This webhook and all its event history will be permanently deleted.
              External systems will receive errors when sending to this URL.
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

// ----------------------------------------------------------------------------
// Bot users section
// ----------------------------------------------------------------------------

function BotUsersSection({
  workspaceId,
  bots,
  loading,
  onChanged,
}: {
  workspaceId: string | null;
  bots: BotUser[];
  loading: boolean;
  onChanged: () => Promise<void> | void;
}) {
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

  const handleCreate = async () => {
    if (!workspaceId || !name.trim()) return;
    setCreating(true);
    try {
      await api.createBotUser(workspaceId, { name: name.trim() });
      toast.success("Bot user created");
      setName("");
      setCreateOpen(false);
      await onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to create bot");
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (botId: string) => {
    if (!workspaceId) return;
    try {
      await api.deleteBotUser(workspaceId, botId);
      toast.success("Bot user deleted");
      await onChanged();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete bot");
    }
  };

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Bot className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">Bot users</h2>
        </div>
        <Button size="sm" variant="outline" onClick={() => setCreateOpen(true)}>
          <Plus className="h-3.5 w-3.5 mr-1" />
          Add bot
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        Bot users post comments on behalf of webhooks. They appear in member lists but
        cannot log in. Bind one to a webhook&apos;s &quot;Comment on linked issue&quot; action.
      </p>

      {loading ? (
        <Skeleton className="h-10 w-full" />
      ) : bots.length === 0 ? (
        <Card>
          <CardContent className="py-4 text-center text-xs text-muted-foreground">
            No bot users yet.
          </CardContent>
        </Card>
      ) : (
        <div className="rounded-md border divide-y">
          {bots.map((b) => (
            <div key={b.id} className="flex items-center justify-between px-3 py-2">
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium truncate">{b.name}</div>
                <div className="text-[11px] text-muted-foreground truncate">{b.email}</div>
              </div>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button variant="ghost" size="icon-sm" onClick={() => setDeleteConfirmId(b.id)}>
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  }
                />
                <TooltipContent>Delete bot</TooltipContent>
              </Tooltip>
            </div>
          ))}
        </div>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Create bot user</DialogTitle>
            <DialogDescription>
              The bot will be added to this workspace as a regular member with role &quot;member&quot;.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="bot-name">Name</Label>
            <Input id="bot-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. GitHub Bot" />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={creating || !name.trim()}>
              {creating ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!deleteConfirmId} onOpenChange={(v) => { if (!v) setDeleteConfirmId(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete bot user</AlertDialogTitle>
            <AlertDialogDescription>
              Webhooks bound to this bot will lose their author and any &quot;Comment on linked issue&quot; actions will fail until you assign a new bot.
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
    </section>
  );
}

// ----------------------------------------------------------------------------
// Webhook card
// ----------------------------------------------------------------------------

function WebhookCard({
  wh,
  bots,
  agentName,
  envName,
  apiBaseUrl,
  onToggle,
  onEdit,
  onRegenerate,
  onDelete,
}: {
  wh: WebhookWithActions;
  bots: BotUser[];
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
  const isGitHub = webhook.source_type === "github";
  const url = isGitHub
    ? `${apiBaseUrl}/api/github/events`
    : `${apiBaseUrl}/api/webhooks/${webhook.id}`;
  const botName = webhook.bot_user_id ? (bots.find((b) => b.id === webhook.bot_user_id)?.name ?? "(deleted bot)") : null;

  return (
    <Card>
      <CardContent className="space-y-2">
        <div className="flex items-center gap-3">
          <button onClick={() => setExpanded(!expanded)} className="shrink-0 text-muted-foreground hover:text-foreground">
            {expanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          </button>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              {isGitHub && <Github className="h-3.5 w-3.5 text-muted-foreground shrink-0" />}
              <span className="text-sm font-medium truncate">{webhook.name}</span>
              <Badge variant={isActive ? "default" : "secondary"} className="text-[10px] px-1.5 py-0">
                {isActive ? "Active" : "Paused"}
              </Badge>
              <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                {webhook.source_type}
              </Badge>
            </div>
            <div className="text-xs text-muted-foreground">
              {wh.actions.length} action{wh.actions.length === 1 ? "" : "s"} · {wh.event_count} events
              {!isGitHub && ` · ${webhook.token_prefix}...`}
              {botName && ` · bot: ${botName}`}
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
            {!isGitHub && (
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
            )}
            {!isGitHub && (
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
            )}
          </div>
        </div>
        {expanded && (
          <div className="ml-7 space-y-2 text-xs text-muted-foreground border-t pt-2">
            <div className="flex gap-2">
              <span className="font-medium w-28 shrink-0">URL:</span>
              <code className="break-all">{url}</code>
            </div>
            <div className="flex gap-2">
              <span className="font-medium w-28 shrink-0">Dedup window:</span>
              <span>{webhook.dedup_window_seconds}s</span>
            </div>
            {wh.actions.map((a, idx) => (
              <ActionSummary
                key={a.id}
                action={a}
                index={idx}
                agentName={agentName}
                envName={envName}
              />
            ))}
            {wh.actions.length === 0 && (
              <div className="italic text-muted-foreground/70">No actions configured</div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function ActionSummary({
  action,
  index,
  agentName,
  envName,
}: {
  action: WebhookAction;
  index: number;
  agentName: (id: string) => string;
  envName: (id: string) => string;
}) {
  return (
    <div className="border-t pt-2 mt-1">
      <span className="text-[10px] font-semibold uppercase tracking-wide">
        Action #{index + 1}: {action.action_type}
        {!action.enabled && <Badge variant="outline" className="ml-2 text-[8px] px-1 py-0">disabled</Badge>}
      </span>
      {action.action_type === "create_issue" && (() => {
        const cfg = action.config as CreateIssueActionConfig;
        return (
          <div className="space-y-0.5 mt-1">
            <PropLine label="Agent" value={agentName(cfg.agent_id)} />
            {cfg.dispatch_provider && <PropLine label="Provider" value={cfg.dispatch_provider} />}
            {cfg.dispatch_daemon_id && <PropLine label="Environment" value={envName(cfg.dispatch_daemon_id)} />}
            {cfg.title_template && <PropLine label="Title" value={cfg.title_template} mono />}
            {cfg.event_types && cfg.event_types.length > 0 && (
              <PropLine label="Events" value={cfg.event_types.join(", ")} />
            )}
            {cfg.repos && cfg.repos.length > 0 && (
              <PropLine label="Repos" value={cfg.repos.join(", ")} />
            )}
          </div>
        );
      })()}
      {action.action_type === "comment_issue" && (() => {
        const cfg = action.config as CommentIssueActionConfig;
        return (
          <div className="space-y-0.5 mt-1">
            {cfg.mention_agent_id && <PropLine label="Mention" value={`@${agentName(cfg.mention_agent_id)}`} />}
            {cfg.content_template && <PropLine label="Content" value={cfg.content_template} mono />}
            {cfg.event_types && cfg.event_types.length > 0 && (
              <PropLine label="Events" value={cfg.event_types.join(", ")} />
            )}
            {cfg.repos && cfg.repos.length > 0 && (
              <PropLine label="Repos" value={cfg.repos.join(", ")} />
            )}
          </div>
        );
      })()}
    </div>
  );
}

function PropLine({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex gap-2">
      <span className="font-medium w-24 shrink-0">{label}:</span>
      {mono ? <code className="break-all">{value}</code> : <span className="truncate">{value}</span>}
    </div>
  );
}

// ----------------------------------------------------------------------------
// Create dialog
// ----------------------------------------------------------------------------

function CreateWebhookDialog({
  open,
  onOpenChange,
  agents,
  daemons,
  adapters,
  apiBaseUrl,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  agents: Agent[];
  daemons: Daemon[];
  adapters: AdapterInfo[];
  apiBaseUrl: string;
  onCreated: (data: { token: string; webhookId: string; url: string; sourceType: string }) => void;
}) {
  const [name, setName] = useState("");
  const [sourceType, setSourceType] = useState<WebhookSourceType>("standard");
  const [agentId, setAgentId] = useState("");
  const [dispatchProvider, setDispatchProvider] = useState("");
  const [dispatchDaemonId, setDispatchDaemonId] = useState("");
  const [titleTemplate, setTitleTemplate] = useState("");
  const [descriptionTemplate, setDescriptionTemplate] = useState("");
  const [selectedLabels, setSelectedLabels] = useState<IssueLabel[]>([]);
  const [creating, setCreating] = useState(false);
  const [showSchema, setShowSchema] = useState(false);
  const allLabels = useLabelStore((s) => s.labels);

  const activeAgents = agents.filter((a: Agent) => !a.archived_at);
  const selectedAgent = activeAgents.find((a) => a.id === agentId);
  const selectableAdapters = adapters.filter((a) => a.source_type !== "github");
  const currentAdapter = adapters.find((a) => a.source_type === sourceType);

  useEffect(() => {
    if (open && activeAgents.length > 0 && !agentId) {
      setAgentId(activeAgents[0]!.id);
    }
  }, [open, activeAgents, agentId]);

  const handleCreate = async () => {
    setCreating(true);
    try {
      const result: CreateWebhookResponse = await api.createWebhook({
        name,
        source_type: sourceType,
        agent_id: agentId,
        title_template: titleTemplate || undefined,
        description_template: descriptionTemplate || undefined,
        labels: selectedLabels.length > 0 ? selectedLabels.map((l) => l.id) : undefined,
        dispatch_provider: dispatchProvider || undefined,
        dispatch_daemon_id: dispatchDaemonId || undefined,
      });
      onCreated({ token: result.token, webhookId: result.webhook.id, url: `${apiBaseUrl}/api/webhooks/${result.webhook.id}`, sourceType });
      onOpenChange(false);
      setName("");
      setSourceType("standard");
      setAgentId("");
      setDispatchProvider("");
      setDispatchDaemonId("");
      setTitleTemplate("");
      setDescriptionTemplate("");
      setSelectedLabels([]);
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
            Create a token-authenticated endpoint for external systems. GitHub webhooks are
            managed automatically through the GitHub App connect flow.
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
              <Select value={sourceType} onValueChange={(v) => { if (v) { setSourceType(v as WebhookSourceType); setShowSchema(false); } }}>
                <SelectTrigger size="sm"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {selectableAdapters.map((a) => (
                    <SelectItem key={a.source_type} value={a.source_type}>
                      {a.source_type}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <SchemaPanel adapter={currentAdapter} show={showSchema} onToggle={() => setShowSchema(!showSchema)} />
          </div>

          <div className="border-t pt-4 space-y-3">
            <div>
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Default Action</h3>
              <p className="text-[11px] text-muted-foreground mt-1">
                A &quot;Create issue&quot; action is created automatically. Add more actions
                (comment, mention, filters) after the webhook exists.
              </p>
            </div>
            <CreateIssueActionFields
              agents={activeAgents}
              selectedAgent={selectedAgent}
              daemons={daemons}
              agentId={agentId}
              setAgentId={setAgentId}
              dispatchProvider={dispatchProvider}
              setDispatchProvider={setDispatchProvider}
              dispatchDaemonId={dispatchDaemonId}
              setDispatchDaemonId={setDispatchDaemonId}
              titleTemplate={titleTemplate}
              setTitleTemplate={setTitleTemplate}
              descriptionTemplate={descriptionTemplate}
              setDescriptionTemplate={setDescriptionTemplate}
              selectedLabels={selectedLabels}
              setSelectedLabels={setSelectedLabels}
              allLabels={allLabels}
            />
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

// ----------------------------------------------------------------------------
// Edit dialog
// ----------------------------------------------------------------------------

function EditWebhookDialog({
  webhook,
  bots,
  agents,
  daemons,
  adapters,
  onOpenChange,
  onUpdated,
}: {
  webhook: WebhookWithActions | null;
  bots: BotUser[];
  agents: Agent[];
  daemons: Daemon[];
  adapters: AdapterInfo[];
  onOpenChange: (v: boolean) => void;
  onUpdated: () => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [botUserId, setBotUserId] = useState<string>("");
  const [actions, setActions] = useState<WebhookAction[]>([]);
  const [saving, setSaving] = useState(false);
  const [showSchema, setShowSchema] = useState(false);
  const allLabels = useLabelStore((s) => s.labels);
  const activeAgents = agents.filter((a: Agent) => !a.archived_at);

  useEffect(() => {
    if (webhook) {
      setName(webhook.webhook.name);
      setBotUserId(webhook.webhook.bot_user_id ?? "");
      setActions(webhook.actions);
    }
  }, [webhook]);

  if (!webhook) return null;

  const sourceType = webhook.webhook.source_type;
  const isGitHub = sourceType === "github";
  const currentAdapter = adapters.find((a) => a.source_type === sourceType);

  const handleAddAction = (actionType: WebhookActionType) => {
    if (!webhook) return;
    const baseConfig: CreateIssueActionConfig | CommentIssueActionConfig =
      actionType === "create_issue"
        ? {
            agent_id: activeAgents[0]?.id ?? "",
            title_template: "",
            description_template: "",
            labels: [],
          }
        : {
            content_template: "{{.body}}",
            mention_agent_id: undefined,
          };
    const draft: WebhookAction = {
      id: `__draft_${Date.now()}`,
      webhook_id: webhook.webhook.id,
      action_type: actionType,
      config: baseConfig,
      enabled: true,
      position: actions.length,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    setActions((prev) => [...prev, draft]);
  };

  const handleRemoveAction = async (action: WebhookAction) => {
    if (!action.id.startsWith("__draft_")) {
      try {
        await api.deleteWebhookAction(webhook.webhook.id, action.id);
      } catch (e) {
        toast.error(e instanceof Error ? e.message : "Failed to delete action");
        return;
      }
    }
    setActions((prev) => prev.filter((a) => a.id !== action.id));
  };

  const handleSave = async () => {
    if (!webhook) return;
    setSaving(true);
    try {
      // 1. Save endpoint-level fields.
      await api.updateWebhook(webhook.webhook.id, {
        name,
        bot_user_id: botUserId || undefined,
      });
      // Backend uses empty string to mean "clear bot user".
      if (!botUserId && webhook.webhook.bot_user_id) {
        await api.updateWebhook(webhook.webhook.id, { bot_user_id: "" });
      }

      // 2. Persist each action: drafts → POST, existing → PUT.
      for (const action of actions) {
        if (action.id.startsWith("__draft_")) {
          await api.createWebhookAction(webhook.webhook.id, {
            action_type: action.action_type,
            config: action.config,
            enabled: action.enabled,
            position: action.position,
          });
        } else {
          await api.updateWebhookAction(webhook.webhook.id, action.id, {
            config: action.config,
            enabled: action.enabled,
            position: action.position,
          });
        }
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

  const updateActionConfig = (idx: number, partial: Record<string, unknown>) => {
    setActions((prev) => prev.map((a, i) => i === idx ? { ...a, config: { ...(a.config as object), ...partial } } : a));
  };

  return (
    <Dialog open={!!webhook} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Edit Webhook</DialogTitle>
          <DialogDescription>
            Configure the endpoint and the actions that fire on incoming events.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-3">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Endpoint</h3>
            <div className="space-y-1.5">
              <Label htmlFor="edit-wh-name">Name</Label>
              <Input id="edit-wh-name" value={name} onChange={(e) => setName(e.target.value)} disabled={isGitHub} />
            </div>
            <div className="space-y-1.5">
              <Label>Source Type</Label>
              <Input value={sourceType} disabled />
            </div>
            <div className="space-y-1.5">
              <Label>Bot user (for comment_issue actions)</Label>
              <Select value={botUserId} onValueChange={(v) => setBotUserId(v ?? "")}>
                <SelectTrigger size="sm"><SelectValue placeholder="None" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="">None</SelectItem>
                  {bots.map((b) => (
                    <SelectItem key={b.id} value={b.id}>{b.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {bots.length === 0 && (
                <p className="text-[10px] text-muted-foreground">
                  Create a bot user above first.
                </p>
              )}
            </div>
            <SchemaPanel adapter={currentAdapter} show={showSchema} onToggle={() => setShowSchema(!showSchema)} />
          </div>

          <div className="border-t pt-4 space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Actions</h3>
              <Popover>
                <PopoverTrigger
                  render={
                    <Button size="sm" variant="outline">
                      <Plus className="h-3 w-3 mr-1" />
                      Add action
                    </Button>
                  }
                />
                <PopoverContent align="end" className="w-44 p-1">
                  <button
                    type="button"
                    onClick={() => handleAddAction("create_issue")}
                    className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm hover:bg-accent"
                  >
                    Create issue
                  </button>
                  <button
                    type="button"
                    onClick={() => handleAddAction("comment_issue")}
                    className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm hover:bg-accent"
                  >
                    Comment on linked issue
                  </button>
                </PopoverContent>
              </Popover>
            </div>

            {actions.length === 0 && (
              <Card>
                <CardContent className="py-4 text-center text-xs text-muted-foreground">
                  No actions yet. Add one to react to incoming events.
                </CardContent>
              </Card>
            )}

            {actions.map((action, idx) => (
              <Card key={action.id} className="border-dashed">
                <CardContent className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="text-xs font-semibold">
                      {action.action_type === "create_issue" ? "Create issue" : "Comment on linked issue"}
                    </div>
                    <Button variant="ghost" size="icon-sm" onClick={() => handleRemoveAction(action)}>
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>

                  {action.action_type === "create_issue" && (
                    <CreateIssueActionEditor
                      cfg={action.config as CreateIssueActionConfig}
                      agents={activeAgents}
                      daemons={daemons}
                      allLabels={allLabels}
                      onChange={(partial) => updateActionConfig(idx, partial)}
                      isGitHub={isGitHub}
                    />
                  )}
                  {action.action_type === "comment_issue" && (
                    <CommentIssueActionEditor
                      cfg={action.config as CommentIssueActionConfig}
                      agents={activeAgents}
                      onChange={(partial) => updateActionConfig(idx, partial)}
                      isGitHub={isGitHub}
                    />
                  )}
                </CardContent>
              </Card>
            ))}
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleSave} disabled={saving || !name.trim()}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ----------------------------------------------------------------------------
// Action editors
// ----------------------------------------------------------------------------

function CreateIssueActionFields({
  agents,
  selectedAgent,
  daemons,
  agentId,
  setAgentId,
  dispatchProvider,
  setDispatchProvider,
  dispatchDaemonId,
  setDispatchDaemonId,
  titleTemplate,
  setTitleTemplate,
  descriptionTemplate,
  setDescriptionTemplate,
  selectedLabels,
  setSelectedLabels,
  allLabels,
}: {
  agents: Agent[];
  selectedAgent: Agent | undefined;
  daemons: Daemon[];
  agentId: string;
  setAgentId: (v: string) => void;
  dispatchProvider: string;
  setDispatchProvider: (v: string) => void;
  dispatchDaemonId: string;
  setDispatchDaemonId: (v: string) => void;
  titleTemplate: string;
  setTitleTemplate: (v: string) => void;
  descriptionTemplate: string;
  setDescriptionTemplate: (v: string) => void;
  selectedLabels: IssueLabel[];
  setSelectedLabels: (v: IssueLabel[]) => void;
  allLabels: IssueLabel[];
}) {
  return (
    <>
      <div className="flex items-center gap-3">
        <Label className="w-24 shrink-0 text-right">Agent</Label>
        <Select value={agentId} onValueChange={(v) => { if (v) setAgentId(v); }}>
          <SelectTrigger size="sm" className="w-auto">
            <SelectValue placeholder="Select agent">
              {agents.find((a) => a.id === agentId)?.name}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {agents.map((a: Agent) => (
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
      <div className="space-y-1.5">
        <Label>Labels <span className="text-muted-foreground">(optional)</span></Label>
        <WebhookLabelPicker labels={allLabels} selected={selectedLabels} onChange={setSelectedLabels} />
      </div>
    </>
  );
}

function CreateIssueActionEditor({
  cfg,
  agents,
  daemons,
  allLabels,
  onChange,
  isGitHub,
}: {
  cfg: CreateIssueActionConfig;
  agents: Agent[];
  daemons: Daemon[];
  allLabels: IssueLabel[];
  onChange: (partial: Partial<CreateIssueActionConfig>) => void;
  isGitHub: boolean;
}) {
  const selectedAgent = agents.find((a) => a.id === cfg.agent_id);
  const selectedLabels = (cfg.labels ?? []).map((id) => allLabels.find((l) => l.id === id)).filter((l): l is IssueLabel => !!l);

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-3">
        <Label className="w-24 shrink-0 text-right">Agent</Label>
        <Select value={cfg.agent_id} onValueChange={(v) => v && onChange({ agent_id: v })}>
          <SelectTrigger size="sm" className="w-auto"><SelectValue placeholder="Select agent" /></SelectTrigger>
          <SelectContent>
            {agents.map((a) => (<SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>))}
          </SelectContent>
        </Select>
      </div>
      {selectedAgent && selectedAgent.providers.length > 0 && (
        <div className="flex items-center gap-3">
          <Label className="w-24 shrink-0 text-right">Provider</Label>
          <Select value={cfg.dispatch_provider ?? ""} onValueChange={(v) => onChange({ dispatch_provider: v || undefined })}>
            <SelectTrigger size="sm" className="w-auto"><SelectValue placeholder="Auto" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="">Auto</SelectItem>
              {selectedAgent.providers.map((p) => (<SelectItem key={p} value={p}>{p}</SelectItem>))}
            </SelectContent>
          </Select>
        </div>
      )}
      <div className="flex items-center gap-3">
        <Label className="w-24 shrink-0 text-right">Environment</Label>
        <Select value={cfg.dispatch_daemon_id ?? ""} onValueChange={(v) => onChange({ dispatch_daemon_id: v || undefined })}>
          <SelectTrigger size="sm" className="w-auto"><SelectValue placeholder="Any environment" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="">Any environment</SelectItem>
            {daemons.map((d) => (<SelectItem key={d.id} value={d.id}>{d.device_name || d.daemon_id}</SelectItem>))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1.5">
        <Label>Title template</Label>
        <Input value={cfg.title_template ?? ""} onChange={(e) => onChange({ title_template: e.target.value })} placeholder="{{.title}}" />
      </div>
      <div className="space-y-1.5">
        <Label>Description template</Label>
        <Textarea value={cfg.description_template ?? ""} onChange={(e) => onChange({ description_template: e.target.value })} rows={2} placeholder="{{.body}}" />
      </div>
      <div className="space-y-1.5">
        <Label>Labels</Label>
        <WebhookLabelPicker
          labels={allLabels}
          selected={selectedLabels}
          onChange={(picks) => onChange({ labels: picks.map((l) => l.id) })}
        />
      </div>
      {isGitHub && (
        <FilterFields
          eventTypes={cfg.event_types ?? []}
          repos={cfg.repos ?? []}
          onChange={(partial) => onChange(partial)}
        />
      )}
    </div>
  );
}

function CommentIssueActionEditor({
  cfg,
  agents,
  onChange,
  isGitHub,
}: {
  cfg: CommentIssueActionConfig;
  agents: Agent[];
  onChange: (partial: Partial<CommentIssueActionConfig>) => void;
  isGitHub: boolean;
}) {
  return (
    <div className="space-y-2">
      <div className="space-y-1.5">
        <Label>Comment template</Label>
        <Textarea
          value={cfg.content_template ?? ""}
          onChange={(e) => onChange({ content_template: e.target.value })}
          rows={3}
          placeholder="{{.body}}"
        />
      </div>
      <div className="flex items-center gap-3">
        <Label className="w-24 shrink-0 text-right">@Mention</Label>
        <Select value={cfg.mention_agent_id ?? ""} onValueChange={(v) => onChange({ mention_agent_id: v || undefined })}>
          <SelectTrigger size="sm" className="w-auto"><SelectValue placeholder="No mention" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="">No mention</SelectItem>
            {agents.map((a) => (<SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>))}
          </SelectContent>
        </Select>
      </div>
      {isGitHub && (
        <FilterFields
          eventTypes={cfg.event_types ?? []}
          repos={cfg.repos ?? []}
          onChange={(partial) => onChange(partial)}
        />
      )}
    </div>
  );
}

function FilterFields({
  eventTypes,
  repos,
  onChange,
}: {
  eventTypes: string[];
  repos: string[];
  onChange: (partial: { event_types?: string[]; repos?: string[] }) => void;
}) {
  const toggleEvent = (val: string) => {
    if (eventTypes.includes(val)) {
      onChange({ event_types: eventTypes.filter((t) => t !== val) });
    } else {
      onChange({ event_types: [...eventTypes, val] });
    }
  };

  const reposText = repos.join(", ");

  return (
    <div className="rounded-md border bg-muted/30 p-2 space-y-2">
      <div className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Filters (GitHub)</div>
      <div className="space-y-1.5">
        <Label className="text-[11px]">Event types (any selected)</Label>
        <div className="flex flex-wrap gap-1">
          {GITHUB_EVENT_TYPES.map((et) => {
            const on = eventTypes.includes(et.value);
            return (
              <button
                key={et.value}
                type="button"
                onClick={() => toggleEvent(et.value)}
                className={`rounded-full border px-2 py-0.5 text-[11px] transition-colors ${on ? "bg-primary text-primary-foreground" : "bg-background hover:bg-accent"}`}
              >
                {et.label}
              </button>
            );
          })}
        </div>
      </div>
      <div className="space-y-1.5">
        <Label className="text-[11px]">Repos (comma-separated owner/repo, leave blank for all)</Label>
        <Input
          value={reposText}
          onChange={(e) => {
            const list = e.target.value
              .split(",")
              .map((s) => s.trim())
              .filter((s) => s.length > 0);
            onChange({ repos: list });
          }}
          placeholder="owner/repo, owner/repo"
        />
      </div>
    </div>
  );
}

// ----------------------------------------------------------------------------
// Schema panel (adapter doc)
// ----------------------------------------------------------------------------

function SchemaPanel({
  adapter,
  show,
  onToggle,
}: {
  adapter: AdapterInfo | undefined;
  show: boolean;
  onToggle: () => void;
}) {
  if (!adapter) return null;
  return (
    <div className="rounded-md border bg-muted/30 p-3 space-y-2">
      <p className="text-xs text-muted-foreground">{adapter.description}</p>
      <button
        type="button"
        onClick={onToggle}
        className="text-xs text-primary hover:underline flex items-center gap-1"
      >
        {show ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
        {show ? "Hide" : "Show"} payload schema & example
      </button>
      {show && (
        <div className="space-y-2 pt-1">
          <div>
            <p className="text-[10px] font-medium text-muted-foreground mb-1">Available template variables:</p>
            <div className="grid gap-0.5">
              {adapter.keys.map((k) => (
                <div key={k.key} className="flex items-start gap-2 text-[10px]">
                  <code className="font-mono text-primary shrink-0">{`{{.${k.key}}}`}</code>
                  <span className="text-muted-foreground">{k.description}</span>
                  {k.required && <Badge variant="outline" className="text-[8px] px-1 py-0 shrink-0">required</Badge>}
                </div>
              ))}
            </div>
          </div>
          {adapter.example && (
            <div>
              <p className="text-[10px] font-medium text-muted-foreground mb-1">Example payload:</p>
              <pre className="rounded bg-muted p-2 text-[10px] overflow-x-auto whitespace-pre">
                {adapter.example}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ----------------------------------------------------------------------------
// Label picker (existing implementation)
// ----------------------------------------------------------------------------

function WebhookLabelPicker({ labels, selected, onChange }: {
  labels: IssueLabel[];
  selected: IssueLabel[];
  onChange: (labels: IssueLabel[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");

  const toggle = (label: IssueLabel) => {
    onChange(
      selected.some((l) => l.id === label.id)
        ? selected.filter((l) => l.id !== label.id)
        : [...selected, label],
    );
  };

  const filtered = labels.filter((l) => l.name.toLowerCase().includes(filter.toLowerCase()));

  return (
    <Popover open={open} onOpenChange={(v) => { setOpen(v); if (!v) setFilter(""); }}>
      <PopoverTrigger
        render={
          <button
            type="button"
            className="flex flex-wrap items-center gap-1 rounded-md border bg-transparent px-3 py-2 text-sm min-h-9 w-full text-left hover:bg-accent/50 transition-colors"
          >
            {selected.length > 0 ? (
              selected.map((l) => (
                <span
                  key={l.id}
                  className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs"
                  style={{ backgroundColor: l.color + "20" }}
                >
                  <span className="h-2 w-2 rounded-full shrink-0" style={{ backgroundColor: l.color }} />
                  {l.name}
                </span>
              ))
            ) : (
              <span className="text-muted-foreground">Select labels...</span>
            )}
          </button>
        }
      />
      <PopoverContent align="start" className="w-52 p-0">
        <div className="px-2 py-1.5 border-b">
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter labels..."
            className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none"
          />
        </div>
        <div className="p-1 max-h-60 overflow-y-auto">
          {filtered.map((label) => (
            <button
              key={label.id}
              type="button"
              onClick={() => toggle(label)}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
            >
              <span className="h-3 w-3 rounded-full shrink-0" style={{ backgroundColor: label.color }} />
              <span className="flex-1 text-left truncate">{label.name}</span>
              {selected.some((l) => l.id === label.id) && (
                <Check className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              )}
            </button>
          ))}
          {labels.length === 0 && (
            <div className="px-2 py-3 text-center text-sm text-muted-foreground">No labels yet</div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
