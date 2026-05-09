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
  GitBranch as Github,
  History,
  ExternalLink,
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
  WebhookEvent,
  WebhookEventStatus,
  MemberWithUser,
} from "@/shared/types";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  AgentPicker,
  AgentDispatchTriggerContent,
} from "@/features/issues/components/pickers";
import type { AgentPickerValue } from "@/features/issues/components/pickers";
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
import { issueUrl } from "@/features/issues/utils/url";
import { useLabelStore } from "@/features/labels";
import type { Label as IssueLabel } from "@/shared/types";

// GitHub event groups for the action filter UI. Each group has a parent
// prefix (e.g. "github.pull_request") and an optional set of leaf actions
// (e.g. "opened", "merged"). Selecting the parent stores the prefix string
// and matches all sub-actions via the backend's prefix-match. Selecting
// individual leaves stores the full event type "github.<event>.<action>".
//
// The set of leaves mirrors the actions accepted by isRelevantGitHubAction
// in server/internal/webhook/github.go.
const GITHUB_EVENT_GROUPS: {
  parent: string;
  label: string;
  actions: { value: string; label: string }[];
}[] = [
  { parent: "github.push", label: "Push", actions: [] },
  {
    parent: "github.pull_request",
    label: "Pull request",
    actions: [
      { value: "opened", label: "Opened" },
      { value: "synchronize", label: "Synchronize" },
      { value: "reopened", label: "Reopened" },
      { value: "closed", label: "Closed (unmerged)" },
      { value: "merged", label: "Merged" },
      { value: "labeled", label: "Labeled" },
    ],
  },
  {
    parent: "github.issues",
    label: "Issue",
    actions: [
      { value: "opened", label: "Opened" },
      { value: "reopened", label: "Reopened" },
      { value: "closed", label: "Closed" },
      { value: "labeled", label: "Labeled" },
    ],
  },
  {
    parent: "github.issue_comment",
    label: "Comment",
    actions: [{ value: "created", label: "Created" }],
  },
];

