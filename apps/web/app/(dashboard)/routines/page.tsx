"use client";

import { useMemo, useState } from "react";
import {
  AlertTriangle,
  Bell,
  CalendarClock,
  Check,
  ChevronDown,
  Clock,
  Code2,
  GitBranch as Github,
  Plus,
  Save,
  Tag,
  Users,
  X,
} from "lucide-react";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
import { AssigneePicker } from "@/features/issues/components/pickers";
import { useLabelStore } from "@/features/labels";
import { useActorName, useWorkspaceStore } from "@/features/workspace";
import type { IssueAssigneeType, IssuePriority, UpdateIssueRequest } from "@/shared/types";

type TriggerType = "schedule" | "github" | "api";
type ScheduleMode = "once" | "hourly" | "daily" | "weekdays" | "weekly" | "custom";

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

const priorityOptions: { value: IssuePriority; label: string }[] = [
  { value: "urgent", label: "Urgent" },
  { value: "high", label: "High" },
  { value: "medium", label: "Medium" },
  { value: "low", label: "Low" },
  { value: "none", label: "None" },
];

const scheduleModes: { value: ScheduleMode; label: string }[] = [
  { value: "once", label: "Once" },
  { value: "hourly", label: "Hourly" },
  { value: "daily", label: "Daily" },
  { value: "weekdays", label: "Weekdays" },
  { value: "weekly", label: "Weekly" },
  { value: "custom", label: "Custom" },
];

const githubEventGroups: {
  label: string;
  events: { value: string; label: string; summary: string }[];
}[] = [
  {
    label: "Pull requests",
    events: [
      { value: "github.pull_request.opened", label: "Opened", summary: "PR opened" },
      { value: "github.pull_request.synchronize", label: "Synchronize", summary: "PR synchronized" },
      { value: "github.pull_request.reopened", label: "Reopened", summary: "PR reopened" },
      { value: "github.pull_request.closed", label: "Closed (unmerged)", summary: "PR closed" },
      { value: "github.pull_request.merged", label: "Merged", summary: "PR merged" },
      { value: "github.pull_request.labeled", label: "Labeled", summary: "PR labeled" },
    ],
  },
  {
    label: "Issues",
    events: [
      { value: "github.issues.opened", label: "Opened", summary: "Issue opened" },
      { value: "github.issues.reopened", label: "Reopened", summary: "Issue reopened" },
      { value: "github.issues.closed", label: "Closed", summary: "Issue closed" },
      { value: "github.issues.labeled", label: "Labeled", summary: "Issue labeled" },
    ],
  },
  {
    label: "Push",
    events: [
      { value: "github.push", label: "Push", summary: "Push" },
    ],
  },
  {
    label: "Comments",
    events: [
      { value: "github.issue_comment.created", label: "Created", summary: "Issue comment created" },
    ],
  },
];

const weekDays = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"];

const githubFilterFields = [
  "Author",
  "Title",
  "Body",
  "Base branch",
  "Head branch",
  "Labels",
  "Is draft",
  "Is merged",
] as const;

const githubFilterOperators = [
  "is one of",
  "is not one of",
  "contains",
  "does not contain",
  "is",
  "is not",
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
  const members = useWorkspaceStore((s) => s.members);
  const labels = useLabelStore((s) => s.labels);
  const { getActorName } = useActorName();

  const [name, setName] = useState("");
  const [instructions, setInstructions] = useState("");
  const [priority, setPriority] = useState<IssuePriority>("medium");
  const [assigneeType, setAssigneeType] = useState<IssueAssigneeType | null>(null);
  const [assigneeId, setAssigneeId] = useState<string | null>(null);
  const [dispatchProvider, setDispatchProvider] = useState<string | null>(null);
  const [dispatchDaemonId, setDispatchDaemonId] = useState<string | null>(null);
  const [dispatchDaemonLabel, setDispatchDaemonLabel] = useState<string | null>(null);
  const [selectedSubscriberIds, setSelectedSubscriberIds] = useState<string[]>([]);
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

  const assigneeLabel =
    assigneeType && assigneeId ? getActorName(assigneeType, assigneeId) : "Unassigned";

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

  return (
    <main
      data-testid="routines-page-scroll"
      className="h-full min-h-0 overflow-y-auto bg-background"
    >
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-4 px-6 py-4">
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
                  <Select value={priority} onValueChange={(value) => setPriority(value as IssuePriority)}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {priorityOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
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
                      {selectedSubscriberIds.length === 0
                        ? "No subscribers"
                        : `${selectedSubscriberIds.length} subscribers`}
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
                        ? "Token will be generated when you save."
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
                      <PreviewPanel
                        title="API endpoint"
                        description="A signed POST endpoint will trigger this routine from CI, scripts, or other internal systems."
                        action="Generate endpoint after save"
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
          <Button disabled>
            <Save className="size-4" />
            Create routine
          </Button>
        </div>
      </div>
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
  const summary = buildGitHubEventSummary(selectedEvents);

  const toggleEvent = (value: string) => {
    onSelectedEventsChange(
      selectedEvents.includes(value)
        ? selectedEvents.filter((event) => event !== value)
        : [...selectedEvents, value],
    );
  };

  const toggleGroup = (events: { value: string }[]) => {
    const values = events.map((event) => event.value);
    const allSelected = values.every((value) => selectedEvents.includes(value));
    onSelectedEventsChange(
      allSelected
        ? selectedEvents.filter((value) => !values.includes(value))
        : Array.from(new Set([...selectedEvents, ...values])),
    );
  };

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
        <Label className="text-xs text-muted-foreground">Events</Label>
        <div className="grid gap-3 md:grid-cols-2">
          {githubEventGroups.map((group) => (
            <div key={group.label} className="rounded-lg border bg-background p-3">
              <div className="mb-2 flex items-center justify-between gap-2">
                <span className="text-xs font-medium text-muted-foreground">
                  {group.label}
                </span>
                <button
                  type="button"
                  onClick={() => toggleGroup(group.events)}
                  className="text-xs text-muted-foreground hover:text-foreground"
                >
                  {group.events.every((event) => selectedEvents.includes(event.value))
                    ? "Clear"
                    : "All"}
                </button>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {group.events.map((event) => {
                  const selected = selectedEvents.includes(event.value);
                  return (
                    <button
                      key={event.value}
                      type="button"
                      aria-pressed={selected}
                      onClick={() => toggleEvent(event.value)}
                      className="rounded-full border px-2.5 py-1 text-xs font-medium transition-colors hover:bg-accent aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground"
                    >
                      {event.label}
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
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
  if (selectedEvents.length === 0) return "GitHub event";
  if (selectedEvents.length <= 3) {
    return selectedEvents
      .map((value) => allEvents.find((event) => event.value === value)?.summary ?? value)
      .join(", ");
  }
  return `${selectedEvents.length} events selected`;
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
        placeholder="Values"
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
