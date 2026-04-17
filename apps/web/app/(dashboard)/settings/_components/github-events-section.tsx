"use client";

import { useEffect, useState, useCallback } from "react";
import { GitBranch, ChevronDown, ChevronRight } from "lucide-react";
import type { GitHubEventRule, GitHubEventType, Agent, Daemon } from "@/shared/types";
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

  const ruleByType = (type: GitHubEventType) =>
    rules.find((r) => r.event_type === type) ?? null;

  return (
    <section className="space-y-4">
      <div className="flex items-center gap-2">
        <GitBranch className="h-4 w-4 text-muted-foreground" />
        <h2 className="text-sm font-semibold">GitHub Events</h2>
        <Badge variant="outline" className="text-xs">Connected</Badge>
      </div>

      <p className="text-xs text-muted-foreground">
        Configure which GitHub events automatically create issues for your agents. Events are received through the GitHub App installed on your organization.
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
              rule={ruleByType(et.type)}
              agents={agents}
              daemons={daemons}
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
  rule,
  agents,
  daemons,
  workspaceId,
  onUpdated,
}: {
  config: EventTypeConfig;
  rule: GitHubEventRule | null;
  agents: Agent[];
  daemons: Daemon[];
  workspaceId: string;
  onUpdated: () => Promise<void>;
}) {
  const [expanded, setExpanded] = useState(false);
  const [saving, setSaving] = useState(false);

  const [agentId, setAgentId] = useState(rule?.agent_id ?? "");
  const [enabled, setEnabled] = useState(rule?.enabled ?? false);
  const [titleTemplate, setTitleTemplate] = useState(rule?.title_template ?? "");
  const [descriptionTemplate, setDescriptionTemplate] = useState(rule?.description_template ?? "");
  const [dispatchProvider, setDispatchProvider] = useState(rule?.dispatch_provider ?? "");
  const [dispatchDaemonId, setDispatchDaemonId] = useState(rule?.dispatch_daemon_id ?? "");

  useEffect(() => {
    setAgentId(rule?.agent_id ?? "");
    setEnabled(rule?.enabled ?? false);
    setTitleTemplate(rule?.title_template ?? "");
    setDescriptionTemplate(rule?.description_template ?? "");
    setDispatchProvider(rule?.dispatch_provider ?? "");
    setDispatchDaemonId(rule?.dispatch_daemon_id ?? "");
  }, [rule]);

  const activeAgents = agents.filter((a) => !a.archived_at);

  const handleToggle = async (checked: boolean) => {
    if (!rule && !checked) return;

    if (!rule && checked) {
      const firstAgent = activeAgents[0];
      if (!firstAgent) {
        toast.error("No agents available. Create an agent first.");
        return;
      }
      setEnabled(true);
      setAgentId(firstAgent.id);
      setExpanded(true);
      return;
    }

    setSaving(true);
    try {
      await api.upsertGitHubEventRule(workspaceId, {
        event_type: config.type,
        agent_id: rule!.agent_id,
        enabled: checked,
      });
      await onUpdated();
      toast.success(checked ? `${config.label} events enabled` : `${config.label} events disabled`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update");
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    if (!agentId) {
      toast.error("Please select an agent");
      return;
    }
    setSaving(true);
    try {
      await api.upsertGitHubEventRule(workspaceId, {
        event_type: config.type,
        agent_id: agentId,
        enabled,
        title_template: titleTemplate,
        description_template: descriptionTemplate,
        dispatch_provider: dispatchProvider || undefined,
        dispatch_daemon_id: dispatchDaemonId || undefined,
      });
      await onUpdated();
      toast.success(`${config.label} rule saved`);
      setExpanded(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!rule) return;
    setSaving(true);
    try {
      await api.deleteGitHubEventRule(workspaceId, rule.id);
      await onUpdated();
      setExpanded(false);
      setEnabled(false);
      setAgentId("");
      toast.success(`${config.label} rule removed`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete");
    } finally {
      setSaving(false);
    }
  };

  const agentName = (id: string) => agents.find((a) => a.id === id)?.name ?? "Unknown";
  const daemonName = (id: string) => {
    const d = daemons.find((dm) => dm.id === id);
    return d?.device_name || d?.daemon_id || "Unknown";
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
              {rule?.enabled && (
                <Badge variant="outline" className="text-xs">
                  {agentName(rule.agent_id)}
                </Badge>
              )}
            </div>
            <p className="text-xs text-muted-foreground truncate">{config.description}</p>
          </div>

          <Switch
            checked={rule ? enabled : enabled}
            onCheckedChange={handleToggle}
            disabled={saving}
          />
        </div>

        {expanded && (
          <div className="border-t px-4 py-3 space-y-3">
            <div className="space-y-1.5">
              <Label className="text-xs">Agent</Label>
              <Select value={agentId} onValueChange={(v) => { if (v) setAgentId(v); }}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="Select agent..." />
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
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="Auto (agent default)" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__auto__">Auto (agent default)</SelectItem>
                  {daemons.filter((d) => !d.archived_at).map((d) => (
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
                placeholder={config.defaultTitle}
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
                {rule && (
                  <Button variant="ghost" size="sm" className="text-destructive text-xs" onClick={handleDelete} disabled={saving}>
                    Remove
                  </Button>
                )}
              </div>
              <Button size="sm" className="text-xs" onClick={handleSave} disabled={saving || !agentId}>
                {saving ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
