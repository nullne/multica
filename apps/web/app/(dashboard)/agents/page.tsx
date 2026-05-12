"use client";

import { useState, useEffect, useRef } from "react";
import { useDefaultLayout } from "react-resizable-panels";
import {
  Bot,
  Plus,
  ListTodo,
  Wrench,
  FileText,
  BookOpenText,
  MessageSquare,
  Timer,
  Trash2,
  Save,
  Key,
  Link2,
  Clock,
  CheckCircle2,
  XCircle,
  Loader2,
  AlertCircle,
  MoreHorizontal,
  Play,
  ChevronDown,
  ChevronLeft,
  Globe,
  Lock,
  Settings,
  Camera,
  Monitor,
  X,
} from "lucide-react";
import type {
  Agent,
  AgentStatus,
  AgentVisibility,
  AgentTool,
  AgentTrigger,
  AgentTriggerType,
  AgentTask,
  CreateAgentRequest,
  UpdateAgentRequest,
} from "@/shared/types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@/components/ui/resizable";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/shared/api";
import { useIsMobile } from "@/hooks/use-mobile";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { useIssueStore } from "@/features/issues";
import { useRuntimeStore } from "@/features/runtimes";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useFileUpload } from "@/shared/hooks/use-file-upload";
import { cn } from "@/lib/utils";


// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const statusConfig: Record<AgentStatus, { label: string; color: string; dot: string }> = {
  idle: { label: "Idle", color: "text-muted-foreground", dot: "bg-muted-foreground" },
  working: { label: "Working", color: "text-success", dot: "bg-success" },
  blocked: { label: "Blocked", color: "text-warning", dot: "bg-warning" },
  error: { label: "Error", color: "text-destructive", dot: "bg-destructive" },
  offline: { label: "Offline", color: "text-muted-foreground/50", dot: "bg-muted-foreground/40" },
};

const taskStatusConfig: Record<string, { label: string; icon: typeof CheckCircle2; color: string }> = {
  queued: { label: "Queued", icon: Clock, color: "text-muted-foreground" },
  dispatched: { label: "Dispatched", icon: Play, color: "text-info" },
  running: { label: "Running", icon: Loader2, color: "text-success" },
  completed: { label: "Completed", icon: CheckCircle2, color: "text-success" },
  failed: { label: "Failed", icon: XCircle, color: "text-destructive" },
  cancelled: { label: "Cancelled", icon: XCircle, color: "text-muted-foreground" },
};


function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}


// ---------------------------------------------------------------------------
// Create Agent Dialog
// ---------------------------------------------------------------------------

const SUPPORTED_PROVIDERS = [
  { key: "claude", label: "Claude Code" },
  { key: "codex", label: "Codex" },
  { key: "opencode", label: "OpenCode" },
  { key: "cursor", label: "Cursor" },
] as const;

