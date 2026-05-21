"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import Link from "next/link";
import {
  Bot,
  ChevronRight,
  Loader2,
  ArrowDown,
  Brain,
  AlertCircle,
  CheckCircle2,
  XCircle,
  Square,
  Layers,
} from "lucide-react";
import { api } from "@/shared/api";
import { useWSEvent } from "@/features/realtime";
import type {
  TaskMessagePayload,
  TaskCompletedPayload,
  TaskFailedPayload,
  TaskCancelledPayload,
} from "@/shared/types/events";
import type { AgentTask } from "@/shared/types/agent";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { useActorName } from "@/features/workspace";
import { useRuntimeStore } from "@/features/runtimes";
import { redactSecrets } from "../utils/redact";

// ─── Shared types & helpers ─────────────────────────────────────────────────

/** A unified timeline entry: tool calls, thinking, text, and errors in chronological order. */
interface TimelineItem {
  seq: number;
  type: "tool_use" | "tool_result" | "thinking" | "text" | "error";
  tool?: string;
  content?: string;
  input?: Record<string, unknown>;
  output?: string;
}

const ACTIVE_STATUSES: AgentTask["status"][] = ["queued", "dispatched", "running"];

export function isActiveTask(task: AgentTask): boolean {
  return ACTIVE_STATUSES.includes(task.status);
}

