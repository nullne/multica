"use client";

import { useState, useEffect, useLayoutEffect, useCallback, useRef, useMemo, memo } from "react";
import { useDefaultLayout, usePanelRef } from "react-resizable-panels";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Calendar,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  GitBranch,
  Link2,
  MoreHorizontal,
  PanelRight,
  Plus,
  Trash2,
  UserMinus,
  Users,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { Skeleton } from "@/components/ui/skeleton";
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
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
} from "@/components/ui/dropdown-menu";
import { ResizablePanelGroup, ResizablePanel, ResizableHandle } from "@/components/ui/resizable";
import { RichTextEditor } from "@/components/common/rich-text-editor";
import { FileUploadButton } from "@/components/common/file-upload-button";
import { TitleEditor } from "@/components/common/title-editor";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { Checkbox } from "@/components/ui/checkbox";
import { Command, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem } from "@/components/ui/command";
import { AvatarGroup, AvatarGroupCount } from "@/components/ui/avatar";
import { ActorAvatar } from "@/components/common/actor-avatar";
import type { Issue, UpdateIssueRequest, IssueStatus, IssuePriority, TimelineEntry } from "@/shared/types";
import { ALL_STATUSES, STATUS_CONFIG, PRIORITY_ORDER, PRIORITY_CONFIG } from "@/features/issues/config";
import { StatusIcon, PriorityIcon, DueDatePicker, AssigneePicker, VerifierPicker, canAssignAgent, PropertyPicker, PickerItem, DaemonPicker } from "@/features/issues/components";
import { CommentCard } from "./comment-card";
import { CommentInput } from "./comment-input";
import { AgentLiveCard, TaskRunHistory } from "./agent-live-card";
import { api } from "@/shared/api";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { useModalStore } from "@/features/modals";
import { useRuntimeStore } from "@/features/runtimes";
import { useIssueStore } from "@/features/issues";
import { LabelPicker } from "@/features/labels/components";
import { useIssueViewStore } from "@/features/issues/stores/view-store";
import { useIssuesScopeStore } from "@/features/issues/stores/issues-scope-store";
import { useIssueTimeline } from "@/features/issues/hooks/use-issue-timeline";
import { useIssueReactions } from "@/features/issues/hooks/use-issue-reactions";
import { useIssueSubscribers } from "@/features/issues/hooks/use-issue-subscribers";
import { filterIssues } from "@/features/issues/utils/filter";
import { sortIssues } from "@/features/issues/utils/sort";
import { issueUrl } from "@/features/issues/utils/url";
import { ReactionBar } from "@/components/common/reaction-bar";
import { useFileUpload } from "@/shared/hooks/use-file-upload";
import { useIsMobile } from "@/hooks/use-mobile";
import { timeAgo } from "@/shared/utils";

