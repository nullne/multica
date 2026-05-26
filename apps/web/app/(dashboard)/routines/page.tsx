"use client";

import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
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
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
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
import { api, regenerateRoutineTriggerToken } from "@/shared/api";
import type { CreateRoutineRequest, IssueAssigneeType, IssuePriority, Routine, RoutineTriggerRequest, UpdateIssueRequest } from "@/shared/types";
import { toast } from "sonner";

type TriggerType = "schedule" | "github" | "api";
type ScheduleMode = "once" | "hourly" | "daily" | "weekdays" | "weekly" | "custom";

interface ApiTriggerCredential {
  id: string;
  tokenPrefix: string;
  token?: string;
}

interface ApiTokenReveal {
  token: string;
  url: string;
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
      { value: "github.pull_request.closed", label: "Closed", summary: "PR closed" },
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
      { value: "github.pull_request.merged", label: "Merged", summary: "PR merged" },
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
      { value: "github.release.published", label: "Published", summary: "Release published" },
      { value: "github.release.created", label: "Created", summary: "Release created" },
      { value: "github.release.deleted", label: "Deleted", summary: "Release deleted" },
      { value: "github.release.edited", label: "Edited", summary: "Release edited" },
      { value: "github.release.prereleased", label: "Prereleased", summary: "Release prereleased" },
      { value: "github.release.released", label: "Released", summary: "Release released" },
      { value: "github.release.unpublished", label: "Unpublished", summary: "Release unpublished" },
    ],
  },
];

const githubEventPresets: { label: string; events: string[] }[] = [
  { label: "PR opened", events: ["github.pull_request.opened"] },
  { label: "PR merged", events: ["github.pull_request.merged"] },
  { label: "Release published", events: ["github.release.published"] },
  { label: "Issue opened", events: ["github.issues.opened"] },
];

const weekDays = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"];