function formatElapsed(startedAt: string): string {
  const elapsed = Date.now() - new Date(startedAt).getTime();
  const seconds = Math.floor(elapsed / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${minutes}m ${secs}s`;
}

function formatDuration(start: string, end: string): string {
  const ms = new Date(end).getTime() - new Date(start).getTime();
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${minutes}m ${secs}s`;
}

function shortenPath(p: string): string {
  const parts = p.split("/");
  if (parts.length <= 3) return p;
  return ".../" + parts.slice(-2).join("/");
}

function getToolSummary(item: TimelineItem): string {
  if (!item.input) return "";
  const inp = item.input as Record<string, string>;
  if (inp.query) return inp.query;
  if (inp.file_path) return shortenPath(inp.file_path);
  if (inp.path) return shortenPath(inp.path);
  if (inp.pattern) return inp.pattern;
  if (inp.description) return String(inp.description);
  if (inp.command) {
    const cmd = String(inp.command);
    return cmd.length > 100 ? cmd.slice(0, 100) + "..." : cmd;
  }
  if (inp.prompt) {
    const p = String(inp.prompt);
    return p.length > 100 ? p.slice(0, 100) + "..." : p;
  }
  if (inp.skill) return String(inp.skill);
  for (const v of Object.values(inp)) {
    if (typeof v === "string" && v.length > 0 && v.length < 120) return v;
  }
  return "";
}

/** Build a chronologically ordered timeline from raw messages. */
function buildTimeline(msgs: TaskMessagePayload[]): TimelineItem[] {
  const items: TimelineItem[] = [];
  for (const msg of msgs) {
    items.push({
      seq: msg.seq,
      type: msg.type,
      tool: msg.tool,
      content: msg.content ? redactSecrets(msg.content) : msg.content,
      input: msg.input,
      output: msg.output ? redactSecrets(msg.output) : msg.output,
    });
  }
  return items.sort((a, b) => a.seq - b.seq);
}

// ─── TaskRunCard (inline, used for both active and completed runs) ─────────

interface TaskRunCardProps {
  task: AgentTask;
  /** When provided, used as a fallback agent label if the agent isn't in the workspace store. */
  fallbackAgentName?: string;
}

/**
 * TaskRunCard renders a single agent run inline in the activity stream.
 *
 * - Active runs (queued/dispatched/running) default to expanded with a live
 *   "Agent is working" header and stream timeline messages over the websocket.
 * - Completed/failed/cancelled runs default to collapsed; expanding fetches
 *   the recorded messages on-demand.
 *
 * The wrapping div carries `id="task-run-<task.id>"` so other UI surfaces
 * (e.g. the sidebar working indicator) can scroll to and expand it.
 */
export function TaskRunCard({ task, fallbackAgentName }: TaskRunCardProps) {
  const { getActorName } = useActorName();
  const runtimes = useRuntimeStore((s) => s.runtimes);
  const daemons = useRuntimeStore((s) => s.daemons);

  const active = isActiveTask(task);
  const [open, setOpen] = useState(active);
  const [items, setItems] = useState<TimelineItem[] | null>(active ? [] : null);
  const seenSeqs = useRef<Set<number>>(new Set());
  const [elapsed, setElapsed] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const [cancelling, setCancelling] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Listen for an explicit "expand" request — used by the sidebar working
  // indicator click-to-jump behavior.
  useEffect(() => {
    const onExpand = (e: Event) => {
      const detail = (e as CustomEvent<{ taskId?: string }>).detail;
      if (!detail || detail.taskId !== task.id) return;
      setOpen(true);
    };
    window.addEventListener("multica:expand-task-run", onExpand as EventListener);
    return () => window.removeEventListener("multica:expand-task-run", onExpand as EventListener);
  }, [task.id]);

  // Active tasks: subscribe to live messages and prime with anything already recorded.
  useEffect(() => {
    if (!active) return;
    let cancelled = false;
    seenSeqs.current = new Set();
    api
      .listTaskMessages(task.id)
      .then((msgs) => {
        if (cancelled) return;
        setItems(buildTimeline(msgs));
        for (const m of msgs) seenSeqs.current.add(m.seq);
      })
      .catch(console.error);
    return () => {
      cancelled = true;
    };
  }, [active, task.id]);

  useWSEvent(
    "task:message",
    useCallback(
      (payload: unknown) => {
        if (!active) return;
        const msg = payload as TaskMessagePayload;
        if (msg.task_id !== task.id) return;
        if (seenSeqs.current.has(msg.seq)) return;
        seenSeqs.current.add(msg.seq);
        setItems((prev) => {
          const item: TimelineItem = {
            seq: msg.seq,
            type: msg.type,
            tool: msg.tool,
            content: msg.content,
            input: msg.input,
            output: msg.output,
          };
          const next = [...(prev ?? []), item];
          next.sort((a, b) => a.seq - b.seq);
          return next;
        });
      },
      [active, task.id],
    ),
  );

  // Completed runs: lazy-load messages on first expand.
  useEffect(() => {
    if (active || !open || items !== null) return;
    api
      .listTaskMessages(task.id)
      .then((msgs) => setItems(buildTimeline(msgs)))
      .catch((e) => {
        console.error(e);
        setItems([]);
      });
  }, [active, open, items, task.id]);

  // Live elapsed time for active runs.
  useEffect(() => {
    if (!active) return;
    const ref = task.started_at ?? task.dispatched_at ?? task.created_at;
    setElapsed(formatElapsed(ref));
    const interval = setInterval(() => setElapsed(formatElapsed(ref)), 1000);
    return () => clearInterval(interval);
  }, [active, task.started_at, task.dispatched_at, task.created_at]);

  // Auto-scroll the live timeline to the latest entry.
  useEffect(() => {
    if (!active || !autoScroll || !scrollRef.current) return;
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [items, autoScroll, active]);

  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 40);
  }, []);

  const handleCancel = useCallback(async () => {
    if (!active || cancelling) return;
    setCancelling(true);
    try {
      await api.cancelTask(task.issue_id, task.id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to cancel task");
      setCancelling(false);
    }
  }, [active, cancelling, task.id, task.issue_id]);

  const taskRuntime = runtimes.find((r) => r.id === task.runtime_id);
  const taskDaemon = taskRuntime?.daemon_ref ? daemons.find((d) => d.id === taskRuntime.daemon_ref) : null;
  const runtimeLabel = [
    taskDaemon?.device_name || taskDaemon?.daemon_id,
    taskRuntime?.provider,
  ]
    .filter(Boolean)
    .join(" / ");

  const agentLabel =
    (task.agent_id ? getActorName("agent", task.agent_id) : null) ?? fallbackAgentName ?? "Agent";

  const duration =
    task.started_at && task.completed_at ? formatDuration(task.started_at, task.completed_at) : null;

  const toolCount = (items ?? []).filter((i) => i.type === "tool_use").length;

  // Header layout differs between active and historical runs to keep the
  // active state visually prominent (per the inline-task-runs design).
  return (
    <div
      id={`task-run-${task.id}`}
      data-task-run-id={task.id}
      data-task-run-status={task.status}
      className={cn(
        "rounded-lg border",
        active ? "border-info/20 bg-info/5" : "border-border bg-card",
      )}
    >
      <Collapsible open={open} onOpenChange={setOpen}>
        <div className="flex items-center gap-2 px-3 py-2">
          <CollapsibleTrigger
            // Render the trigger as a flexible expand-button that fills the
            // available row width. Sibling action buttons (Stop, runtime
            // link) live outside the trigger so we never nest a button
            // inside another button.
            className="flex flex-1 items-center gap-2 min-w-0 text-left"
          >
            <ChevronRight
              className={cn(
                "h-3 w-3 shrink-0 text-muted-foreground transition-transform",
                open && "rotate-90",
              )}
            />
            {active ? (
              <span className="flex items-center justify-center h-5 w-5 rounded-full bg-info/10 text-info shrink-0">
                <Bot className="h-3 w-3" />
              </span>
            ) : task.status === "completed" ? (
              <CheckCircle2 className="h-4 w-4 shrink-0 text-success" />
            ) : (
              <XCircle className="h-4 w-4 shrink-0 text-destructive" />
            )}

            {active ? (
              <span className="flex items-center gap-1.5 text-xs font-medium min-w-0">
                <Loader2 className="h-3 w-3 animate-spin text-info shrink-0" />
                <span className="truncate">{agentLabel} is working</span>
              </span>
            ) : (
              <span className="flex items-center gap-1.5 text-xs min-w-0">
                <span className="font-medium shrink-0">{agentLabel}</span>
                <span className="text-muted-foreground shrink-0">
                  {new Date(task.created_at).toLocaleString(undefined, {
                    month: "short",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </span>
                {duration && (
                  <span className="text-muted-foreground shrink-0">· {duration}</span>
                )}
              </span>
            )}
          </CollapsibleTrigger>

          {runtimeLabel && (
            <Link
              href="/daemons"
              className={cn(
                "text-xs hover:underline transition-colors shrink-0 truncate",
                active
                  ? "text-muted-foreground font-normal hover:text-foreground"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {active ? `on ${runtimeLabel}` : runtimeLabel}
            </Link>
          )}

          {active ? (
            <>
              <span className="text-xs text-muted-foreground tabular-nums shrink-0">{elapsed}</span>
              {toolCount > 0 && (
                <span className="text-xs text-muted-foreground shrink-0">
                  {toolCount} tool {toolCount === 1 ? "call" : "calls"}
                </span>
              )}
              <button
                onClick={handleCancel}
                disabled={cancelling}
                className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50 shrink-0"
                title="Stop agent"
              >
                {cancelling ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <Square className="h-3 w-3" />
                )}
                <span>Stop</span>
              </button>
            </>
          ) : (
            <span
              className={cn(
                "capitalize text-xs shrink-0",
                task.status === "completed" && "text-success",
                task.status === "failed" && "text-destructive",
                task.status === "cancelled" && "text-muted-foreground",
              )}
            >
              {task.status}
            </span>
          )}
        </div>

        <CollapsibleContent>
          {active ? (
            (items ?? []).length > 0 && (
              <div
                ref={scrollRef}
                onScroll={handleScroll}
                className="relative max-h-80 overflow-y-auto border-t border-info/10 px-3 py-2 space-y-0.5"
              >
                {(items ?? []).map((item, idx) => (
                  <TimelineRow key={`${item.seq}-${idx}`} item={item} />
                ))}

                {!autoScroll && (
                  <button
                    onClick={() => {
                      if (scrollRef.current) {
                        scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
                        setAutoScroll(true);
                      }
                    }}
                    className="sticky bottom-0 left-1/2 -translate-x-1/2 flex items-center gap-1 rounded-full bg-background border px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground shadow-sm"
                  >
                    <ArrowDown className="h-3 w-3" />
                    Latest
                  </button>
                )}
              </div>
            )
          ) : (
            <div className="ml-5 mb-2 max-h-64 overflow-y-auto rounded border bg-muted/30 px-3 py-2 space-y-0.5 mr-3">
              {task.status === "failed" && task.error && (
                <div className="rounded bg-destructive/10 px-2 py-1.5 text-xs text-destructive mb-1">
                  {task.error}
                </div>
              )}
              {items === null ? (
                <div className="flex items-center gap-2 text-xs text-muted-foreground py-2">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  Loading...
                </div>
              ) : items.length === 0 && !(task.status === "failed" && task.error) ? (
                <p className="text-xs text-muted-foreground py-2">No execution data recorded.</p>
              ) : (
                items.map((item, idx) => <TimelineRow key={`${item.seq}-${idx}`} item={item} />)
              )}
            </div>
          )}
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}

// ─── Shared timeline row rendering ──────────────────────────────────────────

function TimelineRow({ item }: { item: TimelineItem }) {
  switch (item.type) {
    case "tool_use":
      return <ToolCallRow item={item} />;
    case "tool_result":
      return <ToolResultRow item={item} />;
    case "thinking":
      return <ThinkingRow item={item} />;
    case "text":
      return <TextRow item={item} />;
    case "error":
      return <ErrorRow item={item} />;
    default:
      return null;
  }
}

function ToolCallRow({ item }: { item: TimelineItem }) {
  const [open, setOpen] = useState(false);
  const summary = getToolSummary(item);
  const hasInput = item.input && Object.keys(item.input).length > 0;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-center gap-1.5 rounded px-1 -mx-1 py-0.5 text-xs hover:bg-accent/30 transition-colors">
        <ChevronRight
          className={cn(
            "h-3 w-3 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-90",
            !hasInput && "invisible",
          )}
        />
        <span className="font-medium text-foreground shrink-0">{item.tool}</span>
        {summary && <span className="truncate text-muted-foreground">{summary}</span>}
      </CollapsibleTrigger>
      {hasInput && (
        <CollapsibleContent>
          <pre className="ml-[18px] mt-0.5 max-h-32 overflow-auto rounded bg-muted/50 p-2 text-[11px] text-muted-foreground whitespace-pre-wrap break-all">
            {redactSecrets(JSON.stringify(item.input, null, 2))}
          </pre>
        </CollapsibleContent>
      )}
    </Collapsible>
  );
}

function ToolResultRow({ item }: { item: TimelineItem }) {
  const [open, setOpen] = useState(false);
  const output = item.output ?? "";
  if (!output) return null;

  const preview = output.length > 120 ? output.slice(0, 120) + "..." : output;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-start gap-1.5 rounded px-1 -mx-1 py-0.5 text-xs hover:bg-accent/30 transition-colors">
        <ChevronRight
          className={cn("h-3 w-3 shrink-0 text-muted-foreground transition-transform mt-0.5", open && "rotate-90")}
        />
        <span className="text-muted-foreground/70 truncate">
          {item.tool ? `${item.tool} result: ` : "result: "}{preview}
        </span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <pre className="ml-[18px] mt-0.5 max-h-40 overflow-auto rounded bg-muted/50 p-2 text-[11px] text-muted-foreground whitespace-pre-wrap break-all">
          {output.length > 4000 ? output.slice(0, 4000) + "\n... (truncated)" : output}
        </pre>
      </CollapsibleContent>
    </Collapsible>
  );
}

function ThinkingRow({ item }: { item: TimelineItem }) {
  const [open, setOpen] = useState(false);
  const text = item.content ?? "";
  if (!text) return null;

  const preview = text.length > 150 ? text.slice(0, 150) + "..." : text;

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-start gap-1.5 rounded px-1 -mx-1 py-0.5 text-xs hover:bg-accent/30 transition-colors">
        <Brain className="h-3 w-3 shrink-0 text-info/60 mt-0.5" />
        <span className="text-muted-foreground italic truncate">{preview}</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <pre className="ml-[18px] mt-0.5 max-h-40 overflow-auto rounded bg-info/5 p-2 text-[11px] text-muted-foreground whitespace-pre-wrap break-words">
          {text}
        </pre>
      </CollapsibleContent>
    </Collapsible>
  );
}

function TextRow({ item }: { item: TimelineItem }) {
  const text = item.content ?? "";
  if (!text.trim()) return null;
  const lines = text.trim().split("\n").filter(Boolean);
  const last = lines[lines.length - 1] ?? "";
  if (!last) return null;

  return (
    <div className="flex items-start gap-1.5 px-1 -mx-1 py-0.5 text-xs">
      <span className="h-3 w-3 shrink-0" />
      <span className="text-muted-foreground/60 truncate">{last}</span>
    </div>
  );
}

function ErrorRow({ item }: { item: TimelineItem }) {
  return (
    <div className="flex items-start gap-1.5 px-1 -mx-1 py-0.5 text-xs">
      <AlertCircle className="h-3 w-3 shrink-0 text-destructive mt-0.5" />
      <span className="text-destructive">{item.content}</span>
    </div>
  );
}

// ─── Hook: per-issue task runs (live updates via WS) ───────────────────────

export function useIssueTaskRuns(issueId: string): AgentTask[] {
  const [tasks, setTasks] = useState<AgentTask[]>([]);

  const refresh = useCallback(() => {
    api.listTasksByIssue(issueId).then(setTasks).catch(console.error);
  }, [issueId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useWSEvent(
    "task:dispatch",
    useCallback(
      (payload: unknown) => {
        const p = payload as { issue_id?: string };
        if (p?.issue_id !== issueId) return;
        refresh();
      },
      [issueId, refresh],
    ),
  );

  useWSEvent(
    "task:completed",
    useCallback(
      (payload: unknown) => {
        const p = payload as TaskCompletedPayload;
        if (p?.issue_id !== issueId) return;
        refresh();
      },
      [issueId, refresh],
    ),
  );

  useWSEvent(
    "task:failed",
    useCallback(
      (payload: unknown) => {
        const p = payload as TaskFailedPayload;
        if (p?.issue_id !== issueId) return;
        refresh();
      },
      [issueId, refresh],
    ),
  );

  useWSEvent(
    "task:cancelled",
    useCallback(
      (payload: unknown) => {
        const p = payload as TaskCancelledPayload;
        if (p?.issue_id !== issueId) return;
        refresh();
      },
      [issueId, refresh],
    ),
  );

  return tasks;
}

/**
 * Group tasks by their trigger (issue itself vs. specific comment).
 * Ordering within each bucket is chronological (oldest first).
 */
export function groupTaskRunsByTrigger(tasks: AgentTask[]): {
  issueTriggered: AgentTask[];
  byCommentId: Map<string, AgentTask[]>;
} {
  const issueTriggered: AgentTask[] = [];
  const byCommentId = new Map<string, AgentTask[]>();

  const sorted = [...tasks].sort(
    (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
  );

  for (const t of sorted) {
    if (t.trigger_comment_id) {
      const list = byCommentId.get(t.trigger_comment_id) ?? [];
      list.push(t);
      byCommentId.set(t.trigger_comment_id, list);
    } else {
      issueTriggered.push(t);
    }
  }

  return { issueTriggered, byCommentId };
}

/**
 * Partition tasks into the two rendering buckets:
 * - `embedded`: tasks with a `result_comment_id` — render inside the
 *   corresponding agent reply comment. Tasks targeting the same comment
 *   are grouped together (oldest first).
 * - `standalone`: tasks without a `result_comment_id` — render as their
 *   own card in the activity timeline (current behavior; preserves
 *   active-run visibility and orphan tasks that never produced a reply).
 */
export function partitionTaskRunsByResultComment(tasks: AgentTask[]): {
  embedded: Map<string, AgentTask[]>;
  standalone: AgentTask[];
} {
  const embedded = new Map<string, AgentTask[]>();
  const standalone: AgentTask[] = [];

  const sorted = [...tasks].sort(
    (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
  );

  for (const t of sorted) {
    if (t.result_comment_id) {
      const list = embedded.get(t.result_comment_id) ?? [];
      list.push(t);
      embedded.set(t.result_comment_id, list);
    } else {
      standalone.push(t);
    }
  }

  return { embedded, standalone };
}

// ─── RunTraceGroup — multiple runs sharing one reply ───────────────────────

interface RunTraceGroupProps {
  tasks: AgentTask[];
  fallbackAgentName?: string;
}

type GroupStatus = "active" | "all_completed" | "has_failed" | "cancelled" | "mixed";

function resolveGroupStatus(tasks: AgentTask[]): GroupStatus {
  if (tasks.some(isActiveTask)) return "active";
  if (tasks.every((t) => t.status === "completed")) return "all_completed";
  const failed = tasks.filter((t) => t.status === "failed").length;
  const cancelled = tasks.filter((t) => t.status === "cancelled").length;
  if (failed > 0) return "has_failed";
  if (cancelled > 0 && cancelled === tasks.length) return "cancelled";
  return "mixed";
}

function sumDurationMs(tasks: AgentTask[]): number {
  let total = 0;
  for (const t of tasks) {
    if (t.started_at && t.completed_at) {
      total += new Date(t.completed_at).getTime() - new Date(t.started_at).getTime();
    }
  }
  return total;
}

function formatDurationMs(ms: number): string {
  if (ms <= 0) return "—";
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${minutes}m ${secs}s`;
}

/**
 * RunTraceGroup wraps N≥2 task runs that share a single agent reply.
 * Collapsed: a single summary strip (count, total duration, status badge).
 * Expanded: each task renders as its own TaskRunCard, individually
 * collapsible. When N=1 the caller should render a bare TaskRunCard
 * instead — this component does not special-case the singleton path so
 * the wrapper stays predictable.
 */
export function RunTraceGroup({ tasks, fallbackAgentName }: RunTraceGroupProps) {
  const [open, setOpen] = useState(false);
  const status = resolveGroupStatus(tasks);
  const totalMs = sumDurationMs(tasks);
  const runtimes = useRuntimeStore((s) => s.runtimes);
  const daemons = useRuntimeStore((s) => s.daemons);

  // Show runtime label only when every task in the group shares one — a
  // mixed-runtime group hides it to avoid lying.
  const runtimeIds = new Set(tasks.map((t) => t.runtime_id));
  let runtimeLabel = "";
  if (runtimeIds.size === 1) {
    const runtime = runtimes.find((r) => r.id === tasks[0]!.runtime_id);
    const daemon = runtime?.daemon_ref ? daemons.find((d) => d.id === runtime.daemon_ref) : null;
    runtimeLabel = [daemon?.device_name || daemon?.daemon_id, runtime?.provider]
      .filter(Boolean)
      .join(" / ");
  }

  const failedCount = tasks.filter((t) => t.status === "failed").length;
  const activeCount = tasks.filter(isActiveTask).length;

  return (
    <div
      data-testid="run-trace-group"
      data-run-count={tasks.length}
      data-run-status={status}
      className={cn(
        "rounded-md border bg-muted/30",
        status === "active" && "border-info/20 bg-info/5",
      )}
    >
      <Collapsible open={open} onOpenChange={setOpen}>
        <CollapsibleTrigger className="flex w-full items-center gap-2 px-3 py-2 text-left">
          <ChevronRight
            className={cn(
              "h-3 w-3 shrink-0 text-muted-foreground transition-transform",
              open && "rotate-90",
            )}
          />
          <Layers className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <span className="text-xs font-medium shrink-0">{tasks.length} runs</span>
          {totalMs > 0 && (
            <span className="text-xs text-muted-foreground shrink-0">
              · {formatDurationMs(totalMs)} total
            </span>
          )}
          {runtimeLabel && (
            <span className="text-xs text-muted-foreground shrink-0 truncate">
              · {runtimeLabel}
            </span>
          )}
          <span className="ml-auto flex items-center gap-1 shrink-0 text-xs">
            {status === "active" && (
              <>
                <Loader2 className="h-3 w-3 animate-spin text-info" />
                <span className="text-info font-medium">{activeCount} running</span>
              </>
            )}
            {status === "all_completed" && (
              <>
                <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                <span className="text-success font-medium">All completed</span>
              </>
            )}
            {status === "has_failed" && (
              <>
                <XCircle className="h-3.5 w-3.5 text-destructive" />
                <span className="text-destructive font-medium">{failedCount} failed</span>
              </>
            )}
            {status === "cancelled" && (
              <span className="text-muted-foreground font-medium">Cancelled</span>
            )}
            {status === "mixed" && (
              <span className="text-muted-foreground font-medium">Mixed</span>
            )}
          </span>
        </CollapsibleTrigger>

        <CollapsibleContent>
          <div className="flex flex-col gap-1.5 border-t border-border/50 p-2">
            {tasks.map((task, idx) => (
              <div
                key={task.id}
                className="flex items-center gap-1.5"
                data-testid="run-trace-group-item"
              >
                <span className="shrink-0 w-10 text-[10px] uppercase tracking-wide text-muted-foreground tabular-nums">
                  Run {idx + 1}
                </span>
                <div className="flex-1 min-w-0">
                  <TaskRunCard task={task} fallbackAgentName={fallbackAgentName} />
                </div>
              </div>
            ))}
          </div>
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}

/** Programmatic helper: scroll to and expand a task run card. */
export function scrollToTaskRun(taskId: string) {
  const el = document.getElementById(`task-run-${taskId}`);
  if (!el) return;
  window.dispatchEvent(
    new CustomEvent("multica:expand-task-run", { detail: { taskId } }),
  );
  // Allow the expansion to start before scrolling so the card lands centered.
  requestAnimationFrame(() => {
    el.scrollIntoView({ behavior: "smooth", block: "center" });
  });
}