function CriteriaApprovalActions({ issueId, onUpdate }: { issueId: string; onUpdate: (issue: Issue) => void }) {
  const [rejecting, setRejecting] = useState(false);
  const [feedback, setFeedback] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleApprove = async () => {
    setSubmitting(true);
    try {
      const updated = await api.approveCriteria(issueId);
      onUpdate(updated);
      toast.success("Criteria approved — executor task enqueued");
    } catch {
      toast.error("Failed to approve criteria");
    } finally {
      setSubmitting(false);
    }
  };

  const handleReject = async () => {
    setSubmitting(true);
    try {
      const updated = await api.rejectCriteria(issueId, feedback);
      onUpdate(updated);
      setRejecting(false);
      setFeedback("");
      toast.success("Criteria rejected — verifier will regenerate");
    } catch {
      toast.error("Failed to reject criteria");
    } finally {
      setSubmitting(false);
    }
  };

  if (rejecting) {
    return (
      <div className="space-y-2 pt-1">
        <textarea
          value={feedback}
          onChange={(e) => setFeedback(e.target.value)}
          placeholder="What should be changed..."
          className="w-full rounded-md border px-2.5 py-1.5 text-xs bg-transparent outline-none focus:ring-1 focus:ring-ring resize-none"
          rows={3}
          autoFocus
        />
        <div className="flex items-center gap-1.5">
          <Button size="xs" variant="destructive" onClick={handleReject} disabled={submitting}>
            {submitting ? "Sending..." : "Request Changes"}
          </Button>
          <Button size="xs" variant="ghost" onClick={() => { setRejecting(false); setFeedback(""); }} disabled={submitting}>
            Cancel
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1.5 pt-1">
      <Button size="xs" onClick={handleApprove} disabled={submitting} className="bg-emerald-600 hover:bg-emerald-700 text-white">
        {submitting ? "Approving..." : "Approve"}
      </Button>
      <Button size="xs" variant="outline" onClick={() => setRejecting(true)} disabled={submitting}>
        Request Changes
      </Button>
    </div>
  );
}

function CriteriaSection({ issue, issueId, criteriaOpen, setCriteriaOpen }: {
  issue: Issue;
  issueId: string;
  criteriaOpen: boolean;
  setCriteriaOpen: (v: boolean) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(issue.acceptance_criteria);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setDraft(issue.acceptance_criteria);
  }, [issue.acceptance_criteria]);

  const updateField = (index: number, field: string, value: string) => {
    setDraft((prev) => prev.map((ac, i) => i === index ? { ...ac, [field]: value } : ac));
  };

  const removeCriterion = (index: number) => {
    setDraft((prev) => prev.filter((_, i) => i !== index));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const updated = await api.updateCriteria(issueId, draft);
      useIssueStore.getState().updateIssue(updated.id, updated);
      setEditing(false);
      toast.success("Criteria updated");
    } catch {
      toast.error("Failed to update criteria");
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setDraft(issue.acceptance_criteria);
    setEditing(false);
  };

  return (
    <div>
      <button
        className={`flex w-full items-center gap-1 text-xs font-medium transition-colors mb-2 ${criteriaOpen ? "" : "text-muted-foreground hover:text-foreground"}`}
        onClick={() => setCriteriaOpen(!criteriaOpen)}
      >
        <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform ${criteriaOpen ? "rotate-90" : ""}`} />
        Acceptance Criteria
        {issue.criteria_status === "approved" && (
          <span className="rounded px-1.5 py-0.5 text-[10px] font-medium bg-emerald-500/10 text-emerald-600">Approved</span>
        )}
        {issue.criteria_status === "pending" && (
          <span className="rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-500/10 text-amber-600">Pending</span>
        )}
        <span className="ml-auto text-muted-foreground font-normal">{issue.acceptance_criteria.length}</span>
      </button>

      {criteriaOpen && (
        <div className="space-y-2 pl-2">
          {(editing ? draft : issue.acceptance_criteria).map((ac, index) => {
            const severity = String(ac.severity ?? "");
            const check = String(ac.check ?? "");
            const title = String(ac.title ?? ac.description ?? "");

            if (editing) {
              return (
                <div key={ac.id ?? index} className="rounded-md border px-3 py-2 text-xs space-y-1.5 relative group">
                  <button
                    type="button"
                    onClick={() => removeCriterion(index)}
                    className="absolute top-1.5 right-1.5 p-0.5 rounded text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
                  >
                    <X className="h-3 w-3" />
                  </button>
                  <div className="flex items-center gap-1.5 mb-1">
                    <span className="font-mono text-muted-foreground text-[10px]">{ac.id}</span>
                    <button
                      type="button"
                      onClick={() => updateField(index, "severity", severity === "must" ? "should" : "must")}
                      className={`rounded px-1 py-0.5 text-[10px] font-medium cursor-pointer transition-colors ${severity === "must" ? "bg-destructive/10 text-destructive hover:bg-destructive/20" : "bg-muted text-muted-foreground hover:bg-muted/80"}`}
                    >
                      {severity || "should"}
                    </button>
                  </div>
                  <input
                    value={title}
                    onChange={(e) => updateField(index, ac.title !== undefined ? "title" : "description", e.target.value)}
                    className="w-full font-medium bg-transparent border-b border-dashed border-muted-foreground/30 outline-none focus:border-primary pb-0.5 pr-5"
                    placeholder="Title"
                  />
                  <textarea
                    value={check}
                    onChange={(e) => updateField(index, "check", e.target.value)}
                    className="w-full bg-transparent border-b border-dashed border-muted-foreground/30 outline-none focus:border-primary text-muted-foreground resize-none leading-relaxed"
                    placeholder="Check description"
                    rows={5}
                  />
                </div>
              );
            }

            return (
              <div
                key={ac.id}
                className="rounded-md border px-3 py-2 text-xs cursor-pointer hover:border-primary/30 transition-colors"
                onClick={() => { setEditing(true); setDraft(issue.acceptance_criteria); }}
              >
                <div className="flex items-center gap-1.5 mb-1">
                  <span className="font-mono text-muted-foreground">{ac.id}</span>
                  {severity && (
                    <span className={`rounded px-1 py-0.5 text-[10px] font-medium ${severity === "must" ? "bg-destructive/10 text-destructive" : "bg-muted text-muted-foreground"}`}>
                      {severity}
                    </span>
                  )}
                </div>
                <div className="font-medium mb-0.5">{title}</div>
                {check && (
                  <div className="text-muted-foreground leading-relaxed">{check}</div>
                )}
              </div>
            );
          })}

          {editing && (
            <div className="flex items-center gap-1.5 pt-1">
              <Button size="xs" onClick={handleSave} disabled={saving}>
                {saving ? "Saving..." : "Save"}
              </Button>
              <Button size="xs" variant="ghost" onClick={handleCancel} disabled={saving}>
                Cancel
              </Button>
            </div>
          )}

          {!editing && issue.criteria_status === "pending" && (
            <CriteriaApprovalActions issueId={issueId} onUpdate={(updated) => useIssueStore.getState().updateIssue(updated.id, updated)} />
          )}
        </div>
      )}
    </div>
  );
}

function shortDate(date: string | null): string {
  if (!date) return "—";
  return new Date(date).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
}

function statusLabel(status: string): string {
  return STATUS_CONFIG[status as IssueStatus]?.label ?? status;
}

function priorityLabel(priority: string): string {
  return PRIORITY_CONFIG[priority as IssuePriority]?.label ?? priority;
}

function humanizeActivityAction(action?: string): string {
  if (!action) return "updated this issue";
  return action.replace(/_/g, " ");
}

function formatActivity(
  entry: TimelineEntry,
  resolveActorName?: (type: string, id: string) => string,
): string {
  const details = (entry.details ?? {}) as Record<string, string>;
  switch (entry.action) {
    case "created":
      return "created this issue";
    case "status_changed":
      return `changed status from ${statusLabel(details.from ?? "?")} to ${statusLabel(details.to ?? "?")}`;
    case "priority_changed":
      return `changed priority from ${priorityLabel(details.from ?? "?")} to ${priorityLabel(details.to ?? "?")}`;
    case "assignee_changed": {
      const isSelfAssign = details.to_type === entry.actor_type && details.to_id === entry.actor_id;
      if (isSelfAssign) return "self-assigned this issue";
      const toName = details.to_id && details.to_type && resolveActorName
        ? resolveActorName(details.to_type, details.to_id)
        : null;
      if (toName) return `assigned to ${toName}`;
      if (details.from_id && !details.to_id) return "removed assignee";
      return "changed assignee";
    }
    case "due_date_changed": {
      if (!details.to) return "removed due date";
      const formatted = new Date(details.to).toLocaleDateString("en-US", { month: "short", day: "numeric" });
      return `set due date to ${formatted}`;
    }
    case "title_changed":
      return `renamed this issue from "${details.from ?? "?"}" to "${details.to ?? "?"}"`;
    case "description_updated":
      return "updated the description";
    case "criteria_approved":
      return "approved acceptance criteria";
    case "task_completed":
      return "completed the task";
    case "task_failed":
      return "task failed";
    default:
      return humanizeActivityAction(entry.action);
  }
}


// ---------------------------------------------------------------------------
// Parent issue picker
// ---------------------------------------------------------------------------

function ParentIssuePicker({
  currentParentId,
  currentIssueId,
  onUpdate,
}: {
  currentParentId: string | null;
  currentIssueId: string;
  onUpdate: (parentId: string | null) => void;
}) {
  const allIssues = useIssueStore((s) => s.issues);
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");

  const parentIssue = allIssues.find((i) => i.id === currentParentId);
  const candidates = allIssues.filter(
    (i) => i.id !== currentIssueId && i.parent_issue_id !== currentIssueId &&
      (filter === "" || i.title.toLowerCase().includes(filter.toLowerCase()) || i.identifier.toLowerCase().includes(filter.toLowerCase())),
  );

  return (
    <Popover open={open} onOpenChange={(v) => { setOpen(v); if (!v) setFilter(""); }}>
      <PopoverTrigger className="flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors overflow-hidden min-w-0 max-w-full">
        {parentIssue ? (
          <>
            <StatusIcon status={parentIssue.status} className="h-3 w-3 shrink-0" />
            <span className="truncate text-xs">{parentIssue.identifier} {parentIssue.title}</span>
          </>
        ) : (
          <span className="text-muted-foreground text-xs">None</span>
        )}
      </PopoverTrigger>
      <PopoverContent align="start" className="w-64 p-0">
        <div className="px-2 py-1.5 border-b">
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Search issues..."
            className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none"
            autoFocus
          />
        </div>
        <div className="p-1 max-h-60 overflow-y-auto">
          {currentParentId && (
            <button
              type="button"
              onClick={() => { onUpdate(null); setOpen(false); }}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors text-muted-foreground"
            >
              <X className="h-3.5 w-3.5" />
              Remove parent
            </button>
          )}
          {candidates.map((issue) => (
            <button
              key={issue.id}
              type="button"
              onClick={() => { onUpdate(issue.id); setOpen(false); }}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
            >
              <StatusIcon status={issue.status} className="h-3.5 w-3.5 shrink-0" />
              <span className="text-muted-foreground shrink-0">{issue.identifier}</span>
              <span className="truncate">{issue.title}</span>
            </button>
          ))}
          {candidates.length === 0 && (
            <div className="px-2 py-3 text-center text-sm text-muted-foreground">No issues found</div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

// ---------------------------------------------------------------------------
// Sub-issues section
// ---------------------------------------------------------------------------

function SubIssuesSection({
  issueId,
  onCreateSubIssue,
}: {
  issueId: string;
  onCreateSubIssue: () => void;
}) {
  const [open, setOpen] = useState(true);
  const [subIssues, setSubIssues] = useState<Issue[]>([]);
  const [loading, setLoading] = useState(true);
  const router = useRouter();
  const workspaceSlug = useWorkspaceStore((s) => s.workspace?.slug ?? "");

  // Also subscribe to store changes so newly created sub-issues appear
  const storeIssues = useIssueStore((s) => s.issues);
  const storeSubIssues = storeIssues.filter((i) => i.parent_issue_id === issueId);

  useEffect(() => {
    setLoading(true);
    api.getSubIssues(issueId)
      .then((res) => {
        setSubIssues(res.issues);
        // Upsert into global store so they're available for navigation
        const store = useIssueStore.getState();
        res.issues.forEach((issue) => {
          if (!store.issues.find((i) => i.id === issue.id)) {
            store.addIssue(issue);
          }
        });
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [issueId]);

  // Merge store sub-issues with fetched sub-issues (store may have more up-to-date data)
  const merged = [...subIssues];
  for (const si of storeSubIssues) {
    if (!merged.find((i) => i.id === si.id)) merged.push(si);
  }
  // Reflect store updates
  const display = merged.map((si) => storeIssues.find((i) => i.id === si.id) ?? si);

  return (
    <div>
      <button
        className={`flex w-full items-center gap-1 text-xs font-medium transition-colors mb-2 ${open ? "" : "text-muted-foreground hover:text-foreground"}`}
        onClick={() => setOpen(!open)}
      >
        <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform ${open ? "rotate-90" : ""}`} />
        Sub-issues
        <span className="ml-auto text-muted-foreground font-normal">{loading ? "…" : display.length}</span>
      </button>

      {open && (
        <div className="space-y-0.5 pl-2">
          {display.map((sub) => (
            <button
              key={sub.id}
              type="button"
              onClick={() => router.push(issueUrl(sub.id, workspaceSlug))}
              className="flex w-full items-center gap-1.5 rounded-md px-1 py-1 text-xs hover:bg-accent transition-colors text-left"
            >
              <StatusIcon status={sub.status} className="h-3 w-3 shrink-0" />
              <span className="text-muted-foreground shrink-0">{sub.identifier}</span>
              <span className="truncate">{sub.title}</span>
            </button>
          ))}
          <button
            type="button"
            onClick={onCreateSubIssue}
            className="flex w-full items-center gap-1.5 rounded-md px-1 py-1 text-xs text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
          >
            <Plus className="h-3 w-3 shrink-0" />
            Add sub-issue
          </button>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Property row
// ---------------------------------------------------------------------------

function PropRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-8 items-center gap-2 rounded-md px-2 -mx-2 hover:bg-accent/50 transition-colors">
      <span className="w-16 shrink-0 text-xs text-muted-foreground">{label}</span>
      <div className="flex min-w-0 flex-1 items-center gap-1.5 text-xs truncate">
        {children}
      </div>
    </div>
  );
}

function DispatchHints({ issue, onUpdate }: { issue: Issue; onUpdate: (updates: Partial<UpdateIssueRequest>) => void }) {
  const agents = useWorkspaceStore((s) => s.agents);
  const agent = agents.find((a) => a.id === issue.assignee_id);
  const providers = agent?.providers ?? [];

  const [providerOpen, setProviderOpen] = useState(false);

  return (
    <>
      <PropRow label="Provider">
        <PropertyPicker
          open={providerOpen}
          onOpenChange={setProviderOpen}
          width="w-40"
          align="start"
          trigger={
            <span className={issue.dispatch_provider ? "capitalize" : "text-muted-foreground"}>
              {issue.dispatch_provider ?? "Auto"}
            </span>
          }
        >
          <PickerItem
            selected={!issue.dispatch_provider}
            onClick={() => { onUpdate({ dispatch_provider: null }); setProviderOpen(false); }}
          >
            <span className="text-muted-foreground">Auto</span>
          </PickerItem>
          {providers.map((p) => (
            <PickerItem
              key={p}
              selected={issue.dispatch_provider === p}
              onClick={() => { onUpdate({ dispatch_provider: p }); setProviderOpen(false); }}
            >
              <span className="capitalize">{p}</span>
            </PickerItem>
          ))}
        </PropertyPicker>
      </PropRow>
      <PropRow label="Environment">
        <DaemonPicker
          daemonId={issue.dispatch_daemon_id}
          provider={issue.dispatch_provider}
          onUpdate={onUpdate}
          align="start"
        />
      </PropRow>
    </>
  );
}


// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface IssueDetailProps {
  issueId: string;
  onDelete?: () => void;
  /** Override mobile back button behavior. When omitted, navigates to /issues. */
  onBack?: () => void;
  defaultSidebarOpen?: boolean;
  layoutId?: string;
  /** When set, the issue detail will auto-scroll to this comment and briefly highlight it. */
  highlightCommentId?: string;
}

