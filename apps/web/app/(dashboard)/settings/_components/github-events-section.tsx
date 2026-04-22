"use client";

import { useEffect, useState, useCallback } from "react";
import { GitBranch, ChevronDown, ChevronRight, Plus } from "lucide-react";
import type { GitHubEventRule, GitHubEventType, Agent, Daemon, WorkspaceRepo } from "@/shared/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { api } from "@/shared/api";
import { useWorkspaceStore } from "@/features/workspace";

interface EventTypeConfig {
  type: GitHubEventType;
  label: string;
  description: string;
  defaultTitle: string;
}

const EVENT_TYPES: EventTypeConfig[] = [
  {
    type: "push",
    label: "Push",
    description: "Triggered when code is pushed to a branch",
    defaultTitle: "Push to {{.repo}}/{{.branch}} by {{.pusher}}",
  },
  {
    type: "pull_request",
    label: "Pull Request",
    description: "Triggered when a PR is opened, updated, reopened, or closed/merged",
    defaultTitle: "PR #{{.number}}: {{.title}} ({{.action}})",
  },
  {
    type: "issues",
    label: "Issues",
    description: "Triggered when a GitHub issue is opened or reopened",
    defaultTitle: "GitHub Issue #{{.number}}: {{.title}} ({{.action}})",
  },
  {
    type: "issue_comment",
    label: "Issue / PR Comment",
    description: "Triggered when a new comment is created on an issue or PR",
    defaultTitle: "Comment on {{.repo}}#{{.number}}",
  },
];

const REPO_PATTERN = /^[A-Za-z0-9._-]+\/[A-Za-z0-9._-]+$/;