// Convert a GitHub repo URL ("https://github.com/owner/repo[.git]") into
// its full name ("owner/repo"). Returns null when the URL doesn't look
// like a GitHub repo URL.
function repoFullNameFromURL(url: string): string | null {
  const m = url.match(/github\.com[:/]+([^/]+)\/([^/.]+)(?:\.git)?\/?$/);
  return m ? `${m[1]}/${m[2]}` : null;
}

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
  const members = useWorkspaceStore((s) => s.members);

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

  const memberName = (userId: string) => {
    const m = members.find((m: MemberWithUser) => m.user_id === userId);
    return m?.name ?? "(deleted member)";
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
          Webhooks let external systems (alerting, monitoring, CI, GitHub) push events that
          create issues or comment on existing ones. Bot users (used as the comment author)
          are managed in the Members tab.
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
                memberName={memberName}
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

      <WebhookEventsSection webhooks={webhooks} />

      <CreateWebhookDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        members={members}
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
        members={members}
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
// Webhook card
// ----------------------------------------------------------------------------

function WebhookCard({
  wh,
  bots,
  agentName,
  memberName,
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
  memberName: (userId: string) => string;
  envName: (daemonId: string) => string;
  apiBaseUrl: string;
  onToggle: () => void;
  onEdit: () => void;
  onRegenerate: () => void;
  onDelete: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [eventsExpanded, setEventsExpanded] = useState(false);
  const [events, setEvents] = useState<WebhookEvent[] | null>(null);
  const [eventsLoading, setEventsLoading] = useState(false);
  const webhook = wh.webhook;
  const isActive = webhook.status === "active";
  const isGitHub = webhook.source_type === "github";

  useEffect(() => {
    if (!eventsExpanded || events !== null) return;
    setEventsLoading(true);
    api.listWebhookEvents(webhook.id)
      .then(setEvents)
      .catch(() => setEvents([]))
      .finally(() => setEventsLoading(false));
  }, [eventsExpanded, events, webhook.id]);
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
                memberName={memberName}
                envName={envName}
              />
            ))}
            {wh.actions.length === 0 && (
              <div className="italic text-muted-foreground/70">No actions configured</div>
            )}
            <WebhookEventsPanel
              events={events}
              loading={eventsLoading}
              expanded={eventsExpanded}
              onToggle={() => setEventsExpanded((v) => !v)}
            />
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
  memberName,
  envName,
}: {
  action: WebhookAction;
  index: number;
  agentName: (id: string) => string;
  memberName: (id: string) => string;
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
            {cfg.github_labels && cfg.github_labels.length > 0 && (
              <PropLine label="GitHub labels" value={cfg.github_labels.join(", ")} />
            )}
            {cfg.subscriber_ids && cfg.subscriber_ids.length > 0 && (
              <PropLine label="Subscribers" value={cfg.subscriber_ids.map(memberName).join(", ")} />
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
            {cfg.github_labels && cfg.github_labels.length > 0 && (
              <PropLine label="GitHub labels" value={cfg.github_labels.join(", ")} />
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
  members,
  adapters,
  apiBaseUrl,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  members: MemberWithUser[];
  adapters: AdapterInfo[];
  apiBaseUrl: string;
  onCreated: (data: { token: string; webhookId: string; url: string; sourceType: string }) => void;
}) {
  const [name, setName] = useState("");
  const [sourceType, setSourceType] = useState<WebhookSourceType>("standard");
  const [agentDispatch, setAgentDispatch] = useState<AgentPickerValue>({
    agentId: null,
    daemonId: null,
    provider: null,
  });
  const [titleTemplate, setTitleTemplate] = useState("");
  const [descriptionTemplate, setDescriptionTemplate] = useState("");
  const [selectedLabels, setSelectedLabels] = useState<IssueLabel[]>([]);
  const [subscriberIds, setSubscriberIds] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);
  const [showSchema, setShowSchema] = useState(false);
  const allLabels = useLabelStore((s) => s.labels);

  const selectableAdapters = adapters.filter((a) => a.source_type !== "github");
  const currentAdapter = adapters.find((a) => a.source_type === sourceType);

  const handleCreate = async () => {
    if (!agentDispatch.agentId) return;
    setCreating(true);
    try {
      const result: CreateWebhookResponse = await api.createWebhook({
        name,
        source_type: sourceType,
        agent_id: agentDispatch.agentId,
        title_template: titleTemplate || undefined,
        description_template: descriptionTemplate || undefined,
        labels: selectedLabels.length > 0 ? selectedLabels.map((l) => l.id) : undefined,
        dispatch_provider: agentDispatch.provider ?? undefined,
        dispatch_daemon_id: agentDispatch.daemonId ?? undefined,
        subscriber_ids: subscriberIds.length > 0 ? subscriberIds : undefined,
      });
      onCreated({ token: result.token, webhookId: result.webhook.id, url: `${apiBaseUrl}/api/webhooks/${result.webhook.id}`, sourceType });
      onOpenChange(false);
      setName("");
      setSourceType("standard");
      setAgentDispatch({ agentId: null, daemonId: null, provider: null });
      setTitleTemplate("");
      setDescriptionTemplate("");
      setSelectedLabels([]);
      setSubscriberIds([]);
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
              members={members}
              agentDispatch={agentDispatch}
              setAgentDispatch={setAgentDispatch}
              titleTemplate={titleTemplate}
              setTitleTemplate={setTitleTemplate}
              descriptionTemplate={descriptionTemplate}
              setDescriptionTemplate={setDescriptionTemplate}
              selectedLabels={selectedLabels}
              setSelectedLabels={setSelectedLabels}
              allLabels={allLabels}
              subscriberIds={subscriberIds}
              setSubscriberIds={setSubscriberIds}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleCreate} disabled={creating || !name.trim() || !agentDispatch.agentId}>
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
  members,
  adapters,
  onOpenChange,
  onUpdated,
}: {
  webhook: WebhookWithActions | null;
  bots: BotUser[];
  agents: Agent[];
  members: MemberWithUser[];
  adapters: AdapterInfo[];
  onOpenChange: (v: boolean) => void;
  onUpdated: () => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [botUserId, setBotUserId] = useState<string>("");
  const [dedupWindow, setDedupWindow] = useState<string>("");
  const [actions, setActions] = useState<WebhookAction[]>([]);
  const [saving, setSaving] = useState(false);
  const [showSchema, setShowSchema] = useState(false);
  const allLabels = useLabelStore((s) => s.labels);
  const activeAgents = agents.filter((a: Agent) => !a.archived_at);

  useEffect(() => {
    if (webhook) {
      setName(webhook.webhook.name);
      setBotUserId(webhook.webhook.bot_user_id ?? "");
      setDedupWindow(String(webhook.webhook.dedup_window_seconds));
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
            // Empty agent_id forces the user to pick + confirm an agent dispatch
            // before saving — `handleSave` rejects drafts with no agent.
            agent_id: "",
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

  const dedupWindowSeconds = parseInt(dedupWindow, 10);
  const dedupWindowValid = !isNaN(dedupWindowSeconds) && dedupWindowSeconds >= 0;
  const allActionsValid = actions.every((a) => {
    if (a.action_type !== "create_issue") return true;
    return !!(a.config as CreateIssueActionConfig).agent_id;
  });

  const handleSave = async () => {
    if (!webhook) return;
    if (!allActionsValid) {
      toast.error("Pick an agent for every Create issue action");
      return;
    }
    setSaving(true);
    try {
      // 1. Save endpoint-level fields.
      await api.updateWebhook(webhook.webhook.id, {
        name,
        bot_user_id: botUserId || undefined,
        dedup_window_seconds: dedupWindowSeconds,
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
              <Label htmlFor="edit-wh-dedup">Dedup window (seconds)</Label>
              <Input
                id="edit-wh-dedup"
                type="number"
                min={0}
                value={dedupWindow}
                onChange={(e) => setDedupWindow(e.target.value)}
                className={!dedupWindowValid ? "border-destructive" : ""}
              />
              <p className="text-[10px] text-muted-foreground">
                How long to suppress duplicate events. Set to 0 to disable deduplication.
              </p>
            </div>
            <div className="space-y-1.5">
              <Label>Bot user (for comment_issue actions)</Label>
              <Select value={botUserId} onValueChange={(v) => setBotUserId(v ?? "")}>
                <SelectTrigger size="sm">
                  <SelectValue placeholder="None">
                    {botUserId
                      ? (bots.find((b) => b.id === botUserId)?.name ?? "(deleted bot)")
                      : "None"}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">None</SelectItem>
                  {bots.map((b) => (
                    <SelectItem key={b.id} value={b.id}>{b.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {bots.length === 0 ? (
                <p className="text-[10px] text-muted-foreground">
                  No bot users exist. Create one in Settings &rarr; Members.
                </p>
              ) : (
                <p className="text-[10px] text-muted-foreground">
                  Manage bot users in Settings &rarr; Members.
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
                      members={members}
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
          <Button onClick={handleSave} disabled={saving || !name.trim() || !dedupWindowValid || !allActionsValid}>
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
  members,
  agentDispatch,
  setAgentDispatch,
  titleTemplate,
  setTitleTemplate,
  descriptionTemplate,
  setDescriptionTemplate,
  selectedLabels,
  setSelectedLabels,
  allLabels,
  subscriberIds,
  setSubscriberIds,
}: {
  members: MemberWithUser[];
  agentDispatch: AgentPickerValue;
  setAgentDispatch: (v: AgentPickerValue) => void;
  titleTemplate: string;
  setTitleTemplate: (v: string) => void;
  descriptionTemplate: string;
  setDescriptionTemplate: (v: string) => void;
  selectedLabels: IssueLabel[];
  setSelectedLabels: (v: IssueLabel[]) => void;
  allLabels: IssueLabel[];
  subscriberIds: string[];
  setSubscriberIds: (v: string[]) => void;
}) {
  return (
    <>
      <div className="space-y-1.5">
        <Label>Agent dispatch</Label>
        <AgentPicker
          value={agentDispatch}
          onChange={setAgentDispatch}
          align="start"
          triggerRender={
            <button
              type="button"
              className="flex w-full items-center gap-2 rounded-md border bg-transparent px-3 py-2 text-sm min-h-9 hover:bg-accent/50 transition-colors"
            />
          }
          trigger={<AgentDispatchTriggerContent value={agentDispatch} placeholder="Select agent" />}
        />
        <p className="text-[10px] text-muted-foreground">
          Picking an agent confirms its provider and environment together.
        </p>
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
      <div className="space-y-1.5">
        <Label>Subscribers <span className="text-muted-foreground">(optional)</span></Label>
        <WebhookMemberPicker members={members} selectedIds={subscriberIds} onChange={setSubscriberIds} />
      </div>
    </>
  );
}

function CreateIssueActionEditor({
  cfg,
  members,
  allLabels,
  onChange,
  isGitHub,
}: {
  cfg: CreateIssueActionConfig;
  members: MemberWithUser[];
  allLabels: IssueLabel[];
  onChange: (partial: Partial<CreateIssueActionConfig>) => void;
  isGitHub: boolean;
}) {
  const selectedLabels = (cfg.labels ?? []).map((id) => allLabels.find((l) => l.id === id)).filter((l): l is IssueLabel => !!l);

  const agentDispatch: AgentPickerValue = {
    agentId: cfg.agent_id || null,
    daemonId: cfg.dispatch_daemon_id ?? null,
    provider: cfg.dispatch_provider ?? null,
  };

  return (
    <div className="space-y-2">
      <div className="space-y-1.5">
        <Label>Agent dispatch</Label>
        <AgentPicker
          value={agentDispatch}
          onChange={(next) =>
            onChange({
              agent_id: next.agentId ?? "",
              dispatch_provider: next.provider ?? undefined,
              dispatch_daemon_id: next.daemonId ?? undefined,
            })
          }
          align="start"
          triggerRender={
            <button
              type="button"
              className="flex w-full items-center gap-2 rounded-md border bg-transparent px-3 py-2 text-sm min-h-9 hover:bg-accent/50 transition-colors"
            />
          }
          trigger={<AgentDispatchTriggerContent value={agentDispatch} placeholder="Select agent" />}
        />
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
      <div className="space-y-1.5">
        <Label>Subscribers</Label>
        <WebhookMemberPicker
          members={members}
          selectedIds={cfg.subscriber_ids ?? []}
          onChange={(ids) => onChange({ subscriber_ids: ids.length > 0 ? ids : undefined })}
        />
      </div>
      {isGitHub && (
        <FilterFields
          eventTypes={cfg.event_types ?? []}
          repos={cfg.repos ?? []}
          githubLabels={cfg.github_labels ?? []}
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
          <SelectTrigger size="sm" className="w-auto">
            <SelectValue placeholder="No mention">
              {cfg.mention_agent_id
                ? (agents.find((a) => a.id === cfg.mention_agent_id)?.name ?? "(deleted agent)")
                : "No mention"}
            </SelectValue>
          </SelectTrigger>
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
          githubLabels={cfg.github_labels ?? []}
          onChange={(partial) => onChange(partial)}
        />
      )}
    </div>
  );
}

function FilterFields({
  eventTypes,
  repos,
  githubLabels,
  onChange,
}: {
  eventTypes: string[];
  repos: string[];
  githubLabels: string[];
  onChange: (partial: { event_types?: string[]; repos?: string[]; github_labels?: string[] }) => void;
}) {
  // Repository options come from the workspace's registered repos (Settings
  // -> Repositories). We display owner/repo and store owner/repo so the
  // filter matches the value the github adapter writes into Event.Data.repo.
  const workspace = useWorkspaceStore((s) => s.workspace);
  const availableRepos = (workspace?.repos ?? [])
    .map((r) => ({ url: r.url, fullName: repoFullNameFromURL(r.url), description: r.description }))
    .filter((r): r is { url: string; fullName: string; description: string } => r.fullName !== null);

  const setEventTypes = (next: string[]) => onChange({ event_types: next });
  const setRepos = (next: string[]) => onChange({ repos: next });
  const setGitHubLabels = (next: string[]) => onChange({ github_labels: next });

  return (
    <div className="rounded-md border bg-muted/30 p-2 space-y-2">
      <div className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Filters (GitHub)</div>

      <EventTypePicker eventTypes={eventTypes} onChange={setEventTypes} />

      <div className="space-y-1.5">
        <Label className="text-[11px]">Repos (leave empty for all)</Label>
        <RepoPicker
          available={availableRepos}
          selected={repos}
          onChange={setRepos}
        />
      </div>

      <div className="space-y-1.5">
        <Label className="text-[11px]">GitHub labels (leave empty for all)</Label>
        <GitHubLabelFilterInput selected={githubLabels} onChange={setGitHubLabels} />
        <p className="text-[10px] text-muted-foreground">
          Only fire when the PR/issue carries any of these GitHub labels. A `labeled` event with one of these labels also triggers.
        </p>
      </div>
    </div>
  );
}

// Free-text multi-entry input for GitHub label filters. The list is stored
// as plain label names — they are matched against the comma-separated label
// names emitted by the github adapter. We accept Enter / comma to commit a
// chip, and Backspace on an empty input to remove the last entry.
function GitHubLabelFilterInput({
  selected,
  onChange,
}: {
  selected: string[];
  onChange: (next: string[]) => void;
}) {
  const [draft, setDraft] = useState("");

  const addLabel = (raw: string) => {
    const name = raw.trim();
    if (!name) return;
    if (selected.includes(name)) {
      setDraft("");
      return;
    }
    onChange([...selected, name]);
    setDraft("");
  };

  const removeLabel = (name: string) => {
    onChange(selected.filter((s) => s !== name));
  };

  return (
    <div className="flex flex-wrap items-center gap-1 rounded-md border bg-background px-2 py-1.5 min-h-9 focus-within:ring-1 focus-within:ring-ring">
      {selected.map((name) => (
        <span
          key={name}
          className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-[11px]"
        >
          {name}
          <button
            type="button"
            aria-label={`Remove label ${name}`}
            onClick={() => removeLabel(name)}
            className="text-muted-foreground hover:text-foreground"
          >
            ×
          </button>
        </span>
      ))}
      <input
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === ",") {
            e.preventDefault();
            addLabel(draft);
          } else if (e.key === "Backspace" && draft === "" && selected.length > 0) {
            removeLabel(selected[selected.length - 1]!);
          }
        }}
        onBlur={() => addLabel(draft)}
        className="flex-1 min-w-[120px] bg-transparent text-sm outline-none placeholder:text-muted-foreground"
        placeholder={selected.length === 0 ? "All labels" : "Add label..."}
      />
    </div>
  );
}

// Hierarchical event-type picker: each group has a "select all" toggle and
// a flat list of leaf chips. Storage rules (so the existing
// actionMatchesFilters prefix-match keeps working unchanged):
//   - Selecting the parent stores the parent prefix (e.g. "github.pull_request")
//     and removes any of that group's leaf entries.
//   - Selecting a leaf stores the full event type (e.g. "github.pull_request.opened")
//     and removes the parent prefix.
function EventTypePicker({
  eventTypes,
  onChange,
}: {
  eventTypes: string[];
  onChange: (next: string[]) => void;
}) {
  const isParentSelected = (parent: string) => eventTypes.includes(parent);
  const isLeafSelected = (parent: string, action: string) =>
    eventTypes.includes(`${parent}.${action}`);

  const toggleParent = (parent: string, hasLeaves: boolean) => {
    const others = eventTypes.filter(
      (t) => t !== parent && !t.startsWith(`${parent}.`),
    );
    if (isParentSelected(parent)) {
      onChange(others);
      return;
    }
    onChange(hasLeaves ? [...others, parent] : [...others, parent]);
  };

  const toggleLeaf = (parent: string, action: string) => {
    const full = `${parent}.${action}`;
    const withoutParent = eventTypes.filter((t) => t !== parent);
    if (isLeafSelected(parent, action)) {
      onChange(withoutParent.filter((t) => t !== full));
    } else {
      onChange([...withoutParent, full]);
    }
  };

  return (
    <div className="space-y-2">
      <Label className="text-[11px]">Event types (any selected)</Label>
      <div className="space-y-1.5">
        {GITHUB_EVENT_GROUPS.map((group) => {
          const parentOn = isParentSelected(group.parent);
          return (
            <div key={group.parent} className="flex flex-wrap items-center gap-1">
              <button
                type="button"
                onClick={() => toggleParent(group.parent, group.actions.length > 0)}
                className={`rounded-full border px-2 py-0.5 text-[11px] font-medium transition-colors ${parentOn ? "bg-primary text-primary-foreground border-primary" : "bg-background hover:bg-accent"}`}
                title={group.actions.length > 0 ? "Match all sub-actions" : undefined}
              >
                {group.label}
                {group.actions.length > 0 ? " (all)" : ""}
              </button>
              {group.actions.map((act) => {
                const on = isLeafSelected(group.parent, act.value);
                return (
                  <button
                    key={act.value}
                    type="button"
                    onClick={() => toggleLeaf(group.parent, act.value)}
                    disabled={parentOn}
                    className={`rounded-full border px-2 py-0.5 text-[11px] transition-colors ${on ? "bg-primary text-primary-foreground border-primary" : "bg-background hover:bg-accent"} ${parentOn ? "opacity-40 cursor-not-allowed" : ""}`}
                  >
                    {act.label}
                  </button>
                );
              })}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// Multi-select popover backed by workspace.repos. Shows "All repos" when
// nothing is selected; selected repos render as chips on the trigger.
function RepoPicker({
  available,
  selected,
  onChange,
}: {
  available: { url: string; fullName: string; description: string }[];
  selected: string[];
  onChange: (next: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const toggle = (fullName: string) => {
    if (selected.includes(fullName)) {
      onChange(selected.filter((r) => r !== fullName));
    } else {
      onChange([...selected, fullName]);
    }
  };

  if (available.length === 0) {
    return (
      <div className="rounded-md border bg-background px-3 py-2 text-[11px] text-muted-foreground">
        No repositories registered. Add them in Settings &rarr; Repositories.
      </div>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <button
            type="button"
            className="flex flex-wrap items-center gap-1 rounded-md border bg-background px-3 py-2 text-sm min-h-9 w-full text-left hover:bg-accent/50 transition-colors"
          >
            {selected.length > 0 ? (
              selected.map((fullName) => (
                <span
                  key={fullName}
                  className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-[11px]"
                >
                  {fullName}
                </span>
              ))
            ) : (
              <span className="text-muted-foreground">All repos</span>
            )}
          </button>
        }
      />
      <PopoverContent align="start" className="w-72 p-0">
        <div className="p-1 max-h-60 overflow-y-auto">
          {available.map((repo) => {
            const on = selected.includes(repo.fullName);
            return (
              <button
                key={repo.url}
                type="button"
                onClick={() => toggle(repo.fullName)}
                className="flex w-full items-start gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent transition-colors"
              >
                <span className="mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded border">
                  {on && <Check className="h-2.5 w-2.5" />}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">{repo.fullName}</span>
                  {repo.description && (
                    <span className="block truncate text-[10px] text-muted-foreground">{repo.description}</span>
                  )}
                </span>
              </button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}

// ----------------------------------------------------------------------------
// Webhook event status helpers
// ----------------------------------------------------------------------------

const EVENT_STATUS_VARIANTS: Record<WebhookEventStatus, { label: string; className: string }> = {
  processed: { label: "processed", className: "bg-green-500/15 text-green-700 dark:text-green-400" },
  filtered: { label: "filtered", className: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400" },
  deduped: { label: "deduped", className: "bg-blue-500/15 text-blue-700 dark:text-blue-400" },
  error: { label: "error", className: "bg-destructive/15 text-destructive" },
};

function EventStatusBadge({ status }: { status: WebhookEventStatus }) {
  const cfg = EVENT_STATUS_VARIANTS[status] ?? EVENT_STATUS_VARIANTS.error;
  return (
    <span className={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium ${cfg.className}`}>
      {cfg.label}
    </span>
  );
}

function formatEventTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  } catch {
    return iso;
  }
}

function payloadSummary(payload: unknown): string {
  if (!payload || typeof payload !== "object") return "";
  const p = payload as Record<string, unknown>;
  const title = p.title ?? p.alertname ?? (p.labels && typeof p.labels === "object" ? (p.labels as Record<string, unknown>).alertname : undefined);
  if (title) return String(title).slice(0, 80);
  return "";
}

// ----------------------------------------------------------------------------
// Per-webhook events panel (shown inside expanded card)
// ----------------------------------------------------------------------------

function WebhookEventsPanel({
  events,
  loading,
  expanded,
  onToggle,
}: {
  events: WebhookEvent[] | null;
  loading: boolean;
  expanded: boolean;
  onToggle: () => void;
}) {
  const displayEvents = events?.slice(0, 20) ?? [];

  return (
    <div className="border-t pt-2 mt-1 space-y-1.5">
      <button
        type="button"
        onClick={onToggle}
        className="flex items-center gap-1 text-[10px] font-semibold uppercase tracking-wide hover:text-foreground text-muted-foreground transition-colors"
      >
        {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
        <History className="h-3 w-3" />
        Recent Events
      </button>
      {expanded && (
        <>
          {loading && (
            <div className="text-muted-foreground/70 italic">Loading events…</div>
          )}
          {!loading && events !== null && displayEvents.length === 0 && (
            <div className="italic text-muted-foreground/70">No events recorded yet</div>
          )}
          {!loading && displayEvents.map((evt) => (
            <WebhookEventRow key={evt.id} event={evt} />
          ))}
          {!loading && events !== null && events.length > 20 && (
            <div className="text-muted-foreground/70 italic">
              Showing 20 of {events.length} events
            </div>
          )}
        </>
      )}
    </div>
  );
}

function WebhookEventRow({ event, webhookName }: { event: WebhookEvent; webhookName?: string }) {
  const summary = payloadSummary(event.payload);
  const workspaceSlug = useWorkspaceStore((s) => s.workspace?.slug ?? "");
  return (
    <div className="flex flex-col gap-0.5 rounded border bg-muted/30 px-2 py-1.5 text-[11px]">
      <div className="flex items-center gap-2 flex-wrap">
        <EventStatusBadge status={event.status as WebhookEventStatus} />
        <span className="text-muted-foreground shrink-0">{formatEventTime(event.created_at)}</span>
        {webhookName && (
          <span className="font-medium truncate max-w-[160px]">{webhookName}</span>
        )}
        {event.dedup_key && (
          <span className="text-muted-foreground/70 truncate max-w-[120px]" title={event.dedup_key}>
            dedup: {event.dedup_key}
          </span>
        )}
        {event.issue_id && (
          <a
            href={issueUrl(event.issue_id, workspaceSlug)}
            className="flex items-center gap-0.5 text-primary hover:underline shrink-0"
          >
            <ExternalLink className="h-2.5 w-2.5" />
            issue
          </a>
        )}
      </div>
      {summary && (
        <span className="text-muted-foreground truncate">{summary}</span>
      )}
      {event.error_message && (
        <span className="text-destructive truncate" title={event.error_message}>
          {event.error_message}
        </span>
      )}
    </div>
  );
}

// ----------------------------------------------------------------------------
// Global webhook events section
// ----------------------------------------------------------------------------

function WebhookEventsSection({
  webhooks,
}: {
  webhooks: WebhookWithActions[];
}) {
  const [sectionExpanded, setSectionExpanded] = useState(false);
  const [events, setEvents] = useState<WebhookEvent[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [showAll, setShowAll] = useState(false);

  useEffect(() => {
    if (!sectionExpanded || events !== null) return;
    setLoading(true);
    api.listWorkspaceWebhookEvents()
      .then(setEvents)
      .catch(() => setEvents([]))
      .finally(() => setLoading(false));
  }, [sectionExpanded, events]);

  const webhookNameMap = new Map(webhooks.map((w) => [w.webhook.id, w.webhook.name]));
  const problemEvents = (events ?? []).filter((e) => e.status !== "processed");
  const displayEvents = showAll ? (events ?? []) : (events ?? []).slice(0, 30);

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={() => setSectionExpanded((v) => !v)}
          className="flex items-center gap-2 hover:text-foreground text-muted-foreground transition-colors group"
        >
          {sectionExpanded ? (
            <ChevronDown className="h-4 w-4" />
          ) : (
            <ChevronRight className="h-4 w-4" />
          )}
          <History className="h-4 w-4" />
          <h2 className="text-sm font-semibold text-foreground">Event History</h2>
          {sectionExpanded && events !== null && events.length > 0 && (
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
              {events.length}
            </Badge>
          )}
          {sectionExpanded && problemEvents.length > 0 && (
            <Badge variant="destructive" className="text-[10px] px-1.5 py-0">
              {problemEvents.length} unprocessed
            </Badge>
          )}
        </button>
      </div>

      {sectionExpanded && (
        <>
          <p className="text-xs text-muted-foreground">
            All webhook events across this workspace, in reverse chronological order. Filtered, deduped, and error events are highlighted.
          </p>

          {loading ? (
            <div className="space-y-1.5">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-10 rounded-md border bg-muted/30 animate-pulse" />
              ))}
            </div>
          ) : events !== null && events.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center text-sm text-muted-foreground">
                No webhook events recorded yet.
              </CardContent>
            </Card>
          ) : events !== null ? (
            <div className="space-y-1.5">
              {displayEvents.map((evt) => (
                <WebhookEventRow
                  key={evt.id}
                  event={evt}
                  webhookName={webhookNameMap.get(evt.webhook_id)}
                />
              ))}
              {events.length > 30 && (
                <button
                  type="button"
                  onClick={() => setShowAll((v) => !v)}
                  className="text-xs text-primary hover:underline"
                >
                  {showAll ? "Show less" : `Show all ${events.length} events`}
                </button>
              )}
            </div>
          ) : null}
        </>
      )}
    </section>
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

function WebhookMemberPicker({ members, selectedIds, onChange }: {
  members: MemberWithUser[];
  selectedIds: string[];
  onChange: (ids: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");

  const toggle = (userId: string) => {
    onChange(
      selectedIds.includes(userId)
        ? selectedIds.filter((id) => id !== userId)
        : [...selectedIds, userId],
    );
  };

  const humanMembers = members.filter((m) => m.kind === "human");
  const filtered = humanMembers.filter((m) => m.name.toLowerCase().includes(filter.toLowerCase()));
  const selectedMembers = selectedIds.map((id) => humanMembers.find((m) => m.user_id === id)).filter((m): m is MemberWithUser => !!m);

  return (
    <Popover open={open} onOpenChange={(v) => { setOpen(v); if (!v) setFilter(""); }}>
      <PopoverTrigger
        render={
          <button
            type="button"
            className="flex flex-wrap items-center gap-1 rounded-md border bg-transparent px-3 py-2 text-sm min-h-9 w-full text-left hover:bg-accent/50 transition-colors"
          >
            {selectedMembers.length > 0 ? (
              selectedMembers.map((m) => (
                <span key={m.user_id} className="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs">
                  {m.name}
                </span>
              ))
            ) : (
              <span className="text-muted-foreground">Select members...</span>
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
            placeholder="Filter members..."
            className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none"
          />
        </div>
        <div className="p-1 max-h-60 overflow-y-auto">
          {filtered.map((m) => (
            <button
              key={m.user_id}
              type="button"
              onClick={() => toggle(m.user_id)}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
            >
              <span className="flex-1 text-left truncate">{m.name}</span>
              {selectedIds.includes(m.user_id) && (
                <Check className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              )}
            </button>
          ))}
          {humanMembers.length === 0 && (
            <div className="px-2 py-3 text-center text-sm text-muted-foreground">No members</div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