// ---------------------------------------------------------------------------
// IssueDetail
// ---------------------------------------------------------------------------

export function IssueDetail({ issueId, onDelete, onBack, defaultSidebarOpen = true, layoutId = "multica_issue_detail_layout", highlightCommentId }: IssueDetailProps) {
  const id = issueId;
  const isMobile = useIsMobile();
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);
  const fetchAllRuntimes = useRuntimeStore((s) => s.fetchAll);
  const currentMemberRole = members.find((m) => m.user_id === user?.id)?.role;

  useEffect(() => {
    if (workspace) fetchAllRuntimes();
  }, [workspace, fetchAllRuntimes]);

  // Single source of truth: read issue directly from global store
  const issue = useIssueStore((s) => s.issues.find((i) => i.id === id)) ?? null;
  const allIssues = useIssueStore((s) => s.issues);
  const scope = useIssuesScopeStore((s) => s.scope);
  const statusFilters = useIssueViewStore((s) => s.statusFilters);
  const priorityFilters = useIssueViewStore((s) => s.priorityFilters);
  const assigneeFilters = useIssueViewStore((s) => s.assigneeFilters);
  const includeNoAssignee = useIssueViewStore((s) => s.includeNoAssignee);
  const creatorFilters = useIssueViewStore((s) => s.creatorFilters);
  const labelFilters = useIssueViewStore((s) => s.labelFilters ?? []);
  const sortBy = useIssueViewStore((s) => s.sortBy);
  const sortDirection = useIssueViewStore((s) => s.sortDirection);
  const scopedIssues = useMemo(() => {
    if (scope === "members") return allIssues.filter((i) => i.assignee_type === "member");
    if (scope === "agents") return allIssues.filter((i) => i.assignee_type === "agent");
    return allIssues;
  }, [allIssues, scope]);
  const filteredIssues = useMemo(
    () => filterIssues(scopedIssues, { statusFilters, priorityFilters, assigneeFilters, includeNoAssignee, creatorFilters, labelFilters }),
    [scopedIssues, statusFilters, priorityFilters, assigneeFilters, includeNoAssignee, creatorFilters, labelFilters],
  );
  const navigationIssues = useMemo(() => {
    if (!issue) return [];
    const hasCurrentIssue = filteredIssues.some((i) => i.id === id);
    const baseIssues = hasCurrentIssue ? filteredIssues : allIssues;
    const sameStatusIssues = baseIssues.filter((i) => i.status === issue.status);
    return sortIssues(sameStatusIssues, sortBy, sortDirection);
  }, [allIssues, filteredIssues, id, issue, sortBy, sortDirection]);
  const currentIndex = navigationIssues.findIndex((i) => i.id === id);
  const prevIssue = currentIndex > 0 ? navigationIssues[currentIndex - 1] : null;
  const nextIssue = currentIndex < navigationIssues.length - 1 ? navigationIssues[currentIndex + 1] : null;
  const { getActorName } = useActorName();
  const { uploadWithToast } = useFileUpload();
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: layoutId,
  });
  const sidebarRef = usePanelRef();
  const [sidebarOpen, setSidebarOpen] = useState(defaultSidebarOpen);
  const [deleting, setDeleting] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [propertiesOpen, setPropertiesOpen] = useState(true);
  const [detailsOpen, setDetailsOpen] = useState(true);
  const [criteriaOpen, setCriteriaOpen] = useState(true);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const [showScrollBottom, setShowScrollBottom] = useState(false);
  const [highlightedId, setHighlightedId] = useState<string | null>(null);

  const [issueLoading, setIssueLoading] = useState(!issue);

  // If issue isn't in the store yet, fetch and upsert it
  useEffect(() => {
    if (issue) {
      setIssueLoading(false);
      return;
    }
    setIssueLoading(true);
    api
      .getIssue(id)
      .then((iss) => {
        useIssueStore.getState().addIssue(iss);
      })
      .catch((e) => {
        console.error(e);
        toast.error("Failed to load issue");
      })
      .finally(() => setIssueLoading(false));
  }, [id, !!issue]);

  // Custom hooks — encapsulate timeline, reactions, subscribers
  const {
    timeline, loading: timelineLoading, submitting, submitComment, submitReply,
    editComment, deleteComment, toggleReaction: handleToggleReaction,
  } = useIssueTimeline(id, user?.id);

  const {
    reactions: issueReactions, loading: reactionsLoading,
    toggleReaction: handleToggleIssueReaction,
  } = useIssueReactions(id, user?.id);

  const {
    subscribers, loading: subscribersLoading, isSubscribed, toggleSubscribe: handleToggleSubscribe, toggleSubscriber,
  } = useIssueSubscribers(id, user?.id);

  const loading = issueLoading;

  // Reset internal scroll to top whenever the displayed issue changes.
  // The scroll container belongs to IssueDetail itself; when this component
  // is reused (or React's natural mount/remount doesn't reset the inner
  // scrollTop reliably — e.g. when nested inside another resizable panel
  // group like the Inbox), we must explicitly snap back to the top so the
  // user always lands on the issue title rather than a stale scroll
  // position. The highlight-comment effect below runs after timeline data
  // loads and will smoothly scroll past this reset when applicable.
  useLayoutEffect(() => {
    const container = scrollContainerRef.current;
    if (container) container.scrollTop = 0;
  }, [id]);

  // Scroll to highlighted comment once timeline loads
  useEffect(() => {
    if (!highlightCommentId || timeline.length === 0) return;
    // Find the comment element — could be a top-level comment or a reply
    const el = document.getElementById(`comment-${highlightCommentId}`);
    if (el) {
      // Small delay to ensure layout is settled
      requestAnimationFrame(() => {
        el.scrollIntoView({ behavior: "smooth", block: "center" });
        setHighlightedId(highlightCommentId);
        // Clear highlight after animation
        const timer = setTimeout(() => setHighlightedId(null), 2000);
        return () => clearTimeout(timer);
      });
    }
  }, [highlightCommentId, timeline.length]);

  // Track scroll position for jump-to-bottom button
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) return;
    const onScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = container;
      setShowScrollBottom(scrollHeight - scrollTop - clientHeight > 200);
    };
    container.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
    return () => container.removeEventListener("scroll", onScroll);
  }, []);

  const scrollToBottom = useCallback(() => {
    scrollContainerRef.current?.scrollTo({ top: scrollContainerRef.current.scrollHeight, behavior: "smooth" });
  }, []);

  // Issue field updates — write directly to the global store (single source of truth)
  const handleUpdateField = useCallback(
    (updates: Partial<UpdateIssueRequest>) => {
      if (!issue) return;
      const prev = { ...issue };
      useIssueStore.getState().updateIssue(id, updates);
      api.updateIssue(id, updates).catch(() => {
        useIssueStore.getState().updateIssue(id, prev);
        toast.error("Failed to update issue");
      });
    },
    [issue, id],
  );

  const descEditorRef = useRef<import("@/components/common/rich-text-editor").RichTextEditorRef>(null);
  const handleDescriptionUpload = useCallback(
    (file: File) => uploadWithToast(file, { issueId: id }),
    [uploadWithToast, id],
  );

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await api.deleteIssue(issue!.id);
      useIssueStore.getState().removeIssue(issue!.id);
      toast.success("Issue deleted");
      if (onDelete) onDelete();
      else router.push("/issues");
    } catch {
      toast.error("Failed to delete issue");
      setDeleting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        {/* Header skeleton */}
        <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-4 w-4" />
          <Skeleton className="h-4 w-24" />
        </div>
        <div className="flex flex-1 min-h-0">
          {/* Content skeleton */}
          <div className="flex-1 p-8 space-y-6">
            <Skeleton className="h-8 w-3/4" />
            <div className="space-y-2">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-5/6" />
              <Skeleton className="h-4 w-2/3" />
            </div>
            <Skeleton className="h-px w-full" />
            <div className="space-y-3">
              <Skeleton className="h-4 w-20" />
              <div className="flex items-start gap-3">
                <Skeleton className="h-8 w-8 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-16 w-full rounded-lg" />
                </div>
              </div>
            </div>
          </div>
          {/* Sidebar skeleton */}
          <div className="w-64 border-l p-4 space-y-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="flex items-center justify-between">
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-5 w-24" />
              </div>
            ))}
            <Skeleton className="h-px w-full" />
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="flex items-center justify-between">
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-4 w-28" />
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (!issue) {
    return (
      <div className="flex flex-1 min-h-0 flex-col items-center justify-center gap-3 text-sm text-muted-foreground">
        <p>This issue does not exist or has been deleted in this workspace.</p>
        {!onDelete && (
          <Button variant="outline" size="sm" onClick={() => router.push("/issues")}>
            <ChevronLeft className="mr-1 h-3.5 w-3.5" />
            Back to Issues
          </Button>
        )}
      </div>
    );
  }

  return (
    <ResizablePanelGroup orientation="horizontal" className="flex-1 min-h-0" defaultLayout={defaultLayout} onLayoutChanged={onLayoutChanged}>
      <ResizablePanel id="content" minSize={isMobile ? "100%" : "50%"}>
      {/* LEFT: Content area */}
      <div className="flex h-full flex-col">
        {/* Header bar */}
        <div className="flex h-12 shrink-0 items-center justify-between border-b bg-background px-4 text-sm">
          <div className="flex items-center gap-1.5 min-w-0">
            {isMobile ? (
              <>
                {onBack ? (
                  <button onClick={onBack} className="text-muted-foreground hover:text-foreground transition-colors shrink-0">
                    <ChevronLeft className="h-4 w-4" />
                  </button>
                ) : (
                  <Link href="/issues" className="text-muted-foreground hover:text-foreground transition-colors shrink-0">
                    <ChevronLeft className="h-4 w-4" />
                  </Link>
                )}
                <span className="truncate text-muted-foreground">{issue.identifier}</span>
              </>
            ) : (
              <>
                {workspace && (
                  <>
                    <Link
                      href="/issues"
                      className="text-muted-foreground hover:text-foreground transition-colors truncate shrink-0"
                    >
                      {workspace.name}
                    </Link>
                    <ChevronRight className="h-3 w-3 text-muted-foreground/50 shrink-0" />
                  </>
                )}
                {issue.parent_issue_id && (() => {
                  const parent = allIssues.find((i) => i.id === issue.parent_issue_id);
                  return parent ? (
                    <>
                      <Link
                        href={issueUrl(parent.id, workspace?.slug ?? "")}
                        className="text-muted-foreground hover:text-foreground transition-colors shrink-0"
                      >
                        {parent.identifier}
                      </Link>
                      <ChevronRight className="h-3 w-3 text-muted-foreground/50 shrink-0" />
                    </>
                  ) : null;
                })()}
                <span className="truncate text-muted-foreground">
                  {issue.identifier}
                </span>
                <ChevronRight className="h-3 w-3 text-muted-foreground/50 shrink-0" />
                <span className="truncate">{issue.title}</span>
              </>
            )}
          </div>
          <div className="flex items-center gap-1 shrink-0">
            {/* Issue navigation — hidden on mobile */}
            {!isMobile && navigationIssues.length > 1 && (
              <div className="flex items-center gap-0.5 mr-1">
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        className="text-muted-foreground"
                        disabled={!prevIssue}
                        onClick={() => prevIssue && router.push(issueUrl(prevIssue.id, workspace?.slug ?? ""))}
                      >
                        <ChevronLeft className="h-4 w-4" />
                      </Button>
                    }
                  />
                  <TooltipContent side="bottom">Previous issue</TooltipContent>
                </Tooltip>
                <span className="text-xs text-muted-foreground tabular-nums px-0.5">
                  {currentIndex >= 0 ? currentIndex + 1 : "?"} / {navigationIssues.length}
                </span>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        className="text-muted-foreground"
                        disabled={!nextIssue}
                        onClick={() => nextIssue && router.push(issueUrl(nextIssue.id, workspace?.slug ?? ""))}
                      >
                        <ChevronRight className="h-4 w-4" />
                      </Button>
                    }
                  />
                  <TooltipContent side="bottom">Next issue</TooltipContent>
                </Tooltip>
              </div>
            )}
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button variant="ghost" size="icon-xs" className="text-muted-foreground">
                    <MoreHorizontal className="h-4 w-4" />
                  </Button>
                }
              />
              <DropdownMenuContent align="end" className="w-auto">
                {/* Status */}
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger>
                    <StatusIcon status={issue.status} className="h-3.5 w-3.5" />
                    Status
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>
                    {ALL_STATUSES.map((s) => (
                      <DropdownMenuItem
                        key={s}
                        onClick={() => handleUpdateField({ status: s })}
                      >
                        <StatusIcon status={s} className="h-3.5 w-3.5" />
                        {STATUS_CONFIG[s].label}
                        {issue.status === s && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>

                {/* Priority */}
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger>
                    <PriorityIcon priority={issue.priority} />
                    Priority
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>
                    {PRIORITY_ORDER.map((p) => (
                      <DropdownMenuItem
                        key={p}
                        onClick={() => handleUpdateField({ priority: p })}
                      >
                        <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${PRIORITY_CONFIG[p].badgeBg} ${PRIORITY_CONFIG[p].badgeText}`}>
                          <PriorityIcon priority={p} className="h-3 w-3" inheritColor />
                          {PRIORITY_CONFIG[p].label}
                        </span>
                        {issue.priority === p && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>

                {/* Assignee */}
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger>
                    <UserMinus className="h-3.5 w-3.5" />
                    Assignee
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>
                    <DropdownMenuItem
                      onClick={() => handleUpdateField({ assignee_type: null, assignee_id: null })}
                    >
                      <UserMinus className="h-3.5 w-3.5 text-muted-foreground" />
                      Unassigned
                      {!issue.assignee_type && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                    </DropdownMenuItem>
                    {members.filter((m) => m.kind !== "bot").map((m) => (
                      <DropdownMenuItem
                        key={m.user_id}
                        onClick={() => handleUpdateField({ assignee_type: "member", assignee_id: m.user_id })}
                      >
                        <ActorAvatar actorType="member" actorId={m.user_id} size={16} />
                        {m.name}
                        {issue.assignee_type === "member" && issue.assignee_id === m.user_id && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                      </DropdownMenuItem>
                    ))}
                    {agents.filter((a) => canAssignAgent(a, user?.id, currentMemberRole)).map((a) => (
                      <DropdownMenuItem
                        key={a.id}
                        onClick={() => handleUpdateField({ assignee_type: "agent", assignee_id: a.id })}
                      >
                        <ActorAvatar actorType="agent" actorId={a.id} size={16} />
                        {a.name}
                        {issue.assignee_type === "agent" && issue.assignee_id === a.id && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>

                {/* Due date */}
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger>
                    <Calendar className="h-3.5 w-3.5" />
                    Due date
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>
                    <DropdownMenuItem onClick={() => handleUpdateField({ due_date: new Date().toISOString() })}>
                      Today
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => {
                      const d = new Date(); d.setDate(d.getDate() + 1);
                      handleUpdateField({ due_date: d.toISOString() });
                    }}>
                      Tomorrow
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => {
                      const d = new Date(); d.setDate(d.getDate() + 7);
                      handleUpdateField({ due_date: d.toISOString() });
                    }}>
                      Next week
                    </DropdownMenuItem>
                    {issue.due_date && (
                      <>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem onClick={() => handleUpdateField({ due_date: null })}>
                          Clear date
                        </DropdownMenuItem>
                      </>
                    )}
                  </DropdownMenuSubContent>
                </DropdownMenuSub>

                <DropdownMenuSeparator />

                {/* Copy link */}
                <DropdownMenuItem onClick={() => {
                  navigator.clipboard.writeText(window.location.href);
                  toast.success("Link copied");
                }}>
                  <Link2 className="h-3.5 w-3.5" />
                  Copy link
                </DropdownMenuItem>

                <DropdownMenuSeparator />

                {/* Delete */}
                <DropdownMenuItem
                  variant="destructive"
                  onClick={() => setDeleteDialogOpen(true)}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  Delete issue
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            {!isMobile && (
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant={sidebarOpen ? "secondary" : "ghost"}
                      size="icon-xs"
                      className={sidebarOpen ? "" : "text-muted-foreground"}
                      onClick={() => {
                        const panel = sidebarRef.current;
                        if (!panel) return;
                        if (panel.isCollapsed()) panel.expand();
                        else panel.collapse();
                      }}
                    >
                      <PanelRight className="h-4 w-4" />
                    </Button>
                  }
                />
                <TooltipContent side="bottom">Toggle sidebar</TooltipContent>
              </Tooltip>
            )}
          </div>

            {/* Delete confirmation dialog (controlled by state) */}
            <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete issue</AlertDialogTitle>
                  <AlertDialogDescription>
                    This will permanently delete this issue and all its comments. This action cannot be undone.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={handleDelete}
                    disabled={deleting}
                    className="bg-destructive text-white hover:bg-destructive/90"
                  >
                    {deleting ? "Deleting..." : "Delete"}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>

        {/* Content — scrollable */}
        <div ref={scrollContainerRef} className="relative flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-4xl px-4 py-4 md:px-8 md:py-8">
          <TitleEditor
            key={`title-${id}`}
            defaultValue={issue.title}
            placeholder="Issue title"
            className="w-full text-xl md:text-2xl font-bold leading-snug tracking-tight"
            onBlur={(value) => {
              const trimmed = value.trim();
              if (trimmed && trimmed !== issue.title) handleUpdateField({ title: trimmed });
            }}
          />

          <RichTextEditor
            ref={descEditorRef}
            key={id}
            defaultValue={issue.description || ""}
            placeholder="Add description..."
            onUpdate={(md) => handleUpdateField({ description: md || undefined })}
            onUploadFile={handleDescriptionUpload}
            debounceMs={1500}
            className="mt-5"
          />

          <div className="flex items-center gap-1 mt-3">
            {reactionsLoading ? (
              <div className="flex items-center gap-1">
                <Skeleton className="h-7 w-14 rounded-full" />
                <Skeleton className="h-7 w-14 rounded-full" />
              </div>
            ) : (
              <ReactionBar
                reactions={issueReactions}
                currentUserId={user?.id}
                onToggle={handleToggleIssueReaction}
              />
            )}
            <FileUploadButton
              size="sm"
              onSelect={(file) => descEditorRef.current?.uploadFile(file)}
            />
          </div>

          <div className="my-8 border-t" />

          {/* Activity / Comments */}
          <div>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <h2 className="text-base font-semibold">Activity</h2>
              </div>
              <div className="flex items-center gap-2">
                {subscribersLoading ? (
                  <div className="flex items-center gap-1">
                    <Skeleton className="h-4 w-16" />
                    <div className="flex -space-x-1">
                      <Skeleton className="h-6 w-6 rounded-full" />
                      <Skeleton className="h-6 w-6 rounded-full" />
                    </div>
                  </div>
                ) : (<>
                <button
                  onClick={handleToggleSubscribe}
                  className="text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  {isSubscribed ? "Unsubscribe" : "Subscribe"}
                </button>
                <Popover>
                  <PopoverTrigger className="cursor-pointer hover:opacity-80 transition-opacity">
                    {subscribers.length > 0 ? (
                      <AvatarGroup>
                        {subscribers.slice(0, 4).map((sub) => (
                          <ActorAvatar
                            key={`${sub.user_type}-${sub.user_id}`}
                            actorType={sub.user_type}
                            actorId={sub.user_id}
                            size={24}
                          />
                        ))}
                        {subscribers.length > 4 && (
                          <AvatarGroupCount>+{subscribers.length - 4}</AvatarGroupCount>
                        )}
                      </AvatarGroup>
                    ) : (
                      <span className="flex items-center justify-center h-6 w-6 rounded-full border border-dashed border-muted-foreground/30 text-muted-foreground">
                        <Users className="h-3 w-3" />
                      </span>
                    )}
                  </PopoverTrigger>
                  <PopoverContent align="end" className="w-64 p-0">
                    <Command>
                      <CommandInput placeholder="Change subscribers..." />
                      <CommandList className="max-h-64">
                        <CommandEmpty>No results found</CommandEmpty>
                        {members.length > 0 && (
                          <CommandGroup heading="Members">
                            {members.filter((m, i, arr) => arr.findIndex((x) => x.user_id === m.user_id) === i).map((m) => {
                              const sub = subscribers.find((s) => s.user_type === "member" && s.user_id === m.user_id);
                              const isSubbed = !!sub;
                              return (
                                <CommandItem
                                  key={`member-${m.user_id}`}
                                  onSelect={() => toggleSubscriber(m.user_id, "member", isSubbed)}
                                  className="flex items-center gap-2.5"
                                >
                                  <Checkbox checked={isSubbed} className="pointer-events-none" />
                                  <ActorAvatar actorType="member" actorId={m.user_id} size={22} />
                                  <span className="truncate flex-1">{m.name}</span>

                                </CommandItem>
                              );
                            })}
                          </CommandGroup>
                        )}
                        {agents.length > 0 && (
                          <CommandGroup heading="Agents">
                            {agents.map((a) => {
                              const sub = subscribers.find((s) => s.user_type === "agent" && s.user_id === a.id);
                              const isSubbed = !!sub;
                              return (
                                <CommandItem
                                  key={`agent-${a.id}`}
                                  onSelect={() => toggleSubscriber(a.id, "agent", isSubbed)}
                                  className="flex items-center gap-2.5"
                                >
                                  <Checkbox checked={isSubbed} className="pointer-events-none" />
                                  <ActorAvatar actorType="agent" actorId={a.id} size={22} />
                                  <span className="truncate flex-1">{a.name}</span>

                                </CommandItem>
                              );
                            })}
                          </CommandGroup>
                        )}
                      </CommandList>
                    </Command>
                  </PopoverContent>
                </Popover>
                </>)}
              </div>
            </div>

            {/* Agent live output */}
            <div className="mt-4">
              <AgentLiveCard
                issueId={id}
                agentName={issue.assignee_type === "agent" && issue.assignee_id ? getActorName("agent", issue.assignee_id) : undefined}
              />
            </div>

            {/* Agent execution history */}
            <div className="mt-3">
              <TaskRunHistory issueId={id} />
            </div>

            {/* Timeline entries */}
            <div className="mt-4 flex flex-col gap-3">
              {timelineLoading ? (
                <div className="space-y-4">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <div key={i} className="flex items-start gap-3 px-4">
                      <Skeleton className="h-8 w-8 rounded-full shrink-0" />
                      <div className="flex-1 space-y-2">
                        <Skeleton className="h-4 w-32" />
                        <Skeleton className="h-16 w-full rounded-lg" />
                      </div>
                    </div>
                  ))}
                </div>
              ) : (() => {
                const topLevel = timeline.filter((e) => e.type === "activity" || !e.parent_id);
                const repliesByParent = new Map<string, TimelineEntry[]>();
                for (const e of timeline) {
                  if (e.type === "comment" && e.parent_id) {
                    const list = repliesByParent.get(e.parent_id) ?? [];
                    list.push(e);
                    repliesByParent.set(e.parent_id, list);
                  }
                }

                // Coalesce: same actor + same action within 2 min → keep last only
                const COALESCE_MS = 2 * 60 * 1000;
                const coalesced: TimelineEntry[] = [];
                for (const entry of topLevel) {
                  if (entry.type === "activity") {
                    const prev = coalesced[coalesced.length - 1];
                    if (
                      prev?.type === "activity" &&
                      prev.action === entry.action &&
                      prev.actor_type === entry.actor_type &&
                      prev.actor_id === entry.actor_id &&
                      Math.abs(new Date(entry.created_at).getTime() - new Date(prev.created_at).getTime()) <= COALESCE_MS
                    ) {
                      // Replace previous with this one (keep the later result)
                      coalesced[coalesced.length - 1] = entry;
                      continue;
                    }
                  }
                  coalesced.push(entry);
                }

                // Group consecutive activities together so the connector line works
                const groups: { type: "activities" | "comment"; entries: TimelineEntry[] }[] = [];
                for (const entry of coalesced) {
                  if (entry.type === "activity") {
                    const last = groups[groups.length - 1];
                    if (last?.type === "activities") {
                      last.entries.push(entry);
                    } else {
                      groups.push({ type: "activities", entries: [entry] });
                    }
                  } else {
                    groups.push({ type: "comment", entries: [entry] });
                  }
                }

                return groups.map((group) => {
                  if (group.type === "comment") {
                    const entry = group.entries[0]!;
                    return (
                      <div key={entry.id} id={`comment-${entry.id}`}>
                        <CommentCard
                          issueId={id}
                          entry={entry}
                          allReplies={repliesByParent}
                          currentUserId={user?.id}
                          onReply={submitReply}
                          onEdit={editComment}
                          onDelete={deleteComment}
                          onToggleReaction={handleToggleReaction}
                          highlightedCommentId={highlightedId}
                        />
                      </div>
                    );
                  }

                  return (
                    <div key={group.entries[0]!.id} className="px-4 flex flex-col gap-3">
                      {group.entries.map((entry, idx) => {
                        const details = (entry.details ?? {}) as Record<string, string>;
                        const isStatusChange = entry.action === "status_changed";
                        const isPriorityChange = entry.action === "priority_changed";
                        const isDueDateChange = entry.action === "due_date_changed";

                        let leadIcon: React.ReactNode;
                        if (isStatusChange && details.to) {
                          leadIcon = <StatusIcon status={details.to as IssueStatus} className="h-4 w-4 shrink-0" />;
                        } else if (isPriorityChange && details.to) {
                          leadIcon = <PriorityIcon priority={details.to as IssuePriority} className="h-4 w-4 shrink-0" />;
                        } else if (isDueDateChange) {
                          leadIcon = <Calendar className="h-4 w-4 shrink-0 text-muted-foreground" />;
                        } else {
                          leadIcon = <ActorAvatar actorType={entry.actor_type} actorId={entry.actor_id} size={16} />;
                        }

                        return (
                          <div key={entry.id} className="flex items-center text-xs text-muted-foreground">
                            <div className="mr-2 flex w-4 shrink-0 justify-center">
                              {leadIcon}
                            </div>
                            <div className="flex min-w-0 flex-1 items-center gap-1">
                              <span className="shrink-0 font-medium">{getActorName(entry.actor_type, entry.actor_id)}</span>
                              <span className="truncate">{formatActivity(entry, getActorName)}</span>
                              <Tooltip>
                                <TooltipTrigger
                                  render={
                                    <span className="ml-auto shrink-0 cursor-default">
                                      {timeAgo(entry.created_at)}
                                    </span>
                                  }
                                />
                                <TooltipContent side="top">
                                  {new Date(entry.created_at).toLocaleString()}
                                </TooltipContent>
                              </Tooltip>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  );
                });
              })()}
            </div>

            {/* Bottom comment input — no avatar, full width */}
            <div className="mt-4">
              <CommentInput issueId={id} onSubmit={submitComment} />
            </div>
          </div>
        </div>
        {/* Jump to bottom button */}
        {showScrollBottom && (
          <div className="sticky bottom-4 flex justify-center pointer-events-none">
            <Button
              variant="secondary"
              size="sm"
              className="pointer-events-auto shadow-md"
              onClick={scrollToBottom}
            >
              <ChevronDown className="mr-1 h-3.5 w-3.5" />
              Jump to bottom
            </Button>
          </div>
        )}
        </div>
      </div>
      </ResizablePanel>
      {!isMobile && (
        <>
          <ResizableHandle />
          <ResizablePanel
            id="sidebar"
            defaultSize={defaultSidebarOpen ? 320 : 0}
            minSize={260}
            maxSize={420}
            collapsible
            groupResizeBehavior="preserve-pixel-size"
            panelRef={sidebarRef}
            onResize={(size) => setSidebarOpen(size.inPixels > 0)}
          >
          {/* RIGHT: Properties sidebar */}
          <div className="overflow-y-auto border-l h-full">
            <div className="p-4 space-y-5">
              {/* Properties section */}
              <div>
                <button
                  className={`flex w-full items-center gap-1 text-xs font-medium transition-colors mb-2 ${propertiesOpen ? "" : "text-muted-foreground hover:text-foreground"}`}
                  onClick={() => setPropertiesOpen(!propertiesOpen)}
                >
                  <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform ${propertiesOpen ? "rotate-90" : ""}`} />
                  Properties
                </button>

                {propertiesOpen && <div className="space-y-0.5 pl-2">
                  {/* Status */}
                  <PropRow label="Status">
                    <DropdownMenu>
                      <DropdownMenuTrigger className="flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors overflow-hidden">
                        <StatusIcon status={issue.status} className="h-3.5 w-3.5 shrink-0" />
                        <span className="truncate">{STATUS_CONFIG[issue.status].label}</span>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="start" className="w-44">
                        {ALL_STATUSES.map((s) => (
                          <DropdownMenuItem key={s} onClick={() => handleUpdateField({ status: s })}>
                            <StatusIcon status={s} className="h-3.5 w-3.5" />
                            {STATUS_CONFIG[s].label}
                            {s === issue.status && <Check className="ml-auto h-3.5 w-3.5" />}
                          </DropdownMenuItem>
                        ))}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </PropRow>

                  {/* Priority */}
                  <PropRow label="Priority">
                    <DropdownMenu>
                      <DropdownMenuTrigger className="flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors overflow-hidden">
                        <PriorityIcon priority={issue.priority} className="shrink-0" />
                        <span className="truncate">{PRIORITY_CONFIG[issue.priority].label}</span>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="start" className="w-44">
                        {PRIORITY_ORDER.map((p) => (
                          <DropdownMenuItem key={p} onClick={() => handleUpdateField({ priority: p })}>
                            <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${PRIORITY_CONFIG[p].badgeBg} ${PRIORITY_CONFIG[p].badgeText}`}>
                              <PriorityIcon priority={p} className="h-3 w-3" inheritColor />
                              {PRIORITY_CONFIG[p].label}
                            </span>
                            {p === issue.priority && <Check className="ml-auto h-3.5 w-3.5" />}
                          </DropdownMenuItem>
                        ))}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </PropRow>

                  {/* Assignee */}
                  <PropRow label="Assignee">
                    <AssigneePicker
                      assigneeType={issue.assignee_type}
                      assigneeId={issue.assignee_id}
                      onUpdate={handleUpdateField}
                      align="start"
                    />
                  </PropRow>

                  {/* Labels */}
                  <PropRow label="Labels">
                    <LabelPicker
                      issueId={issue.id}
                      selected={issue.labels ?? []}
                      onUpdate={(labels) =>
                        useIssueStore.getState().updateIssue(issue.id, { labels })
                      }
                      align="start"
                    />
                  </PropRow>

                  {/* Due date */}
                  <PropRow label="Due date">
                    <DueDatePicker
                      dueDate={issue.due_date}
                      onUpdate={handleUpdateField}
                    />
                  </PropRow>

                  {/* Parent issue */}
                  <PropRow label="Parent">
                    <ParentIssuePicker
                      currentParentId={issue.parent_issue_id}
                      currentIssueId={issue.id}
                      onUpdate={(parentId) => handleUpdateField({ parent_issue_id: parentId })}
                    />
                  </PropRow>

                  {/* Verifier — only when assignee is an agent */}
                  {issue.assignee_type === "agent" && issue.assignee_id && (
                    <PropRow label="Verifier">
                      <VerifierPicker
                        verifierAgentId={issue.verifier_agent_id}
                        assigneeId={issue.assignee_id}
                        onUpdate={handleUpdateField}
                        align="start"
                      />
                    </PropRow>
                  )}

                  {/* Max verification rounds — only when verifier is set */}
                  {issue.verifier_agent_id && (
                    <PropRow label="Max rounds">
                      <input
                        type="number"
                        min={1}
                        max={20}
                        value={issue.max_verification_rounds ?? ""}
                        placeholder="5"
                        onChange={(e) => {
                          const v = e.target.value ? parseInt(e.target.value, 10) : null;
                          handleUpdateField({ max_verification_rounds: v && v > 0 ? v : null });
                        }}
                        className="w-14 rounded border px-1.5 py-0.5 text-xs text-right bg-transparent outline-none focus:ring-1 focus:ring-ring"
                      />
                    </PropRow>
                  )}

                  {/* Dispatch hints — visible when assignee is an agent */}
                  {issue.assignee_type === "agent" && issue.assignee_id && (
                    <DispatchHints issue={issue} onUpdate={handleUpdateField} />
                  )}
                </div>}
              </div>

              {/* Details section */}
              <div>
                <button
                  className={`flex w-full items-center gap-1 text-xs font-medium transition-colors mb-2 ${detailsOpen ? "" : "text-muted-foreground hover:text-foreground"}`}
                  onClick={() => setDetailsOpen(!detailsOpen)}
                >
                  <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform ${detailsOpen ? "rotate-90" : ""}`} />
                  Details
                </button>

                {detailsOpen && <div className="space-y-0.5 pl-2">
                  <PropRow label="Created by">
                    <ActorAvatar
                      actorType={issue.creator_type}
                      actorId={issue.creator_id}
                      size={18}
                    />
                    <span className="truncate">{getActorName(issue.creator_type, issue.creator_id)}</span>
                  </PropRow>
                  <PropRow label="Created">
                    <span className="text-muted-foreground">{shortDate(issue.created_at)}</span>
                  </PropRow>
                  <PropRow label="Updated">
                    <span className="text-muted-foreground">{shortDate(issue.updated_at)}</span>
                  </PropRow>
                  {(() => {
                    const branch = issue.links?.find((l) => l.kind === "branch" && l.direction === "output");
                    const outgoingPr = issue.links?.find((l) => l.kind === "pr" && l.direction === "output");
                    const sourceLinks = issue.links?.filter((l) => l.direction === "source") ?? [];
                    return (
                      <>
                        <PropRow label="Branch">
                          {branch ? (
                            <code className="rounded bg-muted px-1 py-0.5 text-[11px]">{branch.external_id || branch.url.replace(/^branch:/, "")}</code>
                          ) : (
                            <span className="text-muted-foreground">—</span>
                          )}
                        </PropRow>
                        <PropRow label="Pull request">
                          {outgoingPr ? (
                            <a
                              href={outgoingPr.url}
                              target="_blank"
                              rel="noreferrer"
                              className="truncate text-primary hover:underline"
                            >
                              {outgoingPr.url}
                            </a>
                          ) : (
                            <span className="text-muted-foreground">—</span>
                          )}
                        </PropRow>
                        {sourceLinks.length > 0 && (
                          <PropRow label="Source">
                            <div className="flex flex-col gap-0.5">
                              {sourceLinks.map((l) => (
                                <a
                                  key={l.id}
                                  href={l.url}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="truncate text-primary hover:underline"
                                  title={l.url}
                                >
                                  {l.external_id || l.url}
                                </a>
                              ))}
                            </div>
                          </PropRow>
                        )}
                      </>
                    );
                  })()}
                </div>}
              </div>

              {/* Sub-issues section */}
              <SubIssuesSection
                issueId={id}
                onCreateSubIssue={() =>
                  useModalStore.getState().open("create-issue", { parent_issue_id: id })
                }
              />

              {/* Acceptance Criteria section */}
              {issue.acceptance_criteria?.length > 0 && (
                <CriteriaSection
                  issue={issue}
                  issueId={id}
                  criteriaOpen={criteriaOpen}
                  setCriteriaOpen={setCriteriaOpen}
                />
              )}

            </div>
          </div>
          </ResizablePanel>
        </>
      )}
    </ResizablePanelGroup>
  );
}