function extractRepoFullName(url: string): string | null {
  if (!url) return null;
  try {
    const u = new URL(url);
    const parts = u.pathname.replace(/^\//, "").replace(/\.git$/, "").split("/");
    if (parts.length >= 2 && parts[0] && parts[1]) {
      return `${parts[0]}/${parts[1]}`;
    }
  } catch {
    // not a valid URL
  }
  return null;
}

export function GitHubEventsSection() {
  const workspace = useWorkspaceStore((s) => s.workspace);
  const agents = useWorkspaceStore((s) => s.agents);
  const [rules, setRules] = useState<GitHubEventRule[]>([]);
  const [daemons, setDaemons] = useState<Daemon[]>([]);
  const [loading, setLoading] = useState(true);

  const isConnected = workspace?.github_connected === true;

  const loadRules = useCallback(async () => {
    if (!workspace?.id) return;
    try {
      const [ruleList, daemonList] = await Promise.all([
        api.listGitHubEventRules(workspace.id),
        api.listDaemons(),
      ]);
      setRules(ruleList);
      setDaemons(daemonList);
    } catch {
      // Silently fail — section will show empty state
    } finally {
      setLoading(false);
    }
  }, [workspace?.id]);

  useEffect(() => {
    if (isConnected) loadRules();
    else setLoading(false);
  }, [isConnected, loadRules]);

  if (!isConnected || !workspace) return null;

  const rulesByType = (type: GitHubEventType) => rules.filter((r) => r.event_type === type);

  return (
    <section className="space-y-4">
      <div className="flex items-center gap-2">
        <GitBranch className="h-4 w-4 text-muted-foreground" />
        <h2 className="text-sm font-semibold">GitHub Events</h2>
        <Badge variant="outline" className="text-xs">Connected</Badge>
      </div>

      <p className="text-xs text-muted-foreground">
        Configure which GitHub events automatically create issues for your agents. Each event type
        has a workspace-wide default plus optional per-repository overrides; the most-specific
        matching rule wins.
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
                <Skeleton className="h-5 w-9 rounded-full" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : (
        <div className="space-y-2">
          {EVENT_TYPES.map((et) => (
            <EventTypeCard
              key={et.type}
              config={et}
              rules={rulesByType(et.type)}
              agents={agents}
              daemons={daemons}
              repos={workspace.repos ?? []}
              workspaceId={workspace.id}
              onUpdated={loadRules}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function EventTypeCard({
  config,
  rules,
  agents,
  daemons,
  repos,
  workspaceId,
  onUpdated,
}: {
  config: EventTypeConfig;
  rules: GitHubEventRule[];
  agents: Agent[];
  daemons: Daemon[];
  repos: WorkspaceRepo[];
  workspaceId: string;
  onUpdated: () => Promise<void>;
}) {
  const [expanded, setExpanded] = useState(false);
  // Drafts for repo-override rules that exist locally but aren't yet saved.
  // Keyed by a stable client-side id.
  const [drafts, setDrafts] = useState<{ id: string }[]>([]);
  const [saving, setSaving] = useState(false);

  const defaultRule = rules.find((r) => r.repo_full_name === "") ?? null;
  const overrideRules = rules.filter((r) => r.repo_full_name !== "");
  const activeAgents = agents.filter((a) => !a.archived_at);

  const enabledCount = rules.filter((r) => r.enabled).length;
  const summaryLabel = (() => {
    if (defaultRule?.enabled && overrideRules.length === 0) {
      return agentName(agents, defaultRule.agent_id);
    }
    if (enabledCount === 0) return null;
    return `${enabledCount} rule${enabledCount === 1 ? "" : "s"}`;
  })();

  const handleToggleDefault = async (checked: boolean) => {
    if (!defaultRule && !checked) return;

    if (!defaultRule && checked) {
      const firstAgent = activeAgents[0];
      if (!firstAgent) {
        toast.error("No agents available. Create an agent first.");
        return;
      }
      setSaving(true);
      try {
        await api.upsertGitHubEventRule(workspaceId, {
          event_type: config.type,
          repo_full_name: "",
          agent_id: firstAgent.id,
          enabled: true,
        });
        await onUpdated();
        setExpanded(true);
        toast.success(`${config.label} default enabled`);
      } catch (e) {
        toast.error(e instanceof Error ? e.message : "Failed to enable");
      } finally {
        setSaving(false);
      }
      return;
    }

    setSaving(true);
    try {
      await api.upsertGitHubEventRule(workspaceId, {
        event_type: config.type,
        repo_full_name: "",
        agent_id: defaultRule!.agent_id,
        enabled: checked,
      });
      await onUpdated();
      toast.success(checked ? `${config.label} default enabled` : `${config.label} default disabled`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update");
    } finally {
      setSaving(false);
    }
  };

  const addDraft = () => {
    setDrafts((d) => [...d, { id: `draft-${Date.now()}-${Math.random().toString(36).slice(2, 6)}` }]);
    setExpanded(true);
  };

  const removeDraft = (id: string) => {
    setDrafts((d) => d.filter((x) => x.id !== id));
  };

  return (
    <Card>
      <CardContent className="p-0">
        <div className="flex items-center gap-3 px-4 py-3">
          <button
            className="text-muted-foreground hover:text-foreground transition-colors shrink-0"
            onClick={() => setExpanded(!expanded)}
          >
            {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
          </button>

          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">{config.label}</span>
              {summaryLabel && (
                <Badge variant="outline" className="text-xs">{summaryLabel}</Badge>
              )}
              {overrideRules.length > 0 && (
                <Badge variant="secondary" className="text-xs">
                  {overrideRules.length} repo override{overrideRules.length === 1 ? "" : "s"}
                </Badge>
              )}
            </div>
            <p className="text-xs text-muted-foreground truncate">{config.description}</p>
          </div>

          <Switch
            checked={defaultRule?.enabled ?? false}
            onCheckedChange={handleToggleDefault}
            disabled={saving}
          />
        </div>

        {expanded && (
          <div className="border-t px-4 py-3 space-y-4">
            <RuleEditor
              key={defaultRule?.id ?? "default-new"}
              variant="default"
              eventConfig={config}
              rule={defaultRule}
              agents={agents}
              daemons={daemons}
              repos={repos}
              existingOverrideRepos={overrideRules.map((r) => r.repo_full_name)}
              workspaceId={workspaceId}
              onUpdated={onUpdated}
            />

            {overrideRules.map((rule) => (
              <RuleEditor
                key={rule.id}
                variant="override"
                eventConfig={config}
                rule={rule}
                agents={agents}
                daemons={daemons}
                repos={repos}
                existingOverrideRepos={overrideRules.map((r) => r.repo_full_name)}
                workspaceId={workspaceId}
                onUpdated={onUpdated}
              />
            ))}

            {drafts.map((draft) => (
              <RuleEditor
                key={draft.id}
                variant="override"
                eventConfig={config}
                rule={null}
                agents={agents}
                daemons={daemons}
                repos={repos}
                existingOverrideRepos={overrideRules.map((r) => r.repo_full_name)}
                workspaceId={workspaceId}
                onUpdated={async () => {
                  removeDraft(draft.id);
                  await onUpdated();
                }}
                onCancel={() => removeDraft(draft.id)}
              />
            ))}

            <div>
              <Button variant="outline" size="sm" className="text-xs" onClick={addDraft}>
                <Plus className="h-3.5 w-3.5 mr-1" />
                Add per-repo override
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function RuleEditor({
  variant,
  eventConfig,
  rule,
  agents,
  daemons,
  repos,
  existingOverrideRepos,
  workspaceId,
  onUpdated,
  onCancel,
}: {
  variant: "default" | "override";
  eventConfig: EventTypeConfig;
  rule: GitHubEventRule | null;
  agents: Agent[];
  daemons: Daemon[];
  repos: WorkspaceRepo[];
  existingOverrideRepos: string[];
  workspaceId: string;
  onUpdated: () => Promise<void>;
  onCancel?: () => void;
}) {
  const [saving, setSaving] = useState(false);
  const [repoFullName, setRepoFullName] = useState(rule?.repo_full_name ?? "");
  const [agentId, setAgentId] = useState(rule?.agent_id ?? "");
  const [enabled, setEnabled] = useState(rule?.enabled ?? true);
  const [titleTemplate, setTitleTemplate] = useState(rule?.title_template ?? "");
  const [descriptionTemplate, setDescriptionTemplate] = useState(rule?.description_template ?? "");
  const [dispatchDaemonId, setDispatchDaemonId] = useState(rule?.dispatch_daemon_id ?? "");
  const [dispatchProvider, setDispatchProvider] = useState(rule?.dispatch_provider ?? "");

  useEffect(() => {
    setRepoFullName(rule?.repo_full_name ?? "");
    setAgentId(rule?.agent_id ?? "");
    setEnabled(rule?.enabled ?? true);
    setTitleTemplate(rule?.title_template ?? "");
    setDescriptionTemplate(rule?.description_template ?? "");
    setDispatchDaemonId(rule?.dispatch_daemon_id ?? "");
    setDispatchProvider(rule?.dispatch_provider ?? "");
  }, [rule]);

  const activeAgents = agents.filter((a) => !a.archived_at);
  const activeDaemons = daemons.filter((d) => !d.archived_at);
  const isOverride = variant === "override";
  const isExisting = rule !== null;

  const repoOptions = (() => {
    const seen = new Set<string>();
    const options: string[] = [];
    for (const r of repos) {
      const full = extractRepoFullName(r.url);
      if (full && !seen.has(full)) {
        seen.add(full);
        options.push(full);
      }
    }
    return options;
  })();
  const takenRepos = new Set(existingOverrideRepos);
  const availableRepoOptions = repoOptions.filter(
    (r) => !takenRepos.has(r) || r === rule?.repo_full_name,
  );

  const handleSave = async () => {
    if (!agentId) {
      toast.error("Please select an agent");
      return;
    }
    if (isOverride) {
      const trimmed = repoFullName.trim();
      if (!trimmed) {
        toast.error("Repository is required for per-repo overrides");
        return;
      }
      if (!REPO_PATTERN.test(trimmed)) {
        toast.error("Repository must be in 'owner/repo' format");
        return;
      }
    }

    setSaving(true);
    try {
      await api.upsertGitHubEventRule(workspaceId, {
        event_type: eventConfig.type,
        repo_full_name: isOverride ? repoFullName.trim() : "",
        agent_id: agentId,
        enabled,
        title_template: titleTemplate,
        description_template: descriptionTemplate,
        dispatch_provider: dispatchProvider || undefined,
        dispatch_daemon_id: dispatchDaemonId || undefined,
      });
      await onUpdated();
      toast.success(
        isOverride
          ? `Override for ${repoFullName.trim()} saved`
          : `${eventConfig.label} default saved`,
      );
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!rule) {
      onCancel?.();
      return;
    }
    setSaving(true);
    try {
      await api.deleteGitHubEventRule(workspaceId, rule.id);
      await onUpdated();
      toast.success(
        isOverride
          ? `Override for ${rule.repo_full_name} removed`
          : `${eventConfig.label} default removed`,
      );
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete");
    } finally {
      setSaving(false);
    }
  };

  const daemonName = (id: string) => {
    const d = daemons.find((dm) => dm.id === id);
    return d?.device_name || d?.daemon_id || "Unknown";
  };

  return (
    <div className="rounded-md border bg-muted/30 p-3 space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Badge variant={isOverride ? "secondary" : "outline"} className="text-xs">
            {isOverride ? (rule?.repo_full_name || "New override") : "Workspace default"}
          </Badge>
          {!isOverride && !isExisting && (
            <span className="text-xs text-muted-foreground">Not configured yet</span>
          )}
        </div>
        {isOverride && (
          <div className="flex items-center gap-2">
            <Label className="text-xs text-muted-foreground">Enabled</Label>
            <Switch checked={enabled} onCheckedChange={setEnabled} disabled={saving} />
          </div>
        )}
      </div>

      {isOverride && (
        <div className="space-y-1.5">
          <Label className="text-xs">Repository</Label>
          {isExisting ? (
            <>
              <Input
                className="h-8 text-xs font-mono"
                value={repoFullName}
                disabled
              />
              <p className="text-[11px] text-muted-foreground">
                Repository name cannot be changed. Remove this override and add a new one to retarget.
              </p>
            </>
          ) : repoOptions.length === 0 ? (
            <>
              <Input
                className="h-8 text-xs font-mono"
                placeholder="owner/repo"
                value={repoFullName}
                onChange={(e) => setRepoFullName(e.target.value)}
                disabled={saving}
              />
              <p className="text-[11px] text-muted-foreground">
                No repositories configured. Add them in the Repositories tab to pick from a list.
              </p>
            </>
          ) : (
            <>
              <Select value={repoFullName} onValueChange={(v) => { if (v) setRepoFullName(v); }}>
                <SelectTrigger className="h-8 text-xs font-mono w-full">
                  <SelectValue placeholder="Select repository...">
                    {repoFullName || "Select repository..."}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {availableRepoOptions.map((r) => (
                    <SelectItem key={r} value={r}>
                      <span className="font-mono">{r}</span>
                    </SelectItem>
                  ))}
                  {availableRepoOptions.length === 0 && (
                    <div className="px-2 py-1.5 text-xs text-muted-foreground">
                      All configured repos already have an override.
                    </div>
                  )}
                </SelectContent>
              </Select>
            </>
          )}
        </div>
      )}

      <div className="space-y-1.5">
        <Label className="text-xs">Agent</Label>
        <Select value={agentId} onValueChange={(v) => { if (v) setAgentId(v); }}>
          <SelectTrigger className="h-8 text-xs w-full">
            <SelectValue placeholder="Select agent...">
              {agentId ? agentName(agents, agentId) : "Select agent..."}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {activeAgents.map((a) => (
              <SelectItem key={a.id} value={a.id}>
                {a.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-1.5">
        <Label className="text-xs">Environment</Label>
        <Select
          value={dispatchDaemonId || "__auto__"}
          onValueChange={(v) => setDispatchDaemonId(v === "__auto__" ? "" : v ?? "")}
        >
          <SelectTrigger className="h-8 text-xs w-full">
            <SelectValue placeholder="Auto (agent default)">
              {dispatchDaemonId ? daemonName(dispatchDaemonId) : "Auto (agent default)"}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__auto__">Auto (agent default)</SelectItem>
            {activeDaemons.map((d) => (
              <SelectItem key={d.id} value={d.id}>
                {daemonName(d.id)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-1.5">
        <Label className="text-xs">
          Issue Title Template
          <span className="text-muted-foreground ml-1">(leave empty for default)</span>
        </Label>
        <Input
          className="h-8 text-xs"
          placeholder={eventConfig.defaultTitle}
          value={titleTemplate}
          onChange={(e) => setTitleTemplate(e.target.value)}
        />
      </div>

      <div className="space-y-1.5">
        <Label className="text-xs">
          Issue Description Template
          <span className="text-muted-foreground ml-1">(leave empty for auto-generated)</span>
        </Label>
        <Textarea
          className="text-xs min-h-[60px]"
          placeholder="{{.body}}"
          value={descriptionTemplate}
          onChange={(e) => setDescriptionTemplate(e.target.value)}
        />
      </div>

      <div className="flex items-center justify-between pt-1">
        <div>
          {(isExisting || onCancel) && (
            <Button
              variant="ghost"
              size="sm"
              className="text-destructive text-xs"
              onClick={handleDelete}
              disabled={saving}
            >
              {isExisting ? "Remove" : "Cancel"}
            </Button>
          )}
        </div>
        <Button size="sm" className="text-xs" onClick={handleSave} disabled={saving || !agentId}>
          {saving ? "Saving..." : "Save"}
        </Button>
      </div>
    </div>
  );
}

function agentName(agents: Agent[], id: string) {
  return agents.find((a) => a.id === id)?.name ?? "Unknown";
}
