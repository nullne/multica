"use client";

import { useEffect, useState } from "react";
import { CheckCircle2, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ActorAvatar } from "@/components/common/actor-avatar";
import type {
  IssueAssigneeType,
  IssuePriority,
  RecurringTemplate,
  CreateRecurringTemplateRequest,
  UpdateRecurringTemplateRequest,
} from "@/shared/types";
import { api } from "@/shared/api";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { AssigneePicker } from "@/features/issues/components/pickers";
import { useRecurringTemplateStore } from "../store";

const PRIORITY_OPTIONS: { value: IssuePriority; label: string }[] = [
  { value: "urgent", label: "Urgent" },
  { value: "high", label: "High" },
  { value: "medium", label: "Medium" },
  { value: "low", label: "Low" },
  { value: "none", label: "None" },
];

const CRON_EXAMPLES = [
  { label: "Every day at 9am", value: "0 9 * * *" },
  { label: "Every Monday at 8am", value: "0 8 * * 1" },
  { label: "First day of month", value: "0 0 1 * *" },
  { label: "Every weekday at 6pm", value: "0 18 * * 1-5" },
];

interface TemplateFormData {
  title: string;
  description: string;
  priority: IssuePriority;
  schedule: string;
  timezone: string;
  enabled: boolean;
  due_date_offset_hours: string;
  max_runs: string;
  assignee_type: IssueAssigneeType | null;
  assignee_id: string | null;
}

const DEFAULT_FORM: TemplateFormData = {
  title: "",
  description: "",
  priority: "medium",
  schedule: "",
  timezone: "UTC",
  enabled: true,
  due_date_offset_hours: "",
  max_runs: "",
  assignee_type: null,
  assignee_id: null,
};

function templateToForm(t: RecurringTemplate): TemplateFormData {
  return {
    title: t.title,
    description: t.description ?? "",
    priority: t.priority,
    schedule: t.schedule,
    timezone: t.timezone,
    enabled: t.enabled,
    due_date_offset_hours: t.due_date_offset_hours != null ? String(t.due_date_offset_hours) : "",
    max_runs: t.max_runs != null ? String(t.max_runs) : "",
    assignee_type: t.assignee_type ?? null,
    assignee_id: t.assignee_id ?? null,
  };
}