const githubFilterFields = [
  "Author",
  "Title",
  "Body",
  "State",
  "Labels",
] as const;

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
  const [selectedTriggerTypes, setSelectedTriggerTypes] = useState<TriggerType[]>([]);
  const [openTriggerType, setOpenTriggerType] = useState<TriggerType | null>(null);
  const [addingTrigger, setAddingTrigger] = useState(false);
  const [scheduleMode, setScheduleMode] = useState<ScheduleMode>("once");
  const [runAt, setRunAt] = useState("2026/05/21, 18:53");
  const [hourlyMinute, setHourlyMinute] = useState("0");
  const [timeOfDay, setTimeOfDay] = useState("09:00");
  const [weeklyDay, setWeeklyDay] = useState("Wednesday");
  const [cronExpression, setCronExpression] = useState("0 9 * * 1");
  const [githubEventValues, setGithubEventValues] = useState<string[]>(["github.pull_request.opened"]);
  const [enabled, setEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [regeneratingApiToken, setRegeneratingApiToken] = useState(false);
  const [loadingRoutine, setLoadingRoutine] = useState(Boolean(routineID));
  const [apiTriggerCredential, setApiTriggerCredential] = useState<ApiTriggerCredential | null>(null);
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

  const scheduleSummary = useMemo(
    () =>
      buildScheduleSummary({
        mode: scheduleMode,
        runAt,
        hourlyMinute,
        timeOfDay,
        weeklyDay,
        cronExpression,
      }),
    [cronExpression, hourlyMinute, runAt, scheduleMode, timeOfDay, weeklyDay],
  );
  const githubSummary = useMemo(
    () => buildGitHubEventSummary(githubEventValues),
    [githubEventValues],
  );
  const canSubmitRoutine =
    name.trim() !== "" &&
    instructions.trim() !== "" &&
    assigneeType !== null &&
    assigneeId !== null &&
    selectedTriggerTypes.length > 0;

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

  const toggleTrigger = (type: TriggerType) => {
    setSelectedTriggerTypes((current) =>
      current.includes(type) ? current : [...current, type],
    );
    setOpenTriggerType((current) => (current === type ? null : type));
    setAddingTrigger(false);
  };

  const removeTrigger = (type: TriggerType) => {
    setSelectedTriggerTypes((current) =>
      current.filter((selectedType) => selectedType !== type),
    );
    setOpenTriggerType((current) => (current === type ? null : current));
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
        const triggerTypes = routine.triggers.map((trigger) => trigger.trigger_type as TriggerType);
        setSelectedTriggerTypes(triggerTypes);
        setOpenTriggerType(triggerTypes[0] ?? null);
        const scheduleTrigger = routine.triggers.find((trigger) => trigger.trigger_type === "schedule");
        if (scheduleTrigger?.schedule) {
          setScheduleMode("custom");
          setCronExpression(scheduleTrigger.schedule);
        }
        const githubTrigger = routine.triggers.find((trigger) => trigger.trigger_type === "github");
        const eventTypes = githubTrigger?.config?.event_types;
        if (Array.isArray(eventTypes)) {
          setGithubEventValues(eventTypes.filter((event): event is string => typeof event === "string"));
        }
        const apiTrigger = routine.triggers.find((trigger) => trigger.trigger_type === "api");
        setApiTriggerCredential(apiTrigger ? {
          id: apiTrigger.id,
          tokenPrefix: apiTrigger.token_prefix,
          token: apiTrigger.token,
        } : null);
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

  const handleRegenerateApiToken = async () => {
    if (!savedRoutineID || !apiTriggerCredential) return;
    setRegeneratingApiToken(true);
    try {
      const result = await regenerateRoutineTriggerToken(savedRoutineID, apiTriggerCredential.id);
      setApiTriggerCredential({
        id: result.trigger.id,
        tokenPrefix: result.trigger.token_prefix,
      });
      setApiTokenReveal({ token: result.token, url: getRoutineTriggerURL(result.trigger.id) });
      toast.success("API token regenerated");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to regenerate API token");
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
        selectedSubscriberIds,
        selectedLabelIds,
        selectedTriggerTypes,
        scheduleMode,
        runAt,
        hourlyMinute,
        timeOfDay,
        weeklyDay,
        cronExpression,
        githubEventValues,
      });
      if (savedRoutineID) {
        await api.updateRoutine(savedRoutineID, payload);
        toast.success("Routine updated");
        window.location.href = `/routines/${savedRoutineID}`;
      } else {
        const created = await api.createRoutine(payload);
        toast.success("Routine created");
        const apiTrigger = created.triggers.find((trigger) => trigger.trigger_type === "api");
        if (apiTrigger) {
          setSavedRoutineID(created.id);
          setApiTriggerCredential({
            id: apiTrigger.id,
            tokenPrefix: apiTrigger.token_prefix,
          });
          if (apiTrigger.token) {
            setApiTokenReveal({ token: apiTrigger.token, url: getRoutineTriggerURL(apiTrigger.id) });
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
              {triggerOptions
                .filter((option) => selectedTriggerTypes.includes(option.id))
                .map((option) => {
                const selected = selectedTriggerTypes.includes(option.id);
                const open = openTriggerType === option.id;
                return (
                  <TriggerAccordionItem
                    key={option.id}
                    option={option}
                    title={
                      option.id === "schedule"
                        ? scheduleSummary
                        : option.id === "github"
                          ? githubSummary
                          : "Call via API"
                    }
                    description={
                      option.id === "api"
                        ? "Generate an endpoint token for external calls."
                        : undefined
                    }
                    selected={selected}
                    open={open}
                    onToggle={() => toggleTrigger(option.id)}
                    onRemove={() => removeTrigger(option.id)}
                  >
                    {option.id === "schedule" && (
                      <ScheduleTriggerEditor
                        mode={scheduleMode}
                        onModeChange={setScheduleMode}
                        runAt={runAt}
                        onRunAtChange={setRunAt}
                        hourlyMinute={hourlyMinute}
                        onHourlyMinuteChange={setHourlyMinute}
                        timeOfDay={timeOfDay}
                        onTimeOfDayChange={setTimeOfDay}
                        weeklyDay={weeklyDay}
                        onWeeklyDayChange={setWeeklyDay}
                        cronExpression={cronExpression}
                        onCronExpressionChange={setCronExpression}
                      />
                    )}

                    {option.id === "github" && (
                      <GitHubTriggerEditor
                        selectedEvents={githubEventValues}
                        onSelectedEventsChange={setGithubEventValues}
                      />
                    )}

                    {option.id === "api" && (
                      <ApiTriggerPanel
                        routineID={savedRoutineID}
                        credential={apiTriggerCredential}
                        regenerating={regeneratingApiToken}
                        onRegenerate={handleRegenerateApiToken}
                      />
                    )}
                  </TriggerAccordionItem>
                );
              })}
            </div>

            {addingTrigger && (
              <div className="space-y-2 pl-3">
                {triggerOptions
                  .filter((option) => !selectedTriggerTypes.includes(option.id))
                  .map((option) => (
                    <AvailableTriggerRow
                      key={option.id}
                      option={option}
                      onSelect={() => toggleTrigger(option.id)}
                    />
                  ))}
              </div>
            )}

            {selectedTriggerTypes.length < triggerOptions.length && (
              <button
                type="button"
                onClick={() => {
                  setAddingTrigger((open) => {
                    const next = !open;
                    if (next) setOpenTriggerType(null);
                    return next;
                  });
                }}
                className="text-sm text-muted-foreground transition-colors hover:text-foreground"
              >
                {addingTrigger ? "⌄" : "+"} Add another trigger
              </button>
            )}

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
                <PreviewPanel
                  title="Behavior"
                  description="Advanced actions, deduplication, and comment-on-existing-issue behavior will live here once the routine backend is connected."
                />
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

function GitHubTriggerEditor({
  selectedEvents,
  onSelectedEventsChange,
}: {
  selectedEvents: string[];
  onSelectedEventsChange: (events: string[]) => void;
}) {
  const [filters, setFilters] = useState<GitHubFilterCondition[]>([]);
  const [query, setQuery] = useState("");
  const [menuOpen, setMenuOpen] = useState(false);
  const summary = buildGitHubEventSummary(selectedEvents);

  const matchesPreset = (events: string[]) =>
    events.length === selectedEvents.length &&
    events.every((value) => selectedEvents.includes(value));
  const isCustomSelection = !githubEventPresets.some((preset) => matchesPreset(preset.events));

  const toggleEvent = (value: string) => {
    onSelectedEventsChange(
      selectedEvents.includes(value)
        ? selectedEvents.filter((event) => event !== value)
        : [...selectedEvents, value],
    );
  };

  const toggleGroupAll = (group: (typeof githubEventGroups)[number]) => {
    const values = group.events.map((event) => event.value);
    const allSelected = values.every((value) => selectedEvents.includes(value));
    onSelectedEventsChange(
      allSelected
        ? selectedEvents.filter((event) => !values.includes(event))
        : Array.from(new Set([...selectedEvents, ...values])),
    );
  };

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

  const addFilter = () => {
    setFilters((current) => [
      ...current,
      {
        id: `filter-${current.length + 1}`,
        field: "Author",
        operator: "is one of",
        value: "",
      },
    ]);
  };

  const updateFilter = (
    id: string,
    patch: Partial<Omit<GitHubFilterCondition, "id">>,
  ) => {
    setFilters((current) =>
      current.map((filter) =>
        filter.id === id ? { ...filter, ...patch } : filter,
      ),
    );
  };

  const removeFilter = (id: string) => {
    setFilters((current) => current.filter((filter) => filter.id !== id));
  };

  return (
    <div className="space-y-3 rounded-xl bg-muted/60 p-3">
      <div className="space-y-3">
        <div className="flex flex-wrap gap-1.5" role="group" aria-label="GitHub event presets">
          {githubEventPresets.map((preset) => (
            <button
              key={preset.label}
              type="button"
              aria-pressed={matchesPreset(preset.events)}
              onClick={() => onSelectedEventsChange(preset.events)}
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
                        {events.map((event) => (
                          <DropdownMenuCheckboxItem
                            key={event.value}
                            checked={selectedEvents.includes(event.value)}
                            closeOnClick={false}
                            onCheckedChange={() => toggleEvent(event.value)}
                          >
                            {event.label}
                          </DropdownMenuCheckboxItem>
                        ))}
                      </DropdownMenuGroup>
                    ))
                  )
                ) : (
                  githubEventGroups.map((group) => {
                    const selectedCount = group.events.filter((event) =>
                      selectedEvents.includes(event.value),
                    ).length;
                    const allSelected = selectedCount === group.events.length;
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
                          <DropdownMenuCheckboxItem
                            checked={allSelected}
                            closeOnClick={false}
                            onCheckedChange={() => toggleGroupAll(group)}
                          >
                            <span className="font-medium">
                              {allSelected ? "Clear all" : "Select all"}
                            </span>
                          </DropdownMenuCheckboxItem>
                          <DropdownMenuSeparator />
                          {group.events.map((event) => (
                            <DropdownMenuCheckboxItem
                              key={event.value}
                              checked={selectedEvents.includes(event.value)}
                              closeOnClick={false}
                              onCheckedChange={() => toggleEvent(event.value)}
                            >
                              {event.label}
                            </DropdownMenuCheckboxItem>
                          ))}
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
        {filters.length > 0 && (
          <div className="space-y-2">
            {filters.map((filter) => (
              <GitHubFilterRow
                key={filter.id}
                filter={filter}
                onChange={(patch) => updateFilter(filter.id, patch)}
                onRemove={() => removeFilter(filter.id)}
              />
            ))}
          </div>
        )}
        <Button
          type="button"
          variant="outline"
          onClick={addFilter}
          className="w-fit justify-start bg-background font-normal"
        >
          Add a filter condition
        </Button>
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

function buildGitHubEventSummary(selectedEvents: string[]) {
  const allEvents = githubEventGroups.flatMap((group) => group.events);
  if (selectedEvents.length === 0) return "Select GitHub events";
  if (selectedEvents.length <= 2) {
    return selectedEvents
      .map((value) => allEvents.find((event) => event.value === value)?.summary ?? value)
      .join(", ");
  }
  const categoryCount = githubEventGroups.filter((group) =>
    group.events.some((event) => selectedEvents.includes(event.value)),
  ).length;
  return `${selectedEvents.length} events · ${categoryCount} ${categoryCount === 1 ? "category" : "categories"}`;
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
  selectedSubscriberIds,
  selectedLabelIds,
  selectedTriggerTypes,
  scheduleMode,
  runAt,
  hourlyMinute,
  timeOfDay,
  weeklyDay,
  cronExpression,
  githubEventValues,
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
  selectedSubscriberIds: string[];
  selectedLabelIds: string[];
  selectedTriggerTypes: TriggerType[];
  scheduleMode: ScheduleMode;
  runAt: string;
  hourlyMinute: string;
  timeOfDay: string;
  weeklyDay: string;
  cronExpression: string;
  githubEventValues: string[];
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
    enabled,
    subscriber_ids: selectedSubscriberIds,
    label_ids: selectedLabelIds,
    triggers: selectedTriggerTypes.map((triggerType) =>
      buildRoutineTrigger({
        triggerType,
        scheduleMode,
        runAt,
        hourlyMinute,
        timeOfDay,
        weeklyDay,
        cronExpression,
        githubEventValues,
      }),
    ),
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

function buildRoutineTrigger({
  triggerType,
  scheduleMode,
  runAt,
  hourlyMinute,
  timeOfDay,
  weeklyDay,
  cronExpression,
  githubEventValues,
}: {
  triggerType: TriggerType;
  scheduleMode: ScheduleMode;
  runAt: string;
  hourlyMinute: string;
  timeOfDay: string;
  weeklyDay: string;
  cronExpression: string;
  githubEventValues: string[];
}): RoutineTriggerRequest {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  if (triggerType === "schedule") {
    const trigger: RoutineTriggerRequest = {
      trigger_type: "schedule",
      timezone,
      config: { mode: scheduleMode },
      enabled: true,
    };
    if (scheduleMode === "once") {
      trigger.run_at = parseLocalDateTime(runAt)?.toISOString() ?? null;
      return trigger;
    }
    trigger.schedule = scheduleToCron(scheduleMode, hourlyMinute, timeOfDay, weeklyDay, cronExpression);
    return trigger;
  }
  if (triggerType === "github") {
    return {
      trigger_type: "github",
      config: { event_types: githubEventValues },
      enabled: true,
    };
  }
  return {
    trigger_type: "api",
    source_type: "standard",
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

function GitHubFilterRow({
  filter,
  onChange,
  onRemove,
}: {
  filter: GitHubFilterCondition;
  onChange: (patch: Partial<Omit<GitHubFilterCondition, "id">>) => void;
  onRemove: () => void;
}) {
  return (
    <div className="rounded-lg bg-background p-2">
      <div className="flex items-center gap-2">
        <NativeSelect
          value={filter.field}
          onChange={(event) => onChange({ field: event.target.value })}
          className="flex-1"
        >
          {githubFilterFields.map((field) => (
            <NativeSelectOption key={field} value={field}>
              {field}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <NativeSelect
          value={filter.operator}
          onChange={(event) => onChange({ operator: event.target.value })}
          className="flex-1"
        >
          {githubFilterOperators.map((operator) => (
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
      <Input
        value={filter.value}
        onChange={(event) => onChange({ value: event.target.value })}
        placeholder="Comma-separated values"
        className="mt-2 bg-background"
      />
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
  routineID,
  credential,
  regenerating,
  onRegenerate,
}: {
  routineID: string | null;
  credential: ApiTriggerCredential | null;
  regenerating: boolean;
  onRegenerate: () => void;
}) {
  const apiURL = credential
    ? getRoutineTriggerURL(credential.id)
    : "Save the routine to generate an API endpoint";
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
            disabled={!routineID || !credential || regenerating}
          >
            {!routineID || !credential
              ? "Save routine first"
              : regenerating
                ? "Regenerating..."
                : "Regenerate token"}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          Regenerating invalidates the previous token. Existing API URL stays the same.
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