function CreateAgentDialog({
  onClose,
  onCreate,
}: {
  onClose: () => void;
  onCreate: (data: CreateAgentRequest) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedProviders, setSelectedProviders] = useState<string[]>([SUPPORTED_PROVIDERS[0].key]);
  const [visibility, setVisibility] = useState<AgentVisibility>("private");
  const [creating, setCreating] = useState(false);

  const handleSubmit = async () => {
    if (!name.trim() || selectedProviders.length === 0) return;
    setCreating(true);
    try {
      await onCreate({
        name: name.trim(),
        description: description.trim(),
        providers: selectedProviders,
        visibility,
        triggers: [
          { id: generateId(), type: "on_assign", enabled: true, config: {} },
          { id: generateId(), type: "on_comment", enabled: true, config: {} },
        ],
      });
      onClose();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create agent");
      setCreating(false);
    }
  };

  return (
    <Dialog open onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create Agent</DialogTitle>
          <DialogDescription>
            Create a new AI agent for your workspace.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div>
            <Label className="text-xs text-muted-foreground">Name</Label>
            <Input
              autoFocus
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Deep Research Agent"
              className="mt-1"
              onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
            />
          </div>

          <div>
            <Label className="text-xs text-muted-foreground">Description</Label>
            <Input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What does this agent do?"
              className="mt-1"
            />
          </div>

          <div>
            <Label className="text-xs text-muted-foreground">Provider</Label>
            <div className="mt-1.5 flex gap-2 flex-wrap">
              {SUPPORTED_PROVIDERS.map((p) => {
                const selected = selectedProviders.includes(p.key);
                return (
                  <button
                    key={p.key}
                    type="button"
                    onClick={() =>
                      setSelectedProviders((prev) =>
                        selected
                          ? prev.filter((k) => k !== p.key)
                          : [...prev, p.key],
                      )
                    }
                    className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors ${
                      selected
                        ? "border-primary bg-primary/5"
                        : "border-border hover:bg-muted"
                    }`}
                  >
                    <span className="font-medium">{p.label}</span>
                  </button>
                );
              })}
            </div>
            <p className="mt-1.5 text-xs text-muted-foreground">
              Select one or more providers. Tasks will be dispatched to any online environment.
            </p>
          </div>

          <div>
            <Label className="text-xs text-muted-foreground">Visibility</Label>
            <div className="mt-1.5 flex gap-2">
              <button
                type="button"
                onClick={() => setVisibility("workspace")}
                className={`flex flex-1 items-center gap-2 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
                  visibility === "workspace"
                    ? "border-primary bg-primary/5"
                    : "border-border hover:bg-muted"
                }`}
              >
                <Globe className="h-4 w-4 shrink-0 text-muted-foreground" />
                <div className="text-left">
                  <div className="font-medium">Workspace</div>
                  <div className="text-xs text-muted-foreground">All members can assign</div>
                </div>
              </button>
              <button
                type="button"
                onClick={() => setVisibility("private")}
                className={`flex flex-1 items-center gap-2 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
                  visibility === "private"
                    ? "border-primary bg-primary/5"
                    : "border-border hover:bg-muted"
                }`}
              >
                <Lock className="h-4 w-4 shrink-0 text-muted-foreground" />
                <div className="text-left">
                  <div className="font-medium">Private</div>
                  <div className="text-xs text-muted-foreground">Only you can assign</div>
                </div>
              </button>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={creating || !name.trim() || selectedProviders.length === 0}
          >
            {creating ? "Creating..." : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Agent List Item
// ---------------------------------------------------------------------------

function AgentListItem({
  agent,
  isSelected,
  onClick,
}: {
  agent: Agent;
  isSelected: boolean;
  onClick: () => void;
}) {
  const st = statusConfig[agent.status];
  const isArchived = !!agent.archived_at;
  const daemons = useRuntimeStore((s) => s.daemons);
  const VisibilityIcon = agent.visibility === "workspace" ? Globe : Lock;
  const codeAccess = agent.github_code_access ?? "read";
  const defaultDaemon = agent.default_daemon_id
    ? daemons.find((d) => d.id === agent.default_daemon_id)
    : null;
  const environmentLabel = defaultDaemon
    ? defaultDaemon.device_name || defaultDaemon.daemon_id || "Default environment"
    : "Any environment";
  const providersLabel =
    agent.providers?.length > 0 ? agent.providers.join(", ") : "No providers";
  const maxTasks = agent.max_concurrent_tasks ?? 6;

  return (
    <button
      onClick={onClick}
      className={`flex w-full items-start gap-3 px-4 py-3 text-left transition-colors ${
        isSelected ? "bg-accent" : "hover:bg-accent/50"
      }`}
    >
      <ActorAvatar
        actorType="agent"
        actorId={agent.id}
        size={32}
        className={`mt-0.5 shrink-0 rounded-lg ${isArchived ? "opacity-50 grayscale" : ""}`}
      />

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className={`truncate text-sm font-medium ${isArchived ? "text-muted-foreground" : ""}`}>
            {agent.name}
          </span>
        </div>

        <div className="mt-0.5 flex min-w-0 items-center gap-1.5">
          {isArchived ? (
            <span className="shrink-0 text-xs text-muted-foreground">Archived</span>
          ) : (
            <>
              <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${st.dot}`} />
              <span className={`shrink-0 text-xs ${st.color}`}>{st.label}</span>
            </>
          )}
          <span className="shrink-0 text-xs text-muted-foreground/40">&middot;</span>
          <span
            className="truncate text-xs text-muted-foreground"
            title={providersLabel}
          >
            {providersLabel}
          </span>
        </div>

        <div className="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-xs text-muted-foreground">
          <span
            className="flex items-center gap-1"
            title={`Visibility: ${agent.visibility}`}
          >
            <VisibilityIcon className="h-3 w-3" />
            <span className="capitalize">{agent.visibility}</span>
          </span>
          <span className="text-muted-foreground/40">&middot;</span>
          <span
            className="capitalize"
            title={`GitHub code access: ${codeAccess}`}
          >
            {codeAccess}
          </span>
          <span className="text-muted-foreground/40">&middot;</span>
          <span
            className="flex min-w-0 max-w-full items-center gap-1"
            title={defaultDaemon ? `Environment: ${environmentLabel}` : "No default environment"}
          >
            <Monitor className="h-3 w-3 shrink-0" />
            <span className={`truncate ${defaultDaemon ? "" : "italic"}`}>
              {environmentLabel}
            </span>
          </span>
          <span className="text-muted-foreground/40">&middot;</span>
          <span title={`Max concurrent tasks: ${maxTasks}`}>
            {maxTasks} task{maxTasks === 1 ? "" : "s"}
          </span>
        </div>
      </div>
    </button>
  );
}


// ---------------------------------------------------------------------------
// Instructions Tab
// ---------------------------------------------------------------------------