function formatNextRun(nextRunAt?: string): string {
  if (!nextRunAt) return "—";
  const d = new Date(nextRunAt);
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function isCompleted(t: RecurringTemplate): boolean {
  return t.max_runs != null && t.successful_runs_count >= t.max_runs;
}

function AssigneeField({
  assigneeType,
  assigneeId,
  onChange,
}: {
  assigneeType: IssueAssigneeType | null;
  assigneeId: string | null;
  onChange: (type: IssueAssigneeType | null, id: string | null) => void;
}) {
  const { getActorName } = useActorName();
  const label =
    assigneeType && assigneeId ? getActorName(assigneeType, assigneeId) : "Unassigned";

  return (
    <AssigneePicker
      assigneeType={assigneeType}
      assigneeId={assigneeId}
      onUpdate={(updates) => {
        const nextType = (updates.assignee_type ?? null) as IssueAssigneeType | null;
        const nextId = (updates.assignee_id ?? null) as string | null;
        onChange(nextType, nextId);
      }}
      align="start"
      triggerRender={
        <Button
          type="button"
          variant="outline"
          className="w-full justify-start font-normal"
        />
      }
      trigger={
        <>
          {assigneeType && assigneeId ? (
            <ActorAvatar actorType={assigneeType} actorId={assigneeId} size={18} />
          ) : null}
          <span className={assigneeType && assigneeId ? "" : "text-muted-foreground"}>
            {label}
          </span>
        </>
      }
    />
  );
}

function TemplateFormDialog({
  open,
  onClose,
  initial,
  onSubmit,
  submitLabel,
}: {
  open: boolean;
  onClose: () => void;
  initial?: TemplateFormData;
  onSubmit: (data: TemplateFormData) => Promise<void>;
  submitLabel: string;
}) {
  const [form, setForm] = useState<TemplateFormData>(initial ?? DEFAULT_FORM);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) setForm(initial ?? DEFAULT_FORM);
  }, [open, initial]);

  const set = (patch: Partial<TemplateFormData>) =>
    setForm((f) => ({ ...f, ...patch }));

  const maxRunsValid =
    form.max_runs.trim() === "" ||
    (Number.isInteger(Number(form.max_runs)) && Number(form.max_runs) > 0);

  const handleSubmit = async () => {
    if (!form.title.trim() || !form.schedule.trim() || !maxRunsValid || submitting) return;
    setSubmitting(true);
    try {
      await onSubmit(form);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{submitLabel === "Create" ? "New recurring template" : "Edit recurring template"}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Title */}
          <div className="space-y-1">
            <label className="text-xs font-medium">Title *</label>
            <Input
              placeholder="Weekly standup issue"
              value={form.title}
              onChange={(e) => set({ title: e.target.value })}
            />
          </div>

          {/* Description */}
          <div className="space-y-1">
            <label className="text-xs font-medium">Description</label>
            <Textarea
              placeholder="Optional description for issues created from this template"
              value={form.description}
              onChange={(e) => set({ description: e.target.value })}
              rows={3}
            />
          </div>

          {/* Priority + Assignee */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="text-xs font-medium">Priority</label>
              <Select value={form.priority} onValueChange={(v) => set({ priority: v as IssuePriority })}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PRIORITY_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <label className="text-xs font-medium">Assignee</label>
              <AssigneeField
                assigneeType={form.assignee_type}
                assigneeId={form.assignee_id}
                onChange={(type, id) => set({ assignee_type: type, assignee_id: id })}
              />
            </div>
          </div>

          {/* Schedule */}
          <div className="space-y-1">
            <label className="text-xs font-medium">Schedule (cron expression) *</label>
            <Input
              placeholder="0 9 * * 1"
              value={form.schedule}
              onChange={(e) => set({ schedule: e.target.value })}
              className="font-mono"
            />
            <div className="flex flex-wrap gap-1.5 mt-1.5">
              {CRON_EXAMPLES.map((ex) => (
                <button
                  key={ex.value}
                  type="button"
                  onClick={() => set({ schedule: ex.value })}
                  className="text-xs px-2 py-0.5 rounded-full border border-border hover:bg-muted transition-colors"
                >
                  {ex.label}
                </button>
              ))}
            </div>
            <p className="text-xs text-muted-foreground">
              5-field cron: minute hour day-of-month month day-of-week
            </p>
          </div>

          {/* Timezone */}
          <div className="space-y-1">
            <label className="text-xs font-medium">Timezone</label>
            <Input
              placeholder="UTC"
              value={form.timezone}
              onChange={(e) => set({ timezone: e.target.value })}
            />
            <p className="text-xs text-muted-foreground">IANA timezone name, e.g. America/New_York</p>
          </div>

          {/* Due date offset + Max runs */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="text-xs font-medium">Due date offset (hours)</label>
              <Input
                type="number"
                placeholder="Optional, e.g. 72 for 3 days"
                value={form.due_date_offset_hours}
                onChange={(e) => set({ due_date_offset_hours: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-medium">Max runs</label>
              <Input
                type="number"
                min={1}
                placeholder="Blank = unlimited"
                value={form.max_runs}
                onChange={(e) => set({ max_runs: e.target.value })}
                aria-invalid={!maxRunsValid}
              />
              <p className="text-xs text-muted-foreground">
                {maxRunsValid
                  ? "Stops creating issues after this many successful runs."
                  : "Must be a positive integer."}
              </p>
            </div>
          </div>

          {/* Enabled */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium">Enabled</p>
              <p className="text-xs text-muted-foreground">Disabled templates are not scheduled</p>
            </div>
            <Switch
              checked={form.enabled}
              onCheckedChange={(v) => set({ enabled: v })}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={submitting}>Cancel</Button>
          <Button
            onClick={handleSubmit}
            disabled={!form.title.trim() || !form.schedule.trim() || !maxRunsValid || submitting}
          >
            {submitting ? "Saving..." : submitLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function buildCreatePayload(form: TemplateFormData): CreateRecurringTemplateRequest {
  // For create, omit fields the user did not set so the server applies its
  // defaults (e.g. null assignee, null description).
  const payload: CreateRecurringTemplateRequest = {
    title: form.title.trim(),
    priority: form.priority,
    schedule: form.schedule.trim(),
    timezone: form.timezone.trim() || "UTC",
    enabled: form.enabled,
  };
  const description = form.description.trim();
  if (description) payload.description = description;
  if (form.assignee_type && form.assignee_id) {
    payload.assignee_type = form.assignee_type;
    payload.assignee_id = form.assignee_id;
  }
  if (form.due_date_offset_hours.trim()) {
    payload.due_date_offset_hours = parseInt(form.due_date_offset_hours, 10);
  }
  if (form.max_runs.trim()) {
    payload.max_runs = parseInt(form.max_runs, 10);
  }
  return payload;
}

function buildUpdatePayload(form: TemplateFormData): UpdateRecurringTemplateRequest {
  // For update, send every nullable field explicitly — null when cleared,
  // the value when set — so the server can distinguish "clear" from "leave
  // unchanged". The backend's PATCH handler reads the JSON keys, not the
  // pointer values, so an explicit null is required to clear a field.
  const description = form.description.trim();
  const offset = form.due_date_offset_hours.trim();
  const maxRuns = form.max_runs.trim();
  return {
    title: form.title.trim(),
    description: description ? description : null,
    priority: form.priority,
    schedule: form.schedule.trim(),
    timezone: form.timezone.trim() || "UTC",
    enabled: form.enabled,
    assignee_type: form.assignee_type,
    assignee_id: form.assignee_id,
    due_date_offset_hours: offset ? parseInt(offset, 10) : null,
    max_runs: maxRuns ? parseInt(maxRuns, 10) : null,
  };
}

export function RecurringTemplateManager() {
  const templates = useRecurringTemplateStore((s) => s.templates);
  const loading = useRecurringTemplateStore((s) => s.loading);
  const includeInactive = useRecurringTemplateStore((s) => s.includeInactive);
  const fetch = useRecurringTemplateStore((s) => s.fetch);
  const setIncludeInactive = useRecurringTemplateStore((s) => s.setIncludeInactive);

  const user = useAuthStore((s) => s.user);
  const workspaceId = useWorkspaceStore((s) => s.workspace?.id);
  const members = useWorkspaceStore((s) => s.members);
  const { getActorName } = useActorName();
  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManage = currentMember?.role === "owner" || currentMember?.role === "admin";

  const [showCreate, setShowCreate] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<RecurringTemplate | null>(null);
  const [deletingTemplate, setDeletingTemplate] = useState<RecurringTemplate | null>(null);
  const [togglingId, setTogglingId] = useState<string | null>(null);

  // Refetch when workspace changes so stale data from a previous workspace
  // does not remain visible after switching.
  useEffect(() => {
    if (workspaceId) fetch();
  }, [workspaceId, fetch]);

  const handleCreate = async (form: TemplateFormData) => {
    try {
      await api.createRecurringTemplate(buildCreatePayload(form));
      // Refetch so the new template lands in the correct active/inactive bucket.
      await fetch();
      setShowCreate(false);
      toast.success("Template created");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create template");
      throw err;
    }
  };

  const handleUpdate = async (form: TemplateFormData) => {
    if (!editingTemplate) return;
    try {
      await api.updateRecurringTemplate(editingTemplate.id, buildUpdatePayload(form));
      // Refetch so changes to enabled/max_runs that move the template across the
      // active/inactive boundary are reflected in the visible list.
      await fetch();
      setEditingTemplate(null);
      toast.success("Template updated");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update template");
      throw err;
    }
  };

  const handleToggleEnabled = async (template: RecurringTemplate) => {
    setTogglingId(template.id);
    try {
      await api.updateRecurringTemplate(template.id, { enabled: !template.enabled });
      await fetch();
    } catch {
      toast.error("Failed to update template");
    } finally {
      setTogglingId(null);
    }
  };

  const handleDelete = async () => {
    if (!deletingTemplate) return;
    try {
      await api.deleteRecurringTemplate(deletingTemplate.id);
      useRecurringTemplateStore.getState().removeTemplate(deletingTemplate.id);
      toast.success("Template deleted");
    } catch {
      toast.error("Failed to delete template");
    } finally {
      setDeletingTemplate(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">Recurring Templates</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            Automatically create issues on a schedule.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-2 text-xs text-muted-foreground">
            <Switch
              checked={includeInactive}
              onCheckedChange={(v) => setIncludeInactive(v)}
              className="scale-75"
            />
            Show inactive
          </label>
          {canManage && (
            <Button size="sm" variant="outline" onClick={() => setShowCreate(true)}>
              <Plus className="h-4 w-4 mr-1" />
              New template
            </Button>
          )}
        </div>
      </div>

      <div className="divide-y rounded-lg border">
        {loading && (
          <div className="px-4 py-8 text-center text-sm text-muted-foreground">
            Loading...
          </div>
        )}
        {!loading && templates.length === 0 && (
          <div className="px-4 py-8 text-center text-sm text-muted-foreground">
            {includeInactive
              ? "No recurring templates yet."
              : "No active recurring templates."}
            {canManage ? " Create one to automatically generate issues on a schedule." : ""}
          </div>
        )}
        {templates.map((t) => {
          const completed = isCompleted(t);
          return (
            <div key={t.id} className="flex items-start gap-3 px-4 py-3 group">
              <div className="flex-1 min-w-0 space-y-0.5">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-sm font-medium truncate">{t.title}</span>
                  {completed ? (
                    <Badge variant="secondary" className="text-[10px] px-1.5 py-0 shrink-0">
                      <CheckCircle2 className="h-2.5 w-2.5 mr-0.5" />
                      completed
                    </Badge>
                  ) : (
                    <Badge variant={t.enabled ? "default" : "secondary"} className="text-[10px] px-1.5 py-0 shrink-0">
                      {t.enabled ? "enabled" : "disabled"}
                    </Badge>
                  )}
                  <Badge variant="outline" className="text-[10px] px-1.5 py-0 font-mono shrink-0">
                    {t.schedule}
                  </Badge>
                  <span className="text-[10px] text-muted-foreground shrink-0">{t.timezone}</span>
                </div>
                {t.description && (
                  <p className="text-xs text-muted-foreground truncate">{t.description}</p>
                )}
                <div className="flex items-center gap-3 text-xs text-muted-foreground">
                  <span className="flex items-center gap-1">
                    <RefreshCw className="h-3 w-3" />
                    Next: {formatNextRun(t.next_run_at ?? undefined)}
                  </span>
                  <span>
                    Runs: {t.successful_runs_count}
                    {t.max_runs != null ? ` / ${t.max_runs}` : ""}
                  </span>
                  {t.assignee_type && t.assignee_id && (
                    <span className="flex items-center gap-1">
                      <ActorAvatar actorType={t.assignee_type} actorId={t.assignee_id} size={14} />
                      <span>{getActorName(t.assignee_type, t.assignee_id)}</span>
                    </span>
                  )}
                  {t.priority !== "medium" && (
                    <span className="capitalize">{t.priority}</span>
                  )}
                </div>
              </div>

              {canManage && (
                <div className="flex items-center gap-1 shrink-0">
                  <Switch
                    checked={t.enabled}
                    onCheckedChange={() => handleToggleEnabled(t)}
                    disabled={togglingId === t.id || completed}
                    className="scale-90"
                  />
                  <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button
                      size="icon-xs"
                      variant="ghost"
                      onClick={() => setEditingTemplate(t)}
                    >
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="icon-xs"
                      variant="ghost"
                      onClick={() => setDeletingTemplate(t)}
                      className="text-destructive hover:text-destructive"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {canManage && (
        <>
          <TemplateFormDialog
            open={showCreate}
            onClose={() => setShowCreate(false)}
            onSubmit={handleCreate}
            submitLabel="Create"
          />

          <TemplateFormDialog
            open={!!editingTemplate}
            onClose={() => setEditingTemplate(null)}
            initial={editingTemplate ? templateToForm(editingTemplate) : undefined}
            onSubmit={handleUpdate}
            submitLabel="Save"
          />

          <AlertDialog open={!!deletingTemplate} onOpenChange={(v) => { if (!v) setDeletingTemplate(null); }}>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete recurring template</AlertDialogTitle>
                <AlertDialogDescription>
                  Delete &ldquo;{deletingTemplate?.title}&rdquo;? This will stop future issues from being created automatically. This action cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  onClick={handleDelete}
                  className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                >
                  Delete
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </>
      )}
    </div>
  );
}
