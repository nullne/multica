"use client";

import { useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import {
  AlertTriangle,
  Bell,
  CalendarClock,
  Check,
  ChevronDown,
  Clock,
  Code2,
  Copy,
  GitBranch as Github,
  Plus,
  Save,
  Sparkles,
  Tag,
  Users,
  X,
} from "lucide-react";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { useAuthStore } from "@/features/auth";
import { PriorityPicker } from "@/features/issues/components/pickers/priority-picker";
import { AssigneePicker } from "@/features/issues/components/pickers";
import { useLabelStore } from "@/features/labels";
import { useActorName, useWorkspaceStore } from "@/features/workspace";
import { api, generateRoutineTriggerTokenDraft, regenerateRoutineTriggerToken } from "@/shared/api";
import type { CreateRoutineRequest, IssueAssigneeType, IssuePriority, Routine, RoutineTriggerRequest, UpdateIssueRequest } from "@/shared/types";
import { toast } from "sonner";

type TriggerType = "schedule" | "github" | "api";
type ScheduleMode = "once" | "hourly" | "daily" | "weekdays" | "weekly" | "custom";

interface ApiTriggerCredential {
  id: string;
  tokenPrefix: string;
  tokenDraftId?: string;
  token?: string;
}

interface ApiTokenReveal {
  token: string;
  url: string;
}

interface ScheduleTriggerConfig {
  mode: ScheduleMode;
  runAt: string;
  hourlyMinute: string;
  timeOfDay: string;
  weeklyDay: string;
  cronExpression: string;
}

interface RoutineTriggerDraft {
  clientId: string;
  id?: string;
  type: TriggerType;
  schedule: ScheduleTriggerConfig;
  githubEventValue: string;
  githubFilters: GitHubFilterCondition[];
  apiCredential: ApiTriggerCredential | null;
}

interface TriggerOption {
  id: TriggerType;
  label: string;
  description: string;
  icon: typeof CalendarClock;
}

const triggerOptions: TriggerOption[] = [
  {
    id: "schedule",
    label: "Schedule",
    description: "Run on a recurring cron schedule or once at a future time",
    icon: CalendarClock,
  },
  {
    id: "github",
    label: "GitHub event",
    description: "Run when a GitHub webhook event fires",
    icon: Github,
  },
  {
    id: "api",
    label: "API",
    description: "Trigger from your own code by sending a POST request",
    icon: Code2,
  },
];

const scheduleModes: { value: ScheduleMode; label: string }[] = [
  { value: "once", label: "Once" },
  { value: "hourly", label: "Hourly" },
  { value: "daily", label: "Daily" },
  { value: "weekdays", label: "Weekdays" },
  { value: "weekly", label: "Weekly" },
  { value: "custom", label: "Custom" },
];

type GitHubEventCategory = "pull_request" | "issues" | "release";

const githubEventGroups: {
  id: GitHubEventCategory;
  label: string;
  events: { value: string; label: string; summary: string }[];
}[] = [
  {
    id: "pull_request",
    label: "Pull request",
    events: [
      { value: "github.pull_request.assigned", label: "Assigned", summary: "PR assigned" },
      { value: "github.pull_request.auto_merge_disabled", label: "Auto merge disabled", summary: "PR auto merge disabled" },
      { value: "github.pull_request.auto_merge_enabled", label: "Auto merge enabled", summary: "PR auto merge enabled" },
      { value: "github.pull_request.closed", label: "Closed", summary: "Pull request closed" },
      { value: "github.pull_request.converted_to_draft", label: "Converted to draft", summary: "PR converted to draft" },
      { value: "github.pull_request.demilestoned", label: "Demilestoned", summary: "PR demilestoned" },
      { value: "github.pull_request.dequeued", label: "Dequeued", summary: "PR dequeued" },
      { value: "github.pull_request.edited", label: "Edited", summary: "PR edited" },
      { value: "github.pull_request.enqueued", label: "Enqueued", summary: "PR enqueued" },
      { value: "github.pull_request.labeled", label: "Labeled", summary: "PR labeled" },
      { value: "github.pull_request.locked", label: "Locked", summary: "PR locked" },
      { value: "github.pull_request.milestoned", label: "Milestoned", summary: "PR milestoned" },
      { value: "github.pull_request.opened", label: "Opened", summary: "PR opened" },
      { value: "github.pull_request.ready_for_review", label: "Ready for review", summary: "PR ready for review" },
      { value: "github.pull_request.reopened", label: "Reopened", summary: "PR reopened" },
      { value: "github.pull_request.review_request_removed", label: "Review request removed", summary: "PR review request removed" },
      { value: "github.pull_request.review_requested", label: "Review requested", summary: "PR review requested" },
      { value: "github.pull_request.synchronize", label: "Synchronize", summary: "PR synchronized" },
      { value: "github.pull_request.unassigned", label: "Unassigned", summary: "PR unassigned" },
      { value: "github.pull_request.unlabeled", label: "Unlabeled", summary: "PR unlabeled" },
      { value: "github.pull_request.unlocked", label: "Unlocked", summary: "PR unlocked" },
    ],
  },
  {
    id: "issues",
    label: "Issue",
    events: [
      { value: "github.issues.assigned", label: "Assigned", summary: "Issue assigned" },
      { value: "github.issues.closed", label: "Closed", summary: "Issue closed" },
      { value: "github.issues.deleted", label: "Deleted", summary: "Issue deleted" },
      { value: "github.issues.demilestoned", label: "Demilestoned", summary: "Issue demilestoned" },
      { value: "github.issues.edited", label: "Edited", summary: "Issue edited" },
      { value: "github.issues.field_added", label: "Field added", summary: "Issue field added" },
      { value: "github.issues.field_removed", label: "Field removed", summary: "Issue field removed" },
      { value: "github.issues.labeled", label: "Labeled", summary: "Issue labeled" },
      { value: "github.issues.locked", label: "Locked", summary: "Issue locked" },
      { value: "github.issues.milestoned", label: "Milestoned", summary: "Issue milestoned" },
      { value: "github.issues.opened", label: "Opened", summary: "Issue opened" },
      { value: "github.issues.pinned", label: "Pinned", summary: "Issue pinned" },
      { value: "github.issues.reopened", label: "Reopened", summary: "Issue reopened" },
      { value: "github.issues.transferred", label: "Transferred", summary: "Issue transferred" },
      { value: "github.issues.typed", label: "Typed", summary: "Issue typed" },
      { value: "github.issues.unassigned", label: "Unassigned", summary: "Issue unassigned" },
      { value: "github.issues.unlabeled", label: "Unlabeled", summary: "Issue unlabeled" },
      { value: "github.issues.unlocked", label: "Unlocked", summary: "Issue unlocked" },
      { value: "github.issues.unpinned", label: "Unpinned", summary: "Issue unpinned" },
      { value: "github.issues.untyped", label: "Untyped", summary: "Issue untyped" },
    ],
  },
  {
    id: "release",
    label: "Release",
    events: [
      { value: "github.release.created", label: "Created", summary: "Release created" },
      { value: "github.release.deleted", label: "Deleted", summary: "Release deleted" },
      { value: "github.release.edited", label: "Edited", summary: "Release edited" },
      { value: "github.release.prereleased", label: "Prereleased", summary: "Release prereleased" },
      { value: "github.release.published", label: "Published", summary: "Release published" },
      { value: "github.release.released", label: "Released", summary: "Release released" },
      { value: "github.release.unpublished", label: "Unpublished", summary: "Release unpublished" },
    ],
  },
];

interface GitHubEventPreset {
  label: string;
  event: string;
  filters?: GitHubFilterCondition[];
}

const githubEventPresets: GitHubEventPreset[] = [
  { label: "PR opened", event: "github.pull_request.opened" },
  {
    label: "PR merged",
    event: "github.pull_request.closed",
    filters: [{
      id: "filter-pr-merged",
      field: "is_merged",
      operator: "equals",
      value: "true",
    }],
  },
  { label: "Release published", event: "github.release.published" },
  { label: "Issue opened", event: "github.issues.opened" },
];

const weekDays = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"];

interface GitHubFilterField {
  value: string;
  label: string;
  placeholder: string;
  defaultOperator?: string;
  operators?: readonly string[];
  valueOptions?: readonly string[];
}

const githubPullRequestAuthorFilter: GitHubFilterField = {
  value: "user",
  label: "Author",
  placeholder: "octocat",
};
const githubPullRequestTitleFilter: GitHubFilterField = {
  value: "title",
  label: "Title",
  placeholder: "Fix login, release",
};
const githubPullRequestBodyFilter: GitHubFilterField = {
  value: "body",
  label: "Body",
  placeholder: "Closes #123",
};
const githubPullRequestBranchesFilter: GitHubFilterField = {
  value: "base_branch",
  label: "Base branch",
  placeholder: "main, releases/**",
};
const githubPullRequestHeadBranchesFilter: GitHubFilterField = {
  value: "head_branch",
  label: "Head branch",
  placeholder: "feature/**, dependabot/**",
};
const githubPullRequestLabelsFilter: GitHubFilterField = {
  value: "labels",
  label: "Labels",
  placeholder: "bug, urgent",
};
const githubPullRequestDraftFilter: GitHubFilterField = {
  value: "is_draft",
  label: "Is draft",
  placeholder: "true",
  defaultOperator: "equals",
  operators: ["equals"],
  valueOptions: ["true", "false"],
};
const githubPullRequestMergedFilter: GitHubFilterField = {
  value: "is_merged",
  label: "Is merged",
  placeholder: "true",
  defaultOperator: "equals",
  operators: ["equals"],
  valueOptions: ["true", "false"],
};
const githubIssueStateFilter: GitHubFilterField = {
  value: "state",
  label: "State",
  placeholder: "open, closed",
};
const githubReleaseTagNameFilter: GitHubFilterField = {
  value: "tag",
  label: "Tag name",
  placeholder: "v1.0.0",
};
const githubReleaseTargetBranchFilter: GitHubFilterField = {
  value: "target_branch",
  label: "Target branch",
  placeholder: "main",
};
const githubReleaseNameFilter: GitHubFilterField = {
  value: "title",
  label: "Release name",
  placeholder: "Release v1.0.0",
};
const githubReleaseDraftFilter: GitHubFilterField = {
  value: "is_draft",
  label: "Is draft",
  placeholder: "true",
  defaultOperator: "equals",
  operators: ["equals"],
  valueOptions: ["true", "false"],
};
const githubReleasePrereleaseFilter: GitHubFilterField = {
  value: "is_prerelease",
  label: "Is prerelease",
  placeholder: "true",
  defaultOperator: "equals",
  operators: ["equals"],
  valueOptions: ["true", "false"],
};
const githubFilterFieldsByCategory: Record<GitHubEventCategory, GitHubFilterField[]> = {
  pull_request: [
    githubPullRequestAuthorFilter,
    githubPullRequestTitleFilter,
    githubPullRequestBodyFilter,
    githubPullRequestBranchesFilter,
    githubPullRequestHeadBranchesFilter,
    githubPullRequestLabelsFilter,
    githubPullRequestDraftFilter,
    githubPullRequestMergedFilter,
  ],
  issues: [
    githubPullRequestAuthorFilter,
    githubPullRequestTitleFilter,
    githubPullRequestBodyFilter,
    githubIssueStateFilter,
    githubPullRequestLabelsFilter,
  ],
  release: [
    githubReleaseTagNameFilter,
    githubReleaseTargetBranchFilter,
    githubReleaseNameFilter,
    githubReleaseDraftFilter,
    githubReleasePrereleaseFilter,
  ],
};

const githubFilterOperators = [
  "is one of",
  "is not one of",
  "starts with",
  "contains",
  "matches regex (whole string)",
  "equals",
] as const;

interface GitHubFilterCondition {
  id: string;
  field: string;
  operator: string;
  value: string;
}

function SelectablePill({
  children,
  selected,
  onClick,
}: {
  children: React.ReactNode;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors hover:bg-accent data-[selected=true]:border-primary data-[selected=true]:bg-primary/10 data-[selected=true]:text-primary"
      data-selected={selected}
    >
      {selected && <Check className="size-3" />}
      {children}
    </button>
  );
}

function EmptyHint({ children }: { children: React.ReactNode }) {
  return (
    <span className="text-xs text-muted-foreground">
      {children}
    </span>
  );
}

export default function RoutinesPage() {
  const searchParams = useSearchParams();
  const editRoutineID = searchParams.get("id");
  const mode = searchParams.get("new") === "1" ? "new" : "list";

  if (mode === "new") {
    return <RoutineCreatePage routineID={editRoutineID} />;
  }
  return <RoutineListPage />;
}

function RoutineListPage() {
  const currentUser = useAuthStore((s) => s.user);
  const members = useWorkspaceStore((s) => s.members);
  const [routines, setRoutines] = useState<Routine[]>([]);
  const [loading, setLoading] = useState(true);
  const currentMember = members.find((member) => member.user_id === currentUser?.id);
  const canManageRoutines = currentMember?.role === "owner" || currentMember?.role === "admin";

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api.listRoutines()
      .then((result) => {
        if (!cancelled) setRoutines(result);
      })
      .catch((error) => {
        if (!cancelled) toast.error(error instanceof Error ? error.message : "Failed to load routines");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <main className="h-full min-h-0 overflow-y-auto bg-background">
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-5 px-6 py-5">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">Routines</h1>
            <p className="text-sm text-muted-foreground">
              Automate recurring work, GitHub events, and API-triggered issue creation.
            </p>
          </div>
          {canManageRoutines && (
            <a href="/routines?new=1" className={buttonVariants()}>
              <Plus className="size-4" />
              New routine
            </a>
          )}
        </div>

        {loading ? (
          <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
            Loading routines...
          </div>
        ) : routines.length === 0 ? (
          <div className="rounded-xl border bg-muted/20 p-8 text-center">
            <div className="mx-auto mb-3 flex size-10 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <Sparkles className="size-5" />
            </div>
            <h2 className="text-sm font-medium">No routines yet</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              {canManageRoutines
                ? "Create your first routine to schedule work or react to external events."
                : "Ask an owner or admin to create routines for this workspace."}
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {routines.map((routine) => (
              <a
                key={routine.id}
                href={`/routines/${routine.id}`}
                className="flex items-center justify-between rounded-xl border px-4 py-3 transition-colors hover:bg-accent/50"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">{routine.name}</span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {routine.triggers.length} trigger{routine.triggers.length === 1 ? "" : "s"}
                    {" · "}
                    {routine.enabled ? "Enabled" : "Paused"}
                  </span>
                </span>
              </a>
            ))}
          </div>
        )}
      </div>
    </main>
  );
}

function RoutineCreatePage({ routineID }: { routineID: string | null }) {
  const currentUser = useAuthStore((s) => s.user);
  const members = useWorkspaceStore((s) => s.members);
  const labels = useLabelStore((s) => s.labels);
  const { getActorName } = useActorName();
  const [savedRoutineID, setSavedRoutineID] = useState<string | null>(routineID);

  const [name, setName] = useState("");
  const [instructions, setInstructions] = useState("");
  const [priority, setPriority] = useState<IssuePriority>("medium");
  const [assigneeType, setAssigneeType] = useState<IssueAssigneeType | null>(null);
  const [assigneeId, setAssigneeId] = useState<string | null>(null);
  const [dispatchProvider, setDispatchProvider] = useState<string | null>(null);
  const [dispatchDaemonId, setDispatchDaemonId] = useState<string | null>(null);
  const [dispatchDaemonLabel, setDispatchDaemonLabel] = useState<string | null>(null);
  const [selectedSubscriberIds, setSelectedSubscriberIds] = useState<string[]>(
    () => (!routineID && currentUser?.id ? [currentUser.id] : []),
  );
  const [defaultedCurrentUserSubscriber, setDefaultedCurrentUserSubscriber] = useState(Boolean(!routineID && currentUser?.id));
  const [selectedLabelIds, setSelectedLabelIds] = useState<string[]>([]);
  const [triggerDrafts, setTriggerDrafts] = useState<RoutineTriggerDraft[]>([]);
  const [openTriggerId, setOpenTriggerId] = useState<string | null>(null);
  const [addingTrigger, setAddingTrigger] = useState(false);
  const [enabled, setEnabled] = useState(true);
  const [githubAutoFixEnabled, setGithubAutoFixEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [regeneratingApiToken, setRegeneratingApiToken] = useState(false);
  const [loadingRoutine, setLoadingRoutine] = useState(Boolean(routineID));
  const [apiTokenReveal, setApiTokenReveal] = useState<ApiTokenReveal | null>(null);

  const assigneeLabel =
    assigneeType && assigneeId ? getActorName(assigneeType, assigneeId) : "Unassigned";
  const subscriberLabel = summarizeSelectedMembers(selectedSubscriberIds, members, "No subscribers");
  const currentMember = members.find((member) => member.user_id === currentUser?.id);
  const canManageRoutines = currentMember?.role === "owner" || currentMember?.role === "admin";

  const updateAssignee = (patch: Partial<UpdateIssueRequest>) => {
    setAssigneeType(patch.assignee_type ?? null);
    setAssigneeId(patch.assignee_id ?? null);
    setDispatchProvider(patch.dispatch_provider ?? null);
    setDispatchDaemonId(patch.dispatch_daemon_id ?? null);
    setDispatchDaemonLabel(patch.dispatch_daemon_label ?? null);
  };

  const effectiveTriggerDrafts = triggerDrafts.filter(
    (trigger) => trigger.type !== "api" || trigger.apiCredential !== null,
  );
  const canSubmitRoutine =
    name.trim() !== "" &&
    instructions.trim() !== "" &&
    assigneeType !== null &&
    assigneeId !== null &&
    effectiveTriggerDrafts.length > 0;

  const toggleSubscriber = (id: string) => {
    setSelectedSubscriberIds((current) =>
      current.includes(id)
        ? current.filter((subscriberId) => subscriberId !== id)
        : [...current, id],
    );
  };

  const toggleLabel = (id: string) => {
    setSelectedLabelIds((current) =>
      current.includes(id)
        ? current.filter((labelId) => labelId !== id)
        : [...current, id],
    );
  };

  const addTrigger = (type: TriggerType) => {
    const trigger = createRoutineTriggerDraft(type);
    setTriggerDrafts((current) => sortTriggerDrafts([...current, trigger]));
    setOpenTriggerId(trigger.clientId);
    setAddingTrigger(false);
  };

  const updateTriggerDraft = (clientId: string, patch: Partial<RoutineTriggerDraft>) => {
    setTriggerDrafts((current) =>
      sortTriggerDrafts(
        current.map((trigger) =>
          trigger.clientId === clientId ? { ...trigger, ...patch } : trigger,
        ),
      ),
    );
  };

  const updateScheduleDraft = (clientId: string, patch: Partial<ScheduleTriggerConfig>) => {
    setTriggerDrafts((current) =>
      sortTriggerDrafts(
        current.map((trigger) =>
          trigger.clientId === clientId
            ? { ...trigger, schedule: { ...trigger.schedule, ...patch } }
            : trigger,
        ),
      ),
    );
  };

  const removeTrigger = (clientId: string) => {
    setTriggerDrafts((current) => current.filter((trigger) => trigger.clientId !== clientId));
    setOpenTriggerId((current) => (current === clientId ? null : current));
  };

  useEffect(() => {
    if (!routineID) return;
    let cancelled = false;
    setLoadingRoutine(true);
    api.getRoutine(routineID)
      .then((routine) => {
        if (cancelled) return;
        setName(routine.name);
        setInstructions(routine.instructions ?? "");
        setPriority(routine.priority);
        setAssigneeType(routine.assignee_type ?? null);
        setAssigneeId(routine.assignee_id ?? null);
        setDispatchProvider(routine.dispatch_provider ?? null);
        setDispatchDaemonId(routine.dispatch_daemon_id ?? null);
        setDispatchDaemonLabel(routine.dispatch_daemon_label ?? null);
        setSelectedSubscriberIds(routine.subscriber_ids ?? []);
        setSelectedLabelIds(routine.label_ids ?? []);
        setEnabled(routine.enabled);
        setGithubAutoFixEnabled(routine.github_auto_fix_enabled);
        const drafts = sortTriggerDrafts(routine.triggers.map(routineTriggerToDraft));
        setTriggerDrafts(drafts);
        setOpenTriggerId(drafts[0]?.clientId ?? null);
      })
      .catch((error) => {
        if (!cancelled) toast.error(error instanceof Error ? error.message : "Failed to load routine");
      })
      .finally(() => {
        if (!cancelled) setLoadingRoutine(false);
      });
    return () => {
      cancelled = true;
    };
  }, [routineID]);

  useEffect(() => {
    if (routineID || defaultedCurrentUserSubscriber || !currentUser?.id) return;
    setSelectedSubscriberIds([currentUser.id]);
    setDefaultedCurrentUserSubscriber(true);
  }, [currentUser?.id, defaultedCurrentUserSubscriber, routineID]);

  const handleRegenerateApiToken = async (clientId: string) => {
    const trigger = triggerDrafts.find((draft) => draft.clientId === clientId);
    if (!trigger || trigger.type !== "api") return;

    setRegeneratingApiToken(true);
    try {
      if (!savedRoutineID || !trigger.apiCredential || trigger.apiCredential.tokenDraftId) {
        const result = await generateRoutineTriggerTokenDraft();
        const credential = {
          id: result.draft_id,
          tokenDraftId: result.draft_id,
          tokenPrefix: result.token_prefix,
        };
        updateTriggerDraft(clientId, { id: result.draft_id, apiCredential: credential });
        setApiTokenReveal({ token: result.token, url: getRoutineTriggerURL(result.draft_id) });
        toast.success("API token generated");
        return;
      }
      const result = await regenerateRoutineTriggerToken(savedRoutineID, trigger.apiCredential.id);
      updateTriggerDraft(clientId, {
        id: result.trigger.id,
        apiCredential: {
          ...trigger.apiCredential,
          id: result.trigger.id,
          tokenPrefix: result.trigger.token_prefix,
          tokenDraftId: undefined,
        },
      });
      setApiTokenReveal({ token: result.token, url: getRoutineTriggerURL(result.trigger.id) });
      toast.success("API token regenerated");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update API token");
    } finally {
      setRegeneratingApiToken(false);
    }
  };

  const handleCreateRoutine = async () => {
    if (!name.trim()) {
      toast.error("Routine name is required");
      return;
    }
    if (!canSubmitRoutine) {
      toast.error("Name, instructions, assignee, and at least one trigger are required");
      return;
    }
    setSaving(true);
    try {
      const payload = buildRoutinePayload({
        name,
        instructions,
        priority,
        assigneeType,
        assigneeId,
        dispatchProvider,
        dispatchDaemonId,
        dispatchDaemonLabel,
        enabled,
        githubAutoFixEnabled,
        selectedSubscriberIds,
        selectedLabelIds,
        triggers: effectiveTriggerDrafts,
      });
      if (savedRoutineID) {
        await api.updateRoutine(savedRoutineID, payload);
        toast.success("Routine updated");
        window.location.href = `/routines/${savedRoutineID}`;
      } else {
        const created = await api.createRoutine(payload);
        toast.success("Routine created");
        const apiTriggers = created.triggers.filter((trigger) => trigger.trigger_type === "api");
        if (apiTriggers.length > 0) {
          setSavedRoutineID(created.id);
          setTriggerDrafts((current) =>
            sortTriggerDrafts(
              current.map((draft) => {
                if (draft.type !== "api" || !draft.apiCredential) return draft;
                const apiTrigger = apiTriggers.find((createdTrigger) => createdTrigger.id === draft.apiCredential?.id);
                if (!apiTrigger) return draft;
                return {
                  ...draft,
                  id: apiTrigger.id,
                  apiCredential: {
                    id: apiTrigger.id,
                    tokenPrefix: apiTrigger.token_prefix,
                    token: apiTrigger.token,
                  },
                };
              }),
            ),
          );
          const tokenTrigger = apiTriggers.find((trigger) => trigger.token);
          if (tokenTrigger?.token) {
            setApiTokenReveal({ token: tokenTrigger.token, url: getRoutineTriggerURL(tokenTrigger.id) });
          }
          window.history.replaceState(null, "", `/routines?new=1&id=${created.id}`);
        } else {
          window.location.href = "/routines";
        }
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to create routine");
    } finally {
      setSaving(false);
    }
  };

  return (
    <main
      data-testid="routines-page-scroll"
      className="h-full min-h-0 overflow-y-auto bg-background"
    >
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-4 px-6 py-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Sparkles className="size-4" />
            <a href="/routines" className="hover:text-foreground">Routines</a>
            <span>/</span>
            <span>New routine</span>
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">{savedRoutineID ? "Edit routine" : "New routine"}</h1>
        </div>

        {!canManageRoutines ? (
          <div className="rounded-xl border bg-muted/20 p-8 text-center">
            <h2 className="text-sm font-medium">Read-only</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Ask an owner or admin to create or edit routines for this workspace.
            </p>
          </div>
        ) : loadingRoutine ? (
          <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
            Loading routine...
          </div>
        ) : (
        <>
        <section className="space-y-4">
            <section className="grid gap-2">
              <Label htmlFor="routine-name">
                Name <span className="text-destructive">*</span>
              </Label>
              <Input
                id="routine-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="E.g., Daily code review"
                className="h-10"
              />
            </section>

            <section className="grid gap-2">
              <div className="flex items-center justify-between gap-3">
                <Label htmlFor="routine-instructions">Instructions</Label>
                <Badge variant="secondary">Issue template</Badge>
              </div>
              <Textarea
                id="routine-instructions"
                value={instructions}
                onChange={(event) => setInstructions(event.target.value)}
                placeholder="Describe what Multica should create, assign, and ask the agent to do each time this routine runs"
                className="min-h-36 resize-none"
              />
            </section>

            <section className="grid gap-3 pt-1">
              <div>
                <h2 className="text-sm font-semibold">Issue template</h2>
                <p className="text-xs text-muted-foreground">
                  Configure the issue that gets created when this routine runs.
                </p>
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                <div className="grid gap-2">
                  <Label>Priority</Label>
                  <PriorityPicker
                    priority={priority}
                    onUpdate={(updates) => {
                      if (updates.priority) setPriority(updates.priority);
                    }}
                    align="start"
                    triggerRender={
                      <Button
                        type="button"
                        variant="outline"
                        className="w-full justify-start font-normal"
                      />
                    }
                  />
                </div>

                <div className="grid gap-2">
                  <Label>Assignee</Label>
                  <AssigneePicker
                    assigneeType={assigneeType}
                    assigneeId={assigneeId}
                    onUpdate={updateAssignee}
                    align="start"
                    triggerRender={
                      <Button
                        type="button"
                        variant="outline"
                        className="w-full justify-between font-normal"
                      />
                    }
                    trigger={
                      <>
                        {assigneeType && assigneeId ? (
                          <ActorAvatar actorType={assigneeType} actorId={assigneeId} size={18} />
                        ) : null}
                        <span className={assigneeType && assigneeId ? "truncate" : "text-muted-foreground"}>
                          {assigneeLabel}
                        </span>
                        <ChevronDown className="ml-auto size-4 text-muted-foreground" />
                      </>
                    }
                  />
                </div>
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                <div className="grid gap-2">
                  <Label>Subscribers</Label>
                  <Popover>
                    <PopoverTrigger
                      render={
                        <Button type="button" variant="outline" className="justify-start font-normal" />
                      }
                    >
                      <Bell className="size-4 text-muted-foreground" />
                      <span className="truncate">{subscriberLabel}</span>
                    </PopoverTrigger>
                    <PopoverContent align="start" className="w-72 p-2">
                      <div className="mb-2 flex items-center gap-2 px-1 text-xs font-medium text-muted-foreground">
                        <Users className="size-3.5" />
                        Notify members when issues are created
                      </div>
                      <div className="flex max-h-56 flex-col gap-1 overflow-y-auto">
                        {members.length === 0 && <EmptyHint>No workspace members yet</EmptyHint>}
                        {members.map((member) => (
                          <button
                            key={member.user_id}
                            type="button"
                            onClick={() => toggleSubscriber(member.user_id)}
                            className="flex items-center justify-between rounded-md px-2 py-1.5 text-sm hover:bg-accent"
                          >
                            <span className="truncate">{member.name}</span>
                            {selectedSubscriberIds.includes(member.user_id) && (
                              <Check className="size-4 text-primary" />
                            )}
                          </button>
                        ))}
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>

                <div className="grid gap-2">
                  <Label>Labels</Label>
                  <div className="flex min-h-10 flex-wrap items-center gap-2 rounded-md border px-3 py-2">
                    {labels.length === 0 && <EmptyHint>No labels configured</EmptyHint>}
                    {labels.map((label) => (
                      <SelectablePill
                        key={label.id}
                        selected={selectedLabelIds.includes(label.id)}
                        onClick={() => toggleLabel(label.id)}
                      >
                        <Tag className="size-3" />
                        {label.name}
                      </SelectablePill>
                    ))}
                  </div>
                </div>
              </div>
            </section>
        </section>

        <section className="space-y-3 pt-1">
            <div>
              <h2 className="text-sm font-semibold">Select a trigger</h2>
              <p className="text-xs text-muted-foreground">
                Choose what starts this routine. Schedule triggers use the same recurrence shape shown here; GitHub uses the existing event filters.
              </p>
            </div>

            <div className="space-y-2">
              {triggerDrafts.map((trigger, index) => {
                const option = getTriggerOption(trigger.type);
                const open = openTriggerId === trigger.clientId;
                return (
                  <TriggerAccordionItem
                    key={trigger.clientId}
                    option={option}
                    title={
                      trigger.type === "schedule"
                        ? buildScheduleSummary(trigger.schedule)
                        : trigger.type === "github"
                          ? buildGitHubEventSummary(trigger.githubEventValue)
                          : "Call via API"
                    }
                    description={
                      trigger.type === "api"
                        ? "Generate an endpoint token for external calls."
                        : `Trigger ${index + 1}`
                    }
                    selected={true}
                    open={open}
                    onToggle={() => setOpenTriggerId((current) => (current === trigger.clientId ? null : trigger.clientId))}
                    onRemove={() => removeTrigger(trigger.clientId)}
                  >
                    {trigger.type === "schedule" && (
                      <ScheduleTriggerEditor
                        mode={trigger.schedule.mode}
                        onModeChange={(mode) => updateScheduleDraft(trigger.clientId, { mode })}
                        runAt={trigger.schedule.runAt}
                        onRunAtChange={(runAt) => updateScheduleDraft(trigger.clientId, { runAt })}
                        hourlyMinute={trigger.schedule.hourlyMinute}
                        onHourlyMinuteChange={(hourlyMinute) => updateScheduleDraft(trigger.clientId, { hourlyMinute })}
                        timeOfDay={trigger.schedule.timeOfDay}
                        onTimeOfDayChange={(timeOfDay) => updateScheduleDraft(trigger.clientId, { timeOfDay })}
                        weeklyDay={trigger.schedule.weeklyDay}
                        onWeeklyDayChange={(weeklyDay) => updateScheduleDraft(trigger.clientId, { weeklyDay })}
                        cronExpression={trigger.schedule.cronExpression}
                        onCronExpressionChange={(cronExpression) => updateScheduleDraft(trigger.clientId, { cronExpression })}
                      />
                    )}

                    {trigger.type === "github" && (
                      <GitHubTriggerEditor
                        selectedEvent={trigger.githubEventValue}
                        filters={trigger.githubFilters}
                        onSelectedEventChange={(githubEventValue) => updateTriggerDraft(trigger.clientId, { githubEventValue })}
                        onFiltersChange={(githubFilters) => updateTriggerDraft(trigger.clientId, { githubFilters })}
                      />
                    )}

                    {trigger.type === "api" && (
                      <ApiTriggerPanel
                        credential={trigger.apiCredential}
                        regenerating={regeneratingApiToken}
                        onRegenerate={() => handleRegenerateApiToken(trigger.clientId)}
                      />
                    )}
                  </TriggerAccordionItem>
                );
              })}
            </div>

            {addingTrigger && (
              <div className="space-y-2 pl-3">
                {triggerOptions.map((option) => (
                    <AvailableTriggerRow
                      key={option.id}
                      option={option}
                      onSelect={() => addTrigger(option.id)}
                    />
                ))}
              </div>
            )}

            <button
              type="button"
              onClick={() => {
                setAddingTrigger((open) => {
                  const next = !open;
                  if (next) setOpenTriggerId(null);
                  return next;
                });
              }}
              className="text-sm text-muted-foreground transition-colors hover:text-foreground"
            >
              {addingTrigger ? "⌄" : "+"} Add another trigger
            </button>

            <Tabs defaultValue="connectors" className="pt-2">
              <TabsList>
                <TabsTrigger value="connectors">Connectors</TabsTrigger>
                <TabsTrigger value="behavior">Behavior</TabsTrigger>
                <TabsTrigger value="permissions">Permissions</TabsTrigger>
              </TabsList>
              <TabsContent value="connectors" className="mt-4">
                <PreviewPanel
                  title="Connectors"
                  description="Connector configuration will be saved with the trigger once the routine backend is connected."
                />
              </TabsContent>
              <TabsContent value="behavior" className="mt-4">
                <div className="rounded-xl border bg-muted/20 px-4 py-3">
                  <div className="flex items-center justify-between gap-4">
                    <div className="min-w-0">
                      <div className="text-sm font-medium">Auto-fix pull requests</div>
                      <p className="text-xs text-muted-foreground">
                        Watch failed checks, workflow completions, and review comments on PRs this routine links.
                      </p>
                    </div>
                    <Switch
                      checked={githubAutoFixEnabled}
                      onCheckedChange={setGithubAutoFixEnabled}
                      aria-label="Auto-fix pull requests"
                    />
                  </div>
                </div>
              </TabsContent>
              <TabsContent value="permissions" className="mt-4">
                <PreviewPanel
                  title="Permissions"
                  description="Owners and admins can manage routines. Members can view run history and issues created by routines."
                />
              </TabsContent>
            </Tabs>
        </section>

        <div className="flex items-center justify-end gap-3 border-t pt-4">
          <div className="flex items-center gap-2 rounded-full border bg-card px-3 py-1.5">
            <Switch checked={enabled} onCheckedChange={setEnabled} aria-label="Routine enabled" />
            <span className="text-xs font-medium">{enabled ? "Enabled" : "Paused"}</span>
          </div>
          <Button disabled={saving || loadingRoutine || !canSubmitRoutine} onClick={handleCreateRoutine}>
            <Save className="size-4" />
            {saving ? (savedRoutineID ? "Updating..." : "Creating...") : (savedRoutineID ? "Update routine" : "Create routine")}
          </Button>
        </div>
        </>
        )}
      </div>
      <ApiTokenRevealDialog
        reveal={apiTokenReveal}
        onClose={() => setApiTokenReveal(null)}
      />
    </main>
  );
}

function TriggerAccordionItem({
  option,
  title,
  description,
  selected,
  open,
  onToggle,
  onRemove,
  children,
}: {
  option: TriggerOption;
  title?: string;
  description?: string;
  selected: boolean;
  open: boolean;
  onToggle: () => void;
  onRemove: () => void;
  children: React.ReactNode;
}) {
  const Icon = option.icon;
  const displayTitle = title ?? option.label;
  const displayDescription = description ?? option.description;

  return (
    <div
      className="overflow-hidden rounded-xl border bg-card data-[open=true]:bg-muted/40"
      data-open={open}
    >
      <div className="flex items-center gap-2 px-4 py-3 transition-colors hover:bg-accent/50">
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={open}
          aria-pressed={selected}
          className="flex min-w-0 flex-1 items-center gap-3 text-left outline-none"
        >
          <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
            <Icon className="size-4" />
          </span>
          <span className="min-w-0 flex-1">
            <span className="block text-sm font-medium">{displayTitle}</span>
            {!open && (
              <span className="block truncate text-xs text-muted-foreground">
                {displayDescription}
              </span>
            )}
          </span>
        </button>
        {selected && (
          <button
            type="button"
            aria-label="Remove trigger"
            onClick={(event) => {
              event.stopPropagation();
              onRemove();
            }}
            className="rounded-md p-1 text-muted-foreground hover:bg-background hover:text-foreground"
          >
            <X className="size-4" />
          </button>
        )}
      </div>
      {open && (
        <div className="border-t bg-background/60 p-3">
          {children}
        </div>
      )}
    </div>
  );
}

function AvailableTriggerRow({
  option,
  onSelect,
  disabled = false,
  disabledReason,
}: {
  option: TriggerOption;
  onSelect: () => void;
  disabled?: boolean;
  disabledReason?: string;
}) {
  const Icon = option.icon;

  return (
    <button
      type="button"
      onClick={onSelect}
      disabled={disabled}
      className="flex w-full items-center gap-3 rounded-xl bg-muted/60 px-4 py-3 text-left transition-colors hover:bg-accent/60 disabled:cursor-not-allowed disabled:opacity-45"
    >
      <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-background/70 text-muted-foreground">
        <Icon className="size-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-medium">{option.label}</span>
        <span className="block truncate text-xs text-muted-foreground">
          {option.description}
        </span>
      </span>
      {disabledReason && (
        <span className="shrink-0 text-xs text-muted-foreground">{disabledReason}</span>
      )}
    </button>
  );
}

function ScheduleTriggerEditor({
  mode,
  onModeChange,
  runAt,
  onRunAtChange,
  hourlyMinute,
  onHourlyMinuteChange,
  timeOfDay,
  onTimeOfDayChange,
  weeklyDay,
  onWeeklyDayChange,
  cronExpression,
  onCronExpressionChange,
}: {
  mode: ScheduleMode;
  onModeChange: (mode: ScheduleMode) => void;
  runAt: string;
  onRunAtChange: (value: string) => void;
  hourlyMinute: string;
  onHourlyMinuteChange: (value: string) => void;
  timeOfDay: string;
  onTimeOfDayChange: (value: string) => void;
  weeklyDay: string;
  onWeeklyDayChange: (value: string) => void;
  cronExpression: string;
  onCronExpressionChange: (value: string) => void;
}) {
  return (
    <div className="rounded-xl bg-muted/60 p-3">
      <Tabs value={mode} onValueChange={(value) => onModeChange(value as ScheduleMode)}>
        <TabsList className="h-8">
          {scheduleModes.map((option) => (
            <TabsTrigger key={option.value} value={option.value} className="h-7 px-2.5">
              {option.label}
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value="once" className="mt-4 space-y-2">
          <Label htmlFor="routine-run-at" className="text-xs text-muted-foreground">
            Run at
          </Label>
          <Input
            id="routine-run-at"
            value={runAt}
            onChange={(event) => onRunAtChange(event.target.value)}
            className="h-10 bg-background"
          />
        </TabsContent>

        <TabsContent value="hourly" className="mt-4 space-y-2">
          <div className="flex items-center gap-2">
            <Label htmlFor="routine-hour-minute" className="text-sm">
              At minute
            </Label>
            <Input
              id="routine-hour-minute"
              value={hourlyMinute}
              onChange={(event) => onHourlyMinuteChange(event.target.value)}
              className="h-10 w-28 bg-background"
            />
          </div>
          <p className="text-xs text-muted-foreground">
            Runs are staggered by a few minutes to spread server load.
          </p>
        </TabsContent>

        <TabsContent value="daily" className="mt-4 space-y-2">
          <TimeField value={timeOfDay} onChange={onTimeOfDayChange} />
          <p className="text-xs text-muted-foreground">
            Runs are staggered by a few minutes to spread server load.
          </p>
        </TabsContent>

        <TabsContent value="weekdays" className="mt-4 space-y-2">
          <TimeField value={timeOfDay} onChange={onTimeOfDayChange} />
          <p className="text-xs text-muted-foreground">
            Runs Monday through Friday in the workspace timezone.
          </p>
        </TabsContent>

        <TabsContent value="weekly" className="mt-4 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <TimeField value={timeOfDay} onChange={onTimeOfDayChange} />
            <Label className="text-sm">On</Label>
            <Select value={weeklyDay} onValueChange={(value) => onWeeklyDayChange(value ?? "Wednesday")}>
              <SelectTrigger className="h-10 w-48 bg-background">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {weekDays.map((day) => (
                  <SelectItem key={day} value={day}>
                    {day}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <p className="text-xs text-muted-foreground">
            Runs are staggered by a few minutes to spread server load.
          </p>
        </TabsContent>

        <TabsContent value="custom" className="mt-4 space-y-2">
          <Label htmlFor="routine-cron" className="text-xs text-muted-foreground">
            Cron expression
          </Label>
          <Input
            id="routine-cron"
            value={cronExpression}
            onChange={(event) => onCronExpressionChange(event.target.value)}
            className="h-10 bg-background"
          />
          <div className="rounded-lg border bg-background/70 p-3 text-xs text-muted-foreground">
            <div className="mb-2 font-medium text-foreground">
              Format: minute hour day-of-month month day-of-week
            </div>
            <div className="grid gap-1 sm:grid-cols-5">
              <span><code className="text-foreground">0</code> minute</span>
              <span><code className="text-foreground">9</code> hour</span>
              <span><code className="text-foreground">*</code> any day</span>
              <span><code className="text-foreground">*</code> any month</span>
              <span><code className="text-foreground">1</code> Monday</span>
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            Runs are staggered by a few minutes to spread server load.
          </p>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function TimeField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <div className="flex items-center gap-2">
      <Label htmlFor="routine-time" className="text-sm">
        At
      </Label>
      <div className="relative">
        <Input
          id="routine-time"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="h-10 w-48 bg-background pr-9"
        />
        <Clock className="pointer-events-none absolute right-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      </div>
    </div>
  );
}

function createRoutineTriggerDraft(type: TriggerType): RoutineTriggerDraft {
  return {
    clientId: createClientId(),
    type,
    schedule: createDefaultScheduleConfig(),
    githubEventValue: "github.pull_request.opened",
    githubFilters: [],
    apiCredential: null,
  };
}

function routineTriggerToDraft(trigger: Routine["triggers"][number]): RoutineTriggerDraft {
  const draft = createRoutineTriggerDraft(trigger.trigger_type as TriggerType);
  draft.id = trigger.id;

  if (trigger.trigger_type === "schedule") {
    const mode = getScheduleMode(trigger.config?.mode);
    draft.schedule = {
      ...draft.schedule,
      mode: mode ?? (trigger.run_at ? "once" : "custom"),
      runAt: trigger.run_at ?? draft.schedule.runAt,
      cronExpression: trigger.schedule ?? draft.schedule.cronExpression,
    };
  }

  if (trigger.trigger_type === "github") {
    const eventTypes = trigger.config?.event_types;
    if (Array.isArray(eventTypes)) {
      const event = eventTypes.find((value): value is string => typeof value === "string");
      if (event) draft.githubEventValue = event;
    }
    draft.githubFilters = parseGitHubFilters(trigger.config?.filters);
  }

  if (trigger.trigger_type === "api") {
    draft.apiCredential = {
      id: trigger.id,
      tokenPrefix: trigger.token_prefix,
      token: trigger.token,
    };
  }

  return draft;
}

function createDefaultScheduleConfig(): ScheduleTriggerConfig {
  return {
    mode: "once",
    runAt: "2026/05/21, 18:53",
    hourlyMinute: "0",
    timeOfDay: "09:00",
    weeklyDay: "Wednesday",
    cronExpression: "0 9 * * 1",
  };
}

function getScheduleMode(value: unknown): ScheduleMode | null {
  return scheduleModes.some((mode) => mode.value === value) ? (value as ScheduleMode) : null;
}

function getTriggerOption(type: TriggerType) {
  return triggerOptions.find((option) => option.id === type) ?? triggerOptions[0]!;
}

function sortTriggerDrafts(triggers: RoutineTriggerDraft[]) {
  const order = new Map<TriggerType, number>(
    triggerOptions.map((option, index) => [option.id, index]),
  );
  return [...triggers].sort((a, b) => (order.get(a.type) ?? 0) - (order.get(b.type) ?? 0));
}

function createClientId() {
  return `trigger-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function GitHubTriggerEditor({
  selectedEvent,
  filters,
  onSelectedEventChange,
  onFiltersChange,
}: {
  selectedEvent: string;
  filters: GitHubFilterCondition[];
  onSelectedEventChange: (event: string) => void;
  onFiltersChange: (filters: GitHubFilterCondition[]) => void;
}) {
  const [query, setQuery] = useState("");
  const [menuOpen, setMenuOpen] = useState(false);
  const summary = buildGitHubEventSummary(selectedEvent);
  const isCustomSelection = !githubEventPresets.some((preset) => isGitHubPresetSelected(preset, selectedEvent, filters));
  const availableFilterFields = getGitHubFilterFields(selectedEvent);
  const selectedFilterFields = new Set(availableFilterFields.map((field) => field.value));

  const normalizedQuery = query.trim().toLowerCase();
  const searchResults = normalizedQuery
    ? githubEventGroups
        .map((group) => ({
          group,
          events: group.events.filter(
            (event) =>
              event.label.toLowerCase().includes(normalizedQuery) ||
              event.value.toLowerCase().includes(normalizedQuery),
          ),
        }))
        .filter((entry) => entry.events.length > 0)
    : [];

  const selectEvent = (event: string) => {
    const nextFilterFields = new Set(getGitHubFilterFields(event).map((field) => field.value));
    onFiltersChange(filters.filter((filter) => nextFilterFields.has(filter.field)));
    onSelectedEventChange(event);
    setQuery("");
    setMenuOpen(false);
  };

  const selectPreset = (preset: GitHubEventPreset) => {
    onFiltersChange(preset.filters ?? []);
    onSelectedEventChange(preset.event);
    setQuery("");
    setMenuOpen(false);
  };

  const addFilter = () => {
    const defaultField = availableFilterFields[0];
    if (!defaultField) return;
    onFiltersChange([
      ...filters,
      {
        id: `filter-${filters.length + 1}`,
        field: defaultField.value,
        operator: defaultField.defaultOperator ?? "is one of",
        value: defaultField.valueOptions?.[0] ?? "",
      },
    ]);
  };

  const updateFilter = (
    id: string,
    patch: Partial<Omit<GitHubFilterCondition, "id">>,
  ) => {
    onFiltersChange(
      filters.map((filter) =>
        filter.id === id ? { ...filter, ...patch } : filter,
      ),
    );
  };

  const removeFilter = (id: string) => {
    onFiltersChange(filters.filter((filter) => filter.id !== id));
  };

  return (
    <div className="space-y-3 rounded-xl bg-muted/60 p-3">
      <div className="space-y-3">
        <div className="flex flex-wrap gap-1.5" role="group" aria-label="GitHub event presets">
          {githubEventPresets.map((preset) => (
            <button
              key={preset.label}
              type="button"
              aria-pressed={isGitHubPresetSelected(preset, selectedEvent, filters)}
              onClick={() => selectPreset(preset)}
              className="rounded-full border px-3 py-1 text-sm font-medium transition-colors hover:bg-accent aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground"
            >
              {preset.label}
            </button>
          ))}
          <button
            type="button"
            aria-pressed={isCustomSelection}
            onClick={() => setMenuOpen(true)}
            className="rounded-full border px-3 py-1 text-sm font-medium transition-colors hover:bg-accent aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground"
          >
            Custom
          </button>
        </div>
        <div className="grid gap-2">
          <Label className="text-xs text-muted-foreground">Event</Label>
          <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
            <DropdownMenuTrigger
              render={
                <Button
                  type="button"
                  variant="outline"
                  aria-label="GitHub event type"
                  className="w-72 justify-between bg-background font-normal"
                />
              }
            >
              <span className="truncate">{summary}</span>
              <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-72 p-0">
              <div className="p-1">
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  onKeyDown={(event) => event.stopPropagation()}
                  placeholder="Search events..."
                  aria-label="Search GitHub events"
                  className="h-8 bg-background"
                />
              </div>
              <DropdownMenuSeparator className="mx-0 mt-0" />
              <div className="max-h-80 overflow-y-auto p-1">
                {normalizedQuery ? (
                  searchResults.length === 0 ? (
                    <div className="px-2 py-6 text-center text-sm text-muted-foreground">
                      No events found
                    </div>
                  ) : (
                    searchResults.map(({ group, events }) => (
                      <DropdownMenuGroup key={group.id}>
                        <DropdownMenuLabel>{group.label}</DropdownMenuLabel>
                        <DropdownMenuRadioGroup value={selectedEvent} onValueChange={selectEvent}>
                          {events.map((event) => (
                            <DropdownMenuRadioItem key={event.value} value={event.value}>
                              {event.label}
                            </DropdownMenuRadioItem>
                          ))}
                        </DropdownMenuRadioGroup>
                      </DropdownMenuGroup>
                    ))
                  )
                ) : (
                  githubEventGroups.map((group) => {
                    const selectedCount = group.events.some((event) => event.value === selectedEvent) ? 1 : 0;
                    return (
                      <DropdownMenuSub key={group.id}>
                        <DropdownMenuSubTrigger>
                          <span>{group.label}</span>
                          {selectedCount > 0 && (
                            <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-[10px]">
                              {selectedCount}
                            </Badge>
                          )}
                        </DropdownMenuSubTrigger>
                        <DropdownMenuSubContent className="max-h-80 w-56 overflow-y-auto">
                          <DropdownMenuRadioGroup value={selectedEvent} onValueChange={selectEvent}>
                            {group.events.map((event) => (
                              <DropdownMenuRadioItem key={event.value} value={event.value}>
                                {event.label}
                              </DropdownMenuRadioItem>
                            ))}
                          </DropdownMenuRadioGroup>
                        </DropdownMenuSubContent>
                      </DropdownMenuSub>
                    );
                  })
                )}
              </div>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <div className="grid gap-1.5">
        <Label className="text-xs text-muted-foreground">Filter</Label>
        {availableFilterFields.length === 0 ? (
          <div className="rounded-lg border bg-background/70 px-3 py-2 text-sm text-muted-foreground">
            This event only supports activity filtering.
          </div>
        ) : filters.length > 0 && (
          <div className="space-y-2">
            {filters.filter((filter) => selectedFilterFields.has(filter.field)).map((filter) => (
              <GitHubFilterRow
                key={filter.id}
                filter={filter}
                fields={availableFilterFields}
                onChange={(patch) => updateFilter(filter.id, patch)}
                onRemove={() => removeFilter(filter.id)}
              />
            ))}
          </div>
        )}
        {availableFilterFields.length > 0 && (
          <Button
            type="button"
            variant="outline"
            onClick={addFilter}
            className="w-fit justify-start bg-background font-normal"
          >
            Add a filter condition
          </Button>
        )}
      </div>

      <div className="flex items-start gap-2 rounded-lg bg-amber-500/10 px-3 py-2 text-sm text-amber-900 dark:text-amber-200">
        <AlertTriangle className="mt-0.5 size-4 shrink-0" />
        <span>
          Fires on every matching event — this can consume your routine run limits quickly.
          Add a filter to narrow it down.
        </span>
      </div>

      <p className="text-xs text-muted-foreground">Runs as the routine creator</p>
    </div>
  );
}

function buildScheduleSummary({
  mode,
  runAt,
  hourlyMinute,
  timeOfDay,
  weeklyDay,
  cronExpression,
}: {
  mode: ScheduleMode;
  runAt: string;
  hourlyMinute: string;
  timeOfDay: string;
  weeklyDay: string;
  cronExpression: string;
}) {
  switch (mode) {
    case "once":
      return `Runs once on ${runAt.replace("/", "-").replace("/", "-")} GMT+8`;
    case "hourly":
      return hourlyMinute === "0" ? "Runs every hour" : `Runs every hour at minute ${hourlyMinute}`;
    case "daily":
      return `Runs daily at ${timeOfDay} GMT+8`;
    case "weekdays":
      return `Runs every weekday at ${timeOfDay} GMT+8`;
    case "weekly":
      return `Runs every ${weeklyDay} at ${timeOfDay} GMT+8`;
    case "custom":
      return cronExpression ? `Runs on cron ${cronExpression}` : "Runs on a custom cron schedule";
  }
}

function buildGitHubEventSummary(selectedEvent: string) {
  const allEvents = githubEventGroups.flatMap((group) => group.events);
  return allEvents.find((event) => event.value === selectedEvent)?.summary ?? selectedEvent;
}

function getGitHubFilterFields(selectedEvent: string) {
  const group = githubEventGroups.find((entry) =>
    entry.events.some((event) => event.value === selectedEvent),
  );
  return group ? githubFilterFieldsByCategory[group.id] : [];
}

function isGitHubPresetSelected(preset: GitHubEventPreset, selectedEvent: string, filters: GitHubFilterCondition[]) {
  if (preset.event !== selectedEvent) return false;
  if (!preset.filters || preset.filters.length === 0) return true;
  return preset.filters.every((presetFilter) =>
    filters.some((filter) =>
      filter.field === presetFilter.field &&
      filter.operator === presetFilter.operator &&
      filter.value === presetFilter.value,
    ),
  );
}

function summarizeSelectedMembers(
  selectedIds: string[],
  members: { user_id: string; name: string }[],
  emptyLabel: string,
) {
  if (selectedIds.length === 0) return emptyLabel;
  const names = selectedIds.map((id) => members.find((member) => member.user_id === id)?.name ?? "Unknown");
  const visibleNames = names.slice(0, 3).join(", ");
  const overflowCount = names.length - 3;
  return overflowCount > 0 ? `${visibleNames} +${overflowCount} more` : visibleNames;
}

function buildRoutinePayload({
  name,
  instructions,
  priority,
  assigneeType,
  assigneeId,
  dispatchProvider,
  dispatchDaemonId,
  dispatchDaemonLabel,
  enabled,
  githubAutoFixEnabled,
  selectedSubscriberIds,
  selectedLabelIds,
  triggers,
}: {
  name: string;
  instructions: string;
  priority: IssuePriority;
  assigneeType: IssueAssigneeType | null;
  assigneeId: string | null;
  dispatchProvider: string | null;
  dispatchDaemonId: string | null;
  dispatchDaemonLabel: string | null;
  enabled: boolean;
  githubAutoFixEnabled: boolean;
  selectedSubscriberIds: string[];
  selectedLabelIds: string[];
  triggers: RoutineTriggerDraft[];
}): CreateRoutineRequest {
  return {
    name: name.trim(),
    instructions: instructions.trim() || null,
    priority,
    assignee_type: assigneeType,
    assignee_id: assigneeId,
    dispatch_provider: dispatchProvider,
    dispatch_daemon_id: dispatchDaemonId,
    dispatch_daemon_label: dispatchDaemonLabel,
    github_auto_fix_enabled: githubAutoFixEnabled,
    enabled,
    subscriber_ids: selectedSubscriberIds,
    label_ids: selectedLabelIds,
    triggers: triggers.map(buildRoutineTrigger),
    actions: [
      {
        action_type: "create_issue",
        config: {},
        enabled: true,
        position: 0,
      },
    ],
  };
}

function buildRoutineTrigger(triggerDraft: RoutineTriggerDraft): RoutineTriggerRequest {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  if (triggerDraft.type === "schedule") {
    const trigger: RoutineTriggerRequest = {
      id: triggerDraft.id,
      trigger_type: "schedule",
      timezone,
      config: { mode: triggerDraft.schedule.mode },
      enabled: true,
    };
    if (triggerDraft.schedule.mode === "once") {
      trigger.run_at = parseLocalDateTime(triggerDraft.schedule.runAt)?.toISOString() ?? null;
      return trigger;
    }
    trigger.schedule = scheduleToCron(
      triggerDraft.schedule.mode,
      triggerDraft.schedule.hourlyMinute,
      triggerDraft.schedule.timeOfDay,
      triggerDraft.schedule.weeklyDay,
      triggerDraft.schedule.cronExpression,
    );
    return trigger;
  }
  if (triggerDraft.type === "github") {
    const config: Record<string, unknown> = { event_types: [triggerDraft.githubEventValue] };
    const activeFilters = triggerDraft.githubFilters.filter((filter) => filter.value.trim() !== "");
    if (activeFilters.length > 0) {
      config.filters = activeFilters.map(({ field, operator, value }) => ({
        field,
        operator,
        value: value.trim(),
      }));
    }
    return {
      id: triggerDraft.id,
      trigger_type: "github",
      config,
      enabled: true,
    };
  }
  return {
    id: triggerDraft.apiCredential?.id,
    trigger_type: "api",
    source_type: "standard",
    token_draft_id: triggerDraft.apiCredential?.tokenDraftId,
    config: {},
    enabled: true,
  };
}

function scheduleToCron(mode: ScheduleMode, hourlyMinute: string, timeOfDay: string, weeklyDay: string, cronExpression: string) {
  if (mode === "custom") return cronExpression.trim();
  const [hour = "9", minute = "0"] = timeOfDay.split(":");
  if (mode === "hourly") return `${normalizeCronNumber(hourlyMinute, "0")} * * * *`;
  if (mode === "daily") return `${normalizeCronNumber(minute, "0")} ${normalizeCronNumber(hour, "9")} * * *`;
  if (mode === "weekdays") return `${normalizeCronNumber(minute, "0")} ${normalizeCronNumber(hour, "9")} * * 1-5`;
  if (mode === "weekly") return `${normalizeCronNumber(minute, "0")} ${normalizeCronNumber(hour, "9")} * * ${weekDayToCron(weeklyDay)}`;
  return cronExpression.trim();
}

function normalizeCronNumber(value: string, fallback: string) {
  const trimmed = value.trim();
  return /^\d+$/.test(trimmed) ? trimmed : fallback;
}

function weekDayToCron(day: string) {
  const index = weekDays.indexOf(day);
  return index < 0 ? "1" : String(index + 1);
}

function parseLocalDateTime(value: string) {
  const match = value.match(/^(\d{4})\/(\d{2})\/(\d{2}),\s*(\d{2}):(\d{2})$/);
  if (!match) return null;
  const [, year, month, day, hour, minute] = match;
  return new Date(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute));
}

function parseGitHubFilters(value: unknown): GitHubFilterCondition[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item, index) => {
    if (!item || typeof item !== "object") return [];
    const filter = item as Record<string, unknown>;
    if (typeof filter.field !== "string") return [];
    return [{
      id: typeof filter.id === "string" ? filter.id : `filter-${index + 1}`,
      field: filter.field,
      operator: typeof filter.operator === "string" ? filter.operator : "matches",
      value: typeof filter.value === "string" ? filter.value : "",
    }];
  });
}

function GitHubFilterRow({
  filter,
  fields,
  onChange,
  onRemove,
}: {
  filter: GitHubFilterCondition;
  fields: GitHubFilterField[];
  onChange: (patch: Partial<Omit<GitHubFilterCondition, "id">>) => void;
  onRemove: () => void;
}) {
  const selectedField = fields.find((field) => field.value === filter.field) ?? fields[0];
  const operators = selectedField?.operators ?? githubFilterOperators;
  return (
    <div className="rounded-lg bg-background p-2">
      <div className="flex items-center gap-2">
        <NativeSelect
          value={filter.field}
          onChange={(event) => {
            const nextField = fields.find((field) => field.value === event.target.value);
            onChange({
              field: event.target.value,
              operator: nextField?.defaultOperator ?? "is one of",
              value: nextField?.valueOptions?.[0] ?? "",
            });
          }}
          className="flex-1"
        >
          {fields.map((field) => (
            <NativeSelectOption key={field.value} value={field.value}>
              {field.label}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <NativeSelect
          value={filter.operator}
          onChange={(event) => onChange({ operator: event.target.value })}
          className="flex-1"
        >
          {operators.map((operator) => (
            <NativeSelectOption key={operator} value={operator}>
              {operator}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <button
          type="button"
          aria-label="Remove filter condition"
          onClick={onRemove}
          className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
        >
          <X className="size-4" />
        </button>
      </div>
      {selectedField?.valueOptions ? (
        <NativeSelect
          value={filter.value}
          onChange={(event) => onChange({ value: event.target.value })}
          className="mt-2 w-full"
        >
          {selectedField.valueOptions.map((option) => (
            <NativeSelectOption key={option} value={option}>
              {option}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      ) : (
        <Input
          value={filter.value}
          onChange={(event) => onChange({ value: event.target.value })}
          placeholder={selectedField?.placeholder ?? "Comma-separated values"}
          className="mt-2 bg-background"
        />
      )}
    </div>
  );
}

function SelectedTriggerPanel({
  icon: Icon,
  title,
  onRemove,
  children,
}: {
  icon: typeof CalendarClock;
  title: string;
  onRemove: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-4 rounded-xl bg-muted/60 p-4">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Icon className="size-4 text-muted-foreground" />
          <span className="text-sm font-medium">{title}</span>
        </div>
        <button
          type="button"
          aria-label={`Remove ${title}`}
          onClick={onRemove}
          className="rounded-md p-1 text-muted-foreground hover:bg-background hover:text-foreground"
        >
          <X className="size-4" />
        </button>
      </div>
      {children}
    </div>
  );
}

function ApiTriggerPanel({
  credential,
  regenerating,
  onRegenerate,
}: {
  credential: ApiTriggerCredential | null;
  regenerating: boolean;
  onRegenerate: () => void;
}) {
  const apiURL = credential
    ? getRoutineTriggerURL(credential.id)
    : "Generate a token to create an API endpoint";
  const tokenValue = credential
    ? `${credential.tokenPrefix}...`
    : "Token will be available after generation";

  return (
    <div className="space-y-3 rounded-xl border bg-muted/30 p-4">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Code2 className="size-4 text-muted-foreground" />
          <h3 className="text-sm font-medium">API trigger</h3>
        </div>
        <Badge variant={credential ? "secondary" : "outline"}>
          {credential ? "Active" : "Not generated"}
        </Badge>
      </div>
      <div className="grid gap-2">
        <Label>API URL</Label>
        <Input value={apiURL} readOnly className="font-mono text-xs" />
      </div>
      <div className="grid gap-2">
        <Label>Token</Label>
        <div className="flex gap-2">
          <Input value={tokenValue} readOnly className="font-mono text-xs" />
          <Button
            type="button"
            variant="outline"
            onClick={onRegenerate}
            disabled={regenerating}
          >
            {regenerating
                ? "Regenerating..."
                : credential
                  ? "Regenerate token"
                  : "Generate token"}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          Save the routine to activate generated API endpoints. Regenerating invalidates the previous token.
        </p>
      </div>
    </div>
  );
}

function ApiTokenRevealDialog({
  reveal,
  onClose,
}: {
  reveal: ApiTokenReveal | null;
  onClose: () => void;
}) {
  const curlRequest = reveal ? buildApiTriggerCurl(reveal.url, reveal.token) : "";
  const copyText = async (value: string) => {
    await navigator.clipboard.writeText(value);
    toast.success("Copied");
  };

  return (
    <Dialog open={!!reveal} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>API token generated</DialogTitle>
          <DialogDescription>
            Copy this token now. After closing, only the token prefix will be shown.
          </DialogDescription>
        </DialogHeader>
        {reveal && (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Token</Label>
              <div className="flex gap-2">
                <code className="min-w-0 flex-1 rounded-md border bg-muted/50 px-3 py-2 text-sm break-all select-all">
                  {reveal.token}
                </code>
                <button
                  type="button"
                  onClick={() => void copyText(reveal.token)}
                  aria-label="Copy token"
                  className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-lg border bg-background px-3 text-sm font-medium transition-colors hover:bg-muted"
                >
                  <Copy className="size-4" />
                  Copy
                </button>
              </div>
            </div>
            <div className="space-y-2">
              <Label>Example curl request</Label>
              <div className="flex items-start gap-2">
                <pre className="min-w-0 flex-1 overflow-x-auto whitespace-pre-wrap rounded-md border bg-muted/50 p-3 text-xs">
                  {curlRequest}
                </pre>
                <button
                  type="button"
                  onClick={() => void copyText(curlRequest)}
                  aria-label="Copy curl request"
                  className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-lg border bg-background px-3 text-sm font-medium transition-colors hover:bg-muted"
                >
                  <Copy className="size-4" />
                  Copy
                </button>
              </div>
            </div>
          </div>
        )}
        <DialogFooter>
          <Button type="button" onClick={onClose}>Done</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function PreviewPanel({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: string;
}) {
  return (
    <div className="rounded-xl border bg-muted/30 p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h3 className="text-sm font-medium">{title}</h3>
          <p className="max-w-2xl text-xs text-muted-foreground">{description}</p>
        </div>
        {action && (
          <Badge variant="outline" className="shrink-0">
            <Plus className="size-3" />
            {action}
          </Badge>
        )}
      </div>
    </div>
  );
}

function getRoutineTriggerURL(triggerID: string) {
  const baseURL = process.env.NEXT_PUBLIC_API_URL || (typeof window !== "undefined" ? window.location.origin : "");
  return `${baseURL}/api/routine-triggers/${triggerID}`;
}

function buildApiTriggerCurl(url: string, token: string) {
  return [
    `curl -X POST ${url} \\`,
    `  -H "Authorization: Bearer ${token}" \\`,
    `  -H "Content-Type: application/json" \\`,
    `  -d '{"title":"Routine run","body":"Triggered from API"}'`,
  ].join("\n");
}