function InstructionsTab({
  agent,
  onSave,
}: {
  agent: Agent;
  onSave: (instructions: string) => Promise<void>;
}) {
  const [value, setValue] = useState(agent.instructions ?? "");
  const [saving, setSaving] = useState(false);
  const isDirty = value !== (agent.instructions ?? "");

  // Sync when switching between agents.
  useEffect(() => {
    setValue(agent.instructions ?? "");
  }, [agent.id, agent.instructions]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(value);
    } catch {
      // toast handled by parent
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold">Agent Instructions</h3>
        <p className="text-xs text-muted-foreground mt-0.5">
          Define this agent&apos;s identity and working style. These instructions are
          injected into the agent&apos;s context for every task.
        </p>
      </div>

      <textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder={`Define this agent's role, expertise, and working style.\n\nExample:\nYou are a frontend engineer specializing in React and TypeScript.\n\n## Working Style\n- Write small, focused PRs — one commit per logical change\n- Prefer composition over inheritance\n- Always add unit tests for new components\n\n## Constraints\n- Do not modify shared/ types without explicit approval\n- Follow the existing component patterns in features/`}
        className="w-full min-h-[300px] rounded-md border bg-transparent px-3 py-2 text-sm font-mono placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring resize-y"
      />

      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {value.length > 0 ? `${value.length} characters` : "No instructions set"}
        </span>
        <Button
          size="xs"
          onClick={handleSave}
          disabled={!isDirty || saving}
        >
          {saving ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : (
            <Save className="h-3 w-3" />
          )}
          Save
        </Button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Skills Tab (picker — skills are managed on /skills page)
// ---------------------------------------------------------------------------

function SkillsTab({
  agent,
}: {
  agent: Agent;
}) {
  const workspaceSkills = useWorkspaceStore((s) => s.skills);
  const refreshAgents = useWorkspaceStore((s) => s.refreshAgents);
  const [saving, setSaving] = useState(false);
  const [showPicker, setShowPicker] = useState(false);

  const agentSkillIds = new Set(agent.skills.map((s) => s.id));
  const availableSkills = workspaceSkills.filter((s) => !agentSkillIds.has(s.id));

  const handleAdd = async (skillId: string) => {
    setSaving(true);
    try {
      const newIds = [...agent.skills.map((s) => s.id), skillId];
      await api.setAgentSkills(agent.id, { skill_ids: newIds });
      await refreshAgents();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to add skill");
    } finally {
      setSaving(false);
      setShowPicker(false);
    }
  };

  const handleRemove = async (skillId: string) => {
    setSaving(true);
    try {
      const newIds = agent.skills.filter((s) => s.id !== skillId).map((s) => s.id);
      await api.setAgentSkills(agent.id, { skill_ids: newIds });
      await refreshAgents();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to remove skill");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">Skills</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Reusable skills assigned to this agent. Manage skills on the Skills page.
          </p>
        </div>
        <Button
          variant="outline"
          size="xs"
          onClick={() => setShowPicker(true)}
          disabled={saving || availableSkills.length === 0}
        >
          <Plus className="h-3 w-3" />
          Add Skill
        </Button>
      </div>

      {agent.skills.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12">
          <FileText className="h-8 w-8 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">No skills assigned</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Add skills from the workspace to this agent.
          </p>
          {availableSkills.length > 0 && (
            <Button
              onClick={() => setShowPicker(true)}
              size="xs"
              className="mt-3"
              disabled={saving}
            >
              <Plus className="h-3 w-3" />
              Add Skill
            </Button>
          )}
        </div>
      ) : (
        <div className="space-y-2">
          {agent.skills.map((skill) => (
            <div
              key={skill.id}
              className="flex items-center gap-3 rounded-lg border px-4 py-3"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                <FileText className="h-4 w-4 text-muted-foreground" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium">{skill.name}</div>
                {skill.description && (
                  <div className="text-xs text-muted-foreground truncate">
                    {skill.description}
                  </div>
                )}
              </div>
              <Button
                variant="ghost"
                size="icon-xs"
                onClick={() => handleRemove(skill.id)}
                disabled={saving}
                className="text-muted-foreground hover:text-destructive"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          ))}
        </div>
      )}

      {/* Skill Picker Dialog */}
      {showPicker && (
        <Dialog open onOpenChange={(v) => { if (!v) setShowPicker(false); }}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle className="text-sm">Add Skill</DialogTitle>
              <DialogDescription className="text-xs">
                Select a skill to assign to this agent.
              </DialogDescription>
            </DialogHeader>
            <div className="max-h-64 overflow-y-auto space-y-1">
              {availableSkills.map((skill) => (
                <button
                  key={skill.id}
                  onClick={() => handleAdd(skill.id)}
                  disabled={saving}
                  className="flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-left text-sm transition-colors hover:bg-accent/50"
                >
                  <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <div className="min-w-0 flex-1">
                    <div className="font-medium">{skill.name}</div>
                    {skill.description && (
                      <div className="text-xs text-muted-foreground truncate">
                        {skill.description}
                      </div>
                    )}
                  </div>
                </button>
              ))}
              {availableSkills.length === 0 && (
                <p className="py-6 text-center text-xs text-muted-foreground">
                  All workspace skills are already assigned.
                </p>
              )}
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setShowPicker(false)}>
                Cancel
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tools Tab
// ---------------------------------------------------------------------------

function AddToolDialog({
  onClose,
  onAdd,
}: {
  onClose: () => void;
  onAdd: (tool: AgentTool) => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [authType, setAuthType] = useState<"oauth" | "api_key" | "none">("api_key");

  const handleAdd = () => {
    if (!name.trim()) return;
    onAdd({
      id: generateId(),
      name: name.trim(),
      description: description.trim(),
      auth_type: authType,
      connected: false,
      config: {},
    });
    onClose();
  };

  return (
    <Dialog open onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-sm">Add Tool</DialogTitle>
          <DialogDescription className="text-xs">
            Connect an external tool for this agent to use.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div>
            <Label className="text-xs text-muted-foreground">Tool Name</Label>
            <Input
              autoFocus
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Google Search, Slack, GitHub"
              className="mt-1"
              onKeyDown={(e) => e.key === "Enter" && handleAdd()}
            />
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">Description</Label>
            <Input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What does this tool do?"
              className="mt-1"
            />
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">Authentication</Label>
            <div className="mt-1.5 flex gap-2">
              {(["api_key", "oauth", "none"] as const).map((type) => (
                <Button
                  key={type}
                  variant={authType === type ? "outline" : "ghost"}
                  size="xs"
                  onClick={() => setAuthType(type)}
                  className={`flex-1 ${
                    authType === type
                      ? "border-primary bg-primary/5 font-medium"
                      : ""
                  }`}
                >
                  {type === "api_key" ? "API Key" : type === "oauth" ? "OAuth" : "None"}
                </Button>
              ))}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={handleAdd}
            disabled={!name.trim()}
          >
            Add
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ToolsTab({
  agent,
  onSave,
}: {
  agent: Agent;
  onSave: (tools: AgentTool[]) => Promise<void>;
}) {
  const [tools, setTools] = useState<AgentTool[]>(agent.tools ?? []);
  const [showAdd, setShowAdd] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setTools(agent.tools ?? []);
  }, [agent.id, agent.tools]);

  const isDirty = JSON.stringify(tools) !== JSON.stringify(agent.tools ?? []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(tools);
    } catch {
      // toast handled by parent
    } finally {
      setSaving(false);
    }
  };

  const toggleConnect = (toolId: string) => {
    setTools((prev) =>
      prev.map((t) => (t.id === toolId ? { ...t, connected: !t.connected } : t)),
    );
  };

  const removeTool = (toolId: string) => {
    setTools((prev) => prev.filter((t) => t.id !== toolId));
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">Tools</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            External tools and APIs this agent can use during task execution.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {isDirty && (
            <Button
              onClick={handleSave}
              disabled={saving}
              size="xs"
            >
              <Save className="h-3 w-3" />
              {saving ? "Saving..." : "Save"}
            </Button>
          )}
          <Button
            variant="outline"
            size="xs"
            onClick={() => setShowAdd(true)}
          >
            <Plus className="h-3 w-3" />
            Add Tool
          </Button>
        </div>
      </div>

      {tools.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12">
          <Wrench className="h-8 w-8 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">No tools configured</p>
          <Button
            onClick={() => setShowAdd(true)}
            size="xs"
            className="mt-3"
          >
            <Plus className="h-3 w-3" />
            Add Tool
          </Button>
        </div>
      ) : (
        <div className="space-y-2">
          {tools.map((tool) => (
            <div
              key={tool.id}
              className="flex items-center gap-3 rounded-lg border px-4 py-3"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                {tool.auth_type === "oauth" ? (
                  <Link2 className="h-4 w-4 text-muted-foreground" />
                ) : tool.auth_type === "api_key" ? (
                  <Key className="h-4 w-4 text-muted-foreground" />
                ) : (
                  <Wrench className="h-4 w-4 text-muted-foreground" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium">{tool.name}</div>
                {tool.description && (
                  <div className="text-xs text-muted-foreground truncate">
                    {tool.description}
                  </div>
                )}
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => toggleConnect(tool.id)}
                  className={
                    tool.connected
                      ? "bg-success/10 text-success"
                      : "bg-muted text-muted-foreground hover:bg-accent"
                  }
                >
                  {tool.connected ? "Connected" : "Connect"}
                </Button>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  onClick={() => removeTool(tool.id)}
                  className="text-muted-foreground hover:text-destructive"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      {showAdd && (
        <AddToolDialog
          onClose={() => setShowAdd(false)}
          onAdd={(tool) => setTools((prev) => [...prev, tool])}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Triggers Tab
// ---------------------------------------------------------------------------

function TriggersTab({
  agent,
  onSave,
}: {
  agent: Agent;
  onSave: (triggers: AgentTrigger[]) => Promise<void>;
}) {
  const [triggers, setTriggers] = useState<AgentTrigger[]>(agent.triggers ?? []);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setTriggers(agent.triggers ?? []);
  }, [agent.id, agent.triggers]);

  const isDirty = JSON.stringify(triggers) !== JSON.stringify(agent.triggers ?? []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(triggers);
    } catch {
      // toast handled by parent
    } finally {
      setSaving(false);
    }
  };

  const toggleTrigger = (triggerId: string) => {
    setTriggers((prev) =>
      prev.map((t) => (t.id === triggerId ? { ...t, enabled: !t.enabled } : t)),
    );
  };

  const removeTrigger = (triggerId: string) => {
    setTriggers((prev) => prev.filter((t) => t.id !== triggerId));
  };

  const addTrigger = (type: AgentTriggerType) => {
    const newTrigger: AgentTrigger = {
      id: generateId(),
      type,
      enabled: true,
      config: type === "scheduled" ? { cron: "0 9 * * 1-5", timezone: "UTC" } : {},
    };
    setTriggers((prev) => [...prev, newTrigger]);
  };

  const updateTriggerConfig = (triggerId: string, config: Record<string, unknown>) => {
    setTriggers((prev) =>
      prev.map((t) => (t.id === triggerId ? { ...t, config } : t)),
    );
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">Triggers</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Configure when this agent should start working.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {isDirty && (
            <Button
              onClick={handleSave}
              disabled={saving}
              size="xs"
            >
              <Save className="h-3 w-3" />
              {saving ? "Saving..." : "Save"}
            </Button>
          )}
        </div>
      </div>

      <div className="space-y-2">
        {triggers.map((trigger) => (
          <div
            key={trigger.id}
            className="rounded-lg border px-4 py-3"
          >
            <div className="flex items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                {trigger.type === "on_assign" ? (
                  <Bot className="h-4 w-4 text-muted-foreground" />
                ) : trigger.type === "on_comment" ? (
                  <MessageSquare className="h-4 w-4 text-muted-foreground" />
                ) : (
                  <Timer className="h-4 w-4 text-muted-foreground" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium">
                  {trigger.type === "on_assign"
                    ? "On Issue Assign"
                    : trigger.type === "on_comment"
                      ? "On Comment"
                      : "Scheduled"}
                </div>
                <div className="text-xs text-muted-foreground">
                  {trigger.type === "on_assign"
                    ? "Runs when an issue is assigned to this agent"
                    : trigger.type === "on_comment"
                      ? "Runs when a member comments on the agent's issue"
                      : `Cron: ${(trigger.config as { cron?: string } | null)?.cron ?? "Not set"}`}
                </div>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => toggleTrigger(trigger.id)}
                  className={`relative h-5 w-9 rounded-full transition-colors ${
                    trigger.enabled ? "bg-primary" : "bg-muted"
                  }`}
                >
                  <span
                    className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${
                      trigger.enabled ? "left-4.5" : "left-0.5"
                    }`}
                  />
                </button>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  onClick={() => removeTrigger(trigger.id)}
                  className="text-muted-foreground hover:text-destructive"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>

            {trigger.type === "scheduled" && (
              <div className="mt-3 grid grid-cols-2 gap-3 pl-12">
                <div>
                  <Label className="text-xs text-muted-foreground">
                    Cron Expression
                  </Label>
                  <Input
                    type="text"
                    value={(trigger.config as { cron?: string }).cron ?? ""}
                    onChange={(e) =>
                      updateTriggerConfig(trigger.id, {
                        ...trigger.config,
                        cron: e.target.value,
                      })
                    }
                    placeholder="0 9 * * 1-5"
                    className="mt-1 text-xs font-mono"
                  />
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground">
                    Timezone
                  </Label>
                  <Input
                    type="text"
                    value={(trigger.config as { timezone?: string }).timezone ?? ""}
                    onChange={(e) =>
                      updateTriggerConfig(trigger.id, {
                        ...trigger.config,
                        timezone: e.target.value,
                      })
                    }
                    placeholder="UTC"
                    className="mt-1 text-xs"
                  />
                </div>
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="flex gap-2">
        <Button
          variant="outline"
          size="xs"
          onClick={() => addTrigger("on_assign")}
          className="border-dashed text-muted-foreground hover:text-foreground"
        >
          <Bot className="h-3 w-3" />
          Add On Assign
        </Button>
        <Button
          variant="outline"
          size="xs"
          onClick={() => addTrigger("on_comment")}
          className="border-dashed text-muted-foreground hover:text-foreground"
        >
          <MessageSquare className="h-3 w-3" />
          Add On Comment
        </Button>
        <Button
          variant="outline"
          size="xs"
          onClick={() => addTrigger("scheduled")}
          className="border-dashed text-muted-foreground hover:text-foreground"
        >
          <Timer className="h-3 w-3" />
          Add Scheduled
        </Button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tasks Tab
// ---------------------------------------------------------------------------

function TasksTab({ agent }: { agent: Agent }) {
  const [tasks, setTasks] = useState<AgentTask[]>([]);
  const [loading, setLoading] = useState(true);
  const issues = useIssueStore((s) => s.issues);

  useEffect(() => {
    setLoading(true);
    api
      .listAgentTasks(agent.id)
      .then(setTasks)
      .catch(() => setTasks([]))
      .finally(() => setLoading(false));
  }, [agent.id]);

  if (loading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3 rounded-lg border px-4 py-3">
            <Skeleton className="h-4 w-4 rounded shrink-0" />
            <div className="flex-1 space-y-1.5">
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-3 w-1/3" />
            </div>
            <Skeleton className="h-4 w-16" />
          </div>
        ))}
      </div>
    );
  }

  // Sort: active tasks (running > dispatched > queued) first, then completed/failed by date
  const activeStatuses = ["running", "dispatched", "queued"];
  const sortedTasks = [...tasks].sort((a, b) => {
    const aActive = activeStatuses.indexOf(a.status);
    const bActive = activeStatuses.indexOf(b.status);
    const aIsActive = aActive !== -1;
    const bIsActive = bActive !== -1;
    if (aIsActive && !bIsActive) return -1;
    if (!aIsActive && bIsActive) return 1;
    if (aIsActive && bIsActive) return aActive - bActive;
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });

  const issueMap = new Map(issues.map((i) => [i.id, i]));

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold">Task Queue</h3>
        <p className="text-xs text-muted-foreground mt-0.5">
          Issues assigned to this agent and their execution status.
        </p>
      </div>

      {tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12">
          <ListTodo className="h-8 w-8 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">No tasks in queue</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Assign an issue to this agent to get started.
          </p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {sortedTasks.map((task) => {
            const config = taskStatusConfig[task.status] ?? taskStatusConfig.queued!;
            const Icon = config.icon;
            const issue = issueMap.get(task.issue_id);
            const isActive = task.status === "running" || task.status === "dispatched";
            const isRunning = task.status === "running";

            return (
              <div
                key={task.id}
                className={`flex items-center gap-3 rounded-lg border px-4 py-3 ${
                  isRunning
                    ? "border-success/40 bg-success/5"
                    : task.status === "dispatched"
                      ? "border-info/40 bg-info/5"
                      : ""
                }`}
              >
                <Icon
                  className={`h-4 w-4 shrink-0 ${config.color} ${
                    isRunning ? "animate-spin" : ""
                  }`}
                />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    {issue && (
                      <span className="shrink-0 text-xs font-mono text-muted-foreground">
                        {issue.identifier}
                      </span>
                    )}
                    <span className={`text-sm truncate ${isActive ? "font-medium" : ""}`}>
                      {issue?.title ?? `Issue ${task.issue_id.slice(0, 8)}...`}
                    </span>
                  </div>
                  <div className="text-xs text-muted-foreground mt-0.5">
                    {isRunning && task.started_at
                      ? `Started ${new Date(task.started_at).toLocaleString()}`
                      : task.status === "dispatched" && task.dispatched_at
                        ? `Dispatched ${new Date(task.dispatched_at).toLocaleString()}`
                        : task.status === "completed" && task.completed_at
                          ? `Completed ${new Date(task.completed_at).toLocaleString()}`
                          : task.status === "failed" && task.completed_at
                            ? `Failed ${new Date(task.completed_at).toLocaleString()}`
                            : `Queued ${new Date(task.created_at).toLocaleString()}`}
                  </div>
                </div>
                <span className={`shrink-0 text-xs font-medium ${config.color}`}>
                  {config.label}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Settings Tab
// ---------------------------------------------------------------------------

function SettingsTab({
  agent,
  onSave,
}: {
  agent: Agent;
  onSave: (updates: Partial<Agent>) => Promise<void>;
}) {
  const [name, setName] = useState(agent.name);
  const [description, setDescription] = useState(agent.description ?? "");
  const [visibility, setVisibility] = useState<AgentVisibility>(agent.visibility);
  const [codeAccess, setCodeAccess] = useState(agent.github_code_access ?? "read");
  const [providers, setProviders] = useState<string[]>(agent.providers ?? []);
  const [defaultDaemonId, setDefaultDaemonId] = useState<string | null>(agent.default_daemon_id ?? null);
  const [maxConcurrentTasks, setMaxConcurrentTasks] = useState<string>(String(agent.max_concurrent_tasks ?? 6));
  const [saving, setSaving] = useState(false);
  const { upload, uploading } = useFileUpload();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const daemons = useRuntimeStore((s) => s.daemons);
  const fetchAllRuntimes = useRuntimeStore((s) => s.fetchAll);
  useEffect(() => { fetchAllRuntimes(); }, [fetchAllRuntimes]);

  const handleAvatarUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = "";
    try {
      const result = await upload(file);
      if (!result) return;
      await onSave({ avatar_url: result.link });
      toast.success("Avatar updated");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to upload avatar");
    }
  };

  const parsedMaxConcurrentTasks = parseInt(maxConcurrentTasks, 10);

  const dirty =
    name !== agent.name ||
    description !== (agent.description ?? "") ||
    visibility !== agent.visibility ||
    codeAccess !== (agent.github_code_access ?? "read") ||
    JSON.stringify(providers) !== JSON.stringify(agent.providers ?? []) ||
    defaultDaemonId !== (agent.default_daemon_id ?? null) ||
    parsedMaxConcurrentTasks !== (agent.max_concurrent_tasks ?? 6);

  const handleSave = async () => {
    if (!name.trim()) {
      toast.error("Name is required");
      return;
    }
    if (!Number.isInteger(parsedMaxConcurrentTasks) || parsedMaxConcurrentTasks <= 0) {
      toast.error("Max concurrent tasks must be a positive integer");
      return;
    }
    setSaving(true);
    try {
      await onSave({ name: name.trim(), description, visibility, providers, github_code_access: codeAccess as Agent["github_code_access"], default_daemon_id: defaultDaemonId, max_concurrent_tasks: parsedMaxConcurrentTasks });
      toast.success("Settings saved");
    } catch {
      toast.error("Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="max-w-lg space-y-6">
      <div>
        <Label className="text-xs text-muted-foreground">Avatar</Label>
        <div className="mt-1.5 flex items-center gap-4">
          <button
            type="button"
            className="group relative h-16 w-16 shrink-0 rounded-full bg-muted overflow-hidden focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
          >
            <ActorAvatar actorType="agent" actorId={agent.id} size={64} className="rounded-none" />
            <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
              {uploading ? (
                <Loader2 className="h-5 w-5 animate-spin text-white" />
              ) : (
                <Camera className="h-5 w-5 text-white" />
              )}
            </div>
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={handleAvatarUpload}
          />
          <div className="text-xs text-muted-foreground">
            Click to upload avatar
          </div>
        </div>
      </div>

      <div>
        <Label className="text-xs text-muted-foreground">Name</Label>
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="mt-1"
        />
      </div>

      <div>
        <Label className="text-xs text-muted-foreground">Description</Label>
        <Input
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="What does this agent do?"
          className="mt-1"
        />
      </div>

      <div>
        <Label className="text-xs text-muted-foreground">Visibility</Label>
        <div className="mt-1.5 flex gap-2">
          <button
            type="button"
            onClick={() => setVisibility("workspace")}
            className={`flex flex-1 items-center gap-2 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
              visibility === "workspace"
                ? "border-primary bg-primary/5"
                : "border-border hover:bg-muted"
            }`}
          >
            <Globe className="h-4 w-4 shrink-0 text-muted-foreground" />
            <div className="text-left">
              <div className="font-medium">Workspace</div>
              <div className="text-xs text-muted-foreground">All members can assign</div>
            </div>
          </button>
          <button
            type="button"
            onClick={() => setVisibility("private")}
            className={`flex flex-1 items-center gap-2 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
              visibility === "private"
                ? "border-primary bg-primary/5"
                : "border-border hover:bg-muted"
            }`}
          >
            <Lock className="h-4 w-4 shrink-0 text-muted-foreground" />
            <div className="text-left">
              <div className="font-medium">Private</div>
              <div className="text-xs text-muted-foreground">Only you can assign</div>
            </div>
          </button>
        </div>
      </div>


      <div>
        <Label className="text-xs text-muted-foreground">GitHub Code Access</Label>
        <p className="text-xs text-muted-foreground mt-0.5 mb-1.5">
          Controls what the agent can do with repository code when a GitHub App is connected.
        </p>
        <div className="mt-1.5 flex gap-2">
          {([
            { value: "read", label: "Read", desc: "Read-only code access" },
            { value: "write", label: "Write", desc: "Push branches (PRs opened by reviewer)" },
            { value: "admin", label: "Admin", desc: "Push, open & merge PRs" },
          ] as const).map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => setCodeAccess(opt.value)}
              className={`flex flex-1 items-center gap-2 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
                codeAccess === opt.value
                  ? "border-primary bg-primary/5"
                  : "border-border hover:bg-muted"
              }`}
            >
              <div className="text-left">
                <div className="font-medium">{opt.label}</div>
                <div className="text-xs text-muted-foreground">{opt.desc}</div>
              </div>
            </button>
          ))}
        </div>
      </div>

      <div>
        <Label className="text-xs text-muted-foreground">Providers</Label>
        <div className="mt-1.5 flex gap-2 flex-wrap">
          {SUPPORTED_PROVIDERS.map((p) => {
            const selected = providers.includes(p.key);
            return (
              <button
                key={p.key}
                type="button"
                onClick={() => {
                  const next = selected
                    ? providers.filter((k) => k !== p.key)
                    : [...providers, p.key];
                  if (next.length === 0) return;
                  setProviders(next);
                }}
                className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors ${
                  selected
                    ? "border-primary bg-primary/5"
                    : "border-border hover:bg-muted"
                }`}
              >
                <span className="font-medium">{p.label}</span>
              </button>
            );
          })}
        </div>
        <p className="mt-1.5 text-xs text-muted-foreground">
          Select one or more. Tasks will be dispatched to any online environment with a matching provider.
        </p>
      </div>

      <div>
        <Label className="text-xs text-muted-foreground">Default Environment</Label>
        <p className="text-xs text-muted-foreground mt-0.5 mb-1.5">
          Bind this agent to a specific daemon. When assigned to issues, this environment will be used automatically.
        </p>
        <div className="mt-1.5">
          {defaultDaemonId ? (
            <div className="flex items-center gap-2 rounded-lg border border-primary bg-primary/5 px-3 py-2 text-sm">
              <Monitor className="h-4 w-4 shrink-0 text-muted-foreground" />
              <span className="font-medium">
                {daemons.find((d) => d.id === defaultDaemonId)?.device_name ?? "Unknown daemon"}
              </span>
              <button
                type="button"
                onClick={() => setDefaultDaemonId(null)}
                className="ml-auto rounded-sm p-0.5 hover:bg-accent transition-colors"
              >
                <X className="h-3.5 w-3.5 text-muted-foreground" />
              </button>
            </div>
          ) : (
            <Popover>
              <PopoverTrigger
                render={
                  <button
                    type="button"
                    className="flex items-center gap-2 rounded-lg border border-dashed px-3 py-2 text-sm text-muted-foreground hover:bg-muted transition-colors"
                  >
                    <Monitor className="h-4 w-4" />
                    <span>Select environment...</span>
                  </button>
                }
              />
              <PopoverContent align="start" className="w-52 p-1">
                {daemons.length === 0 && (
                  <div className="px-2 py-3 text-center text-sm text-muted-foreground">No daemons registered</div>
                )}
                {daemons.map((d) => (
                  <button
                    key={d.id}
                    type="button"
                    onClick={() => setDefaultDaemonId(d.id)}
                    className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
                  >
                    <span
                      className={`h-1.5 w-1.5 shrink-0 rounded-full ${d.status === "online" ? "bg-green-500" : "bg-muted-foreground/40"}`}
                    />
                    <span>{d.device_name || d.daemon_id}</span>
                  </button>
                ))}
              </PopoverContent>
            </Popover>
          )}
        </div>
      </div>

      <div>
        <Label className="text-xs text-muted-foreground">Max Concurrent Tasks</Label>
        <p className="text-xs text-muted-foreground mt-0.5 mb-1.5">
          Maximum number of tasks this agent can run simultaneously. Defaults to 6.
        </p>
        <Input
          type="number"
          min={1}
          value={maxConcurrentTasks}
          onChange={(e) => setMaxConcurrentTasks(e.target.value)}
          className="mt-1 w-32"
        />
      </div>

      <Button onClick={handleSave} disabled={!dirty || saving} size="sm">
        {saving ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <Save className="h-3.5 w-3.5 mr-1.5" />}
        Save Changes
      </Button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Agent Detail
// ---------------------------------------------------------------------------

type DetailTab = "instructions" | "skills" | "tools" | "triggers" | "tasks" | "settings";

const detailTabs: { id: DetailTab; label: string; icon: typeof FileText }[] = [
  { id: "instructions", label: "Instructions", icon: FileText },
  { id: "skills", label: "Skills", icon: BookOpenText },
  { id: "tools", label: "Tools", icon: Wrench },
  { id: "triggers", label: "Triggers", icon: Timer },
  { id: "tasks", label: "Tasks", icon: ListTodo },
  { id: "settings", label: "Settings", icon: Settings },
];

function AgentDetail({
  agent,
  onUpdate,
  onArchive,
  onRestore,
}: {
  agent: Agent;
  onUpdate: (id: string, data: Partial<Agent>) => Promise<void>;
  onArchive: (id: string) => Promise<void>;
  onRestore: (id: string) => Promise<void>;
}) {
  const st = statusConfig[agent.status];
  const [activeTab, setActiveTab] = useState<DetailTab>("instructions");
  const [confirmArchive, setConfirmArchive] = useState(false);
  const isArchived = !!agent.archived_at;

  return (
    <div className="flex h-full flex-col">
      {isArchived && (
        <div className="flex items-center gap-2 bg-muted/50 px-4 py-2 text-xs text-muted-foreground border-b">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          <span className="flex-1">This agent is archived. It cannot be assigned or mentioned.</span>
          <Button variant="outline" size="sm" className="h-6 text-xs" onClick={() => onRestore(agent.id)}>
            Restore
          </Button>
        </div>
      )}

      <div className="flex h-12 shrink-0 items-center gap-3 border-b px-4">
        <ActorAvatar actorType="agent" actorId={agent.id} size={28} className={`rounded-md ${isArchived ? "opacity-50" : ""}`} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className={`text-sm font-semibold truncate ${isArchived ? "text-muted-foreground" : ""}`}>{agent.name}</h2>
            {isArchived ? (
              <span className="rounded-md bg-muted px-1.5 py-0.5 text-xs font-medium text-muted-foreground">
                Archived
              </span>
            ) : (
              <span className={`flex items-center gap-1.5 text-xs ${st.color}`}>
                <span className={`h-1.5 w-1.5 rounded-full ${st.dot}`} />
                {st.label}
              </span>
            )}
            {agent.providers?.length > 0 && (
              <span className="flex items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-xs font-medium text-muted-foreground">
                {agent.providers.join(", ")}
              </span>
            )}
          </div>
        </div>
        {!isArchived && (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="ghost" size="icon-sm" />
              }
            >
              <MoreHorizontal className="h-4 w-4 text-muted-foreground" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                className="text-destructive"
                onClick={() => setConfirmArchive(true)}
              >
                <Trash2 className="h-3.5 w-3.5" />
                Archive Agent
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>

      {/* Tabs */}
      <div className="flex border-b px-6">
        {detailTabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-1.5 border-b-2 px-3 py-2.5 text-xs font-medium transition-colors ${
              activeTab === tab.id
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            <tab.icon className="h-3.5 w-3.5" />
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="flex-1 overflow-y-auto p-6">
        {activeTab === "instructions" && (
          <InstructionsTab
            agent={agent}
            onSave={(instructions) => onUpdate(agent.id, { instructions })}
          />
        )}
        {activeTab === "skills" && (
          <SkillsTab agent={agent} />
        )}
        {activeTab === "tools" && (
          <ToolsTab
            agent={agent}
            onSave={(tools) => onUpdate(agent.id, { tools })}
          />
        )}
        {activeTab === "triggers" && (
          <TriggersTab
            agent={agent}
            onSave={(triggers) => onUpdate(agent.id, { triggers })}
          />
        )}
        {activeTab === "tasks" && <TasksTab agent={agent} />}
        {activeTab === "settings" && (
          <SettingsTab
            agent={agent}
            onSave={(updates) => onUpdate(agent.id, updates)}
          />
        )}
      </div>

      {/* Archive Confirmation */}
      {confirmArchive && (
        <Dialog open onOpenChange={(v) => { if (!v) setConfirmArchive(false); }}>
          <DialogContent className="max-w-sm" showCloseButton={false}>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-destructive/10">
                <AlertCircle className="h-5 w-5 text-destructive" />
              </div>
              <DialogHeader className="flex-1 gap-1">
                <DialogTitle className="text-sm font-semibold">Archive agent?</DialogTitle>
                <DialogDescription className="text-xs">
                  &quot;{agent.name}&quot; will be archived. It won&apos;t be assignable or mentionable, but all history is preserved. You can restore it later.
                </DialogDescription>
              </DialogHeader>
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setConfirmArchive(false)}>
                Cancel
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  setConfirmArchive(false);
                  onArchive(agent.id);
                }}
              >
                Archive
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function AgentsPage() {
  const isMobile = useIsMobile();
  const isLoading = useAuthStore((s) => s.isLoading);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const allAgents = useWorkspaceStore((s) => s.agents);
  const refreshAgents = useWorkspaceStore((s) => s.refreshAgents);
  const fetchAllRuntimes = useRuntimeStore((s) => s.fetchAll);
  const [selectedId, setSelectedId] = useState<string>("");
  const [showCreate, setShowCreate] = useState(false);
  const [showArchived, setShowArchived] = useState(false);

  useEffect(() => {
    fetchAllRuntimes();
  }, [fetchAllRuntimes]);

  const activeAgents = allAgents.filter((a) => !a.archived_at);
  const archivedAgents = allAgents.filter((a) => !!a.archived_at);
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "multica_agents_layout",
  });

  // Select first agent on initial load
  useEffect(() => {
    if (activeAgents.length > 0 && !selectedId) {
      setSelectedId(activeAgents[0]!.id);
    }
  }, [activeAgents, selectedId]);

  const handleCreate = async (data: CreateAgentRequest) => {
    const agent = await api.createAgent(data);
    await refreshAgents();
    setSelectedId(agent.id);
  };

  const handleUpdate = async (id: string, data: Record<string, unknown>) => {
    try {
      await api.updateAgent(id, data as UpdateAgentRequest);
      await refreshAgents();
      toast.success("Agent updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update agent");
      throw e;
    }
  };

  const handleArchive = async (id: string) => {
    try {
      await api.archiveAgent(id);
      await refreshAgents();
      toast.success("Agent archived");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to archive agent");
    }
  };

  const handleRestore = async (id: string) => {
    try {
      await api.restoreAgent(id);
      await refreshAgents();
      toast.success("Agent restored");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to restore agent");
    }
  };

  const selected = allAgents.find((a) => a.id === selectedId) ?? null;

  if (isLoading) {
    return (
      <div className="flex flex-1 min-h-0">
        {/* List skeleton */}
        <div className="w-72 border-r">
          <div className="flex h-12 items-center justify-between border-b px-4">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-6 w-6 rounded" />
          </div>
          <div className="divide-y">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 px-4 py-3">
                <Skeleton className="h-8 w-8 rounded-full" />
                <div className="flex-1 space-y-1.5">
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-3 w-16" />
                </div>
              </div>
            ))}
          </div>
        </div>
        {/* Detail skeleton */}
        <div className="flex-1 p-6 space-y-6">
          <div className="flex items-center gap-3">
            <Skeleton className="h-10 w-10 rounded-full" />
            <div className="space-y-1.5">
              <Skeleton className="h-5 w-32" />
              <Skeleton className="h-3 w-20" />
            </div>
          </div>
          <div className="space-y-3">
            <Skeleton className="h-8 w-full rounded-lg" />
            <Skeleton className="h-8 w-full rounded-lg" />
            <Skeleton className="h-8 w-3/4 rounded-lg" />
          </div>
        </div>
      </div>
    );
  }

  const agentList = (
    <div className="overflow-y-auto h-full border-r">
      <div className="flex h-12 items-center justify-between border-b px-4">
        <h1 className="text-sm font-semibold">Agents</h1>
        <Button
          variant="ghost"
          size="icon-xs"
          onClick={() => setShowCreate(true)}
        >
          <Plus className="h-4 w-4 text-muted-foreground" />
        </Button>
      </div>
      {activeAgents.length === 0 && archivedAgents.length === 0 ? (
        <div className="flex flex-col items-center justify-center px-4 py-12">
          <Bot className="h-8 w-8 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">No agents yet</p>
          <Button
            onClick={() => setShowCreate(true)}
            size="xs"
            className="mt-3"
          >
            <Plus className="h-3 w-3" />
            Create Agent
          </Button>
        </div>
      ) : (
        <>
          <div className="divide-y">
            {activeAgents.map((agent) => (
              <AgentListItem
                key={agent.id}
                agent={agent}
                isSelected={agent.id === selectedId}
                onClick={() => setSelectedId(agent.id)}
              />
            ))}
          </div>
          {archivedAgents.length > 0 && (
            <>
              <button
                type="button"
                onClick={() => setShowArchived(!showArchived)}
                className="flex w-full items-center gap-1.5 px-4 py-2 text-xs text-muted-foreground hover:text-foreground transition-colors border-t"
              >
                <ChevronDown className={cn("h-3 w-3 transition-transform", !showArchived && "-rotate-90")} />
                <span>Archived ({archivedAgents.length})</span>
              </button>
              {showArchived && (
                <div className="divide-y">
                  {archivedAgents.map((agent) => (
                    <AgentListItem
                      key={agent.id}
                      agent={agent}
                      isSelected={agent.id === selectedId}
                      onClick={() => setSelectedId(agent.id)}
                    />
                  ))}
                </div>
              )}
            </>
          )}
        </>
      )}
    </div>
  );

  const agentDetail = selected ? (
    <AgentDetail
      key={selected.id}
      agent={selected}
      onUpdate={handleUpdate}
      onArchive={handleArchive}
      onRestore={handleRestore}
    />
  ) : (
    <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
      <Bot className="h-10 w-10 text-muted-foreground/30" />
      <p className="mt-3 text-sm">Select an agent to view details</p>
      <Button
        onClick={() => setShowCreate(true)}
        size="xs"
        className="mt-3"
      >
        <Plus className="h-3 w-3" />
        Create Agent
      </Button>
    </div>
  );

  if (isMobile) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        {selected ? (
          <>
            <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
              <Button variant="ghost" size="icon-xs" onClick={() => setSelectedId("")}>
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <span className="text-sm font-medium truncate">Agents</span>
            </div>
            <div className="flex-1 min-h-0 overflow-hidden">
              {agentDetail}
            </div>
          </>
        ) : (
          agentList
        )}
        {showCreate && (
          <CreateAgentDialog
            onClose={() => setShowCreate(false)}
            onCreate={handleCreate}
          />
        )}
      </div>
    );
  }

  return (
    <ResizablePanelGroup
      orientation="horizontal"
      className="flex-1 min-h-0"
      defaultLayout={defaultLayout}
      onLayoutChanged={onLayoutChanged}
    >
      <ResizablePanel id="list" defaultSize={280} minSize={240} maxSize={400} groupResizeBehavior="preserve-pixel-size">
        {agentList}
      </ResizablePanel>

      <ResizableHandle />

      <ResizablePanel id="detail" minSize="50%">
        {agentDetail}
      </ResizablePanel>

      {showCreate && (
        <CreateAgentDialog
          onClose={() => setShowCreate(false)}
          onCreate={handleCreate}
        />
      )}
    </ResizablePanelGroup>
  );
}
