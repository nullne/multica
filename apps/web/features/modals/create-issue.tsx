"use client";

import { useState, useRef, useEffect } from "react";
import { useRouter } from "next/navigation";
import { CalendarDays, Check, ChevronRight, Monitor, Maximize2, Minimize2, ShieldCheck, ShieldOff, Tag, X as XIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type { IssueStatus, IssuePriority, IssueAssigneeType, Label, UpdateIssueRequest } from "@/shared/types";
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import { Calendar } from "@/components/ui/calendar";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { Button } from "@/components/ui/button";
import { RichTextEditor, type RichTextEditorRef } from "@/components/common/rich-text-editor";
import { TitleEditor } from "@/components/common/title-editor";
import { StatusIcon, PriorityIcon, AssigneePicker, canAssignAgent } from "@/features/issues/components";
import { useRuntimeStore } from "@/features/runtimes";
import { ALL_STATUSES, STATUS_CONFIG, PRIORITY_ORDER, PRIORITY_CONFIG } from "@/features/issues/config";
import { useLabelStore } from "@/features/labels";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { useIssueStore } from "@/features/issues";
import { useIssueDraftStore } from "@/features/issues/stores/draft-store";
import { useIssueDefaultsStore } from "@/features/issues/stores/defaults-store";
import { issueUrl } from "@/features/issues/utils/url";
import { getAgentDispatchDefaults } from "@/features/issues/utils/dispatch";
import { api } from "@/shared/api";
import { useFileUpload } from "@/shared/hooks/use-file-upload";
import { FileUploadButton } from "@/components/common/file-upload-button";
import { ActorAvatar } from "@/components/common/actor-avatar";

// ---------------------------------------------------------------------------
// Pill trigger — shared rounded-full button style for toolbar
// ---------------------------------------------------------------------------

function PillButton({
  children,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs",
        "hover:bg-accent/60 transition-colors cursor-pointer",
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
}

function DispatchDaemonLabel({ daemonId }: { daemonId: string }) {
  const daemons = useRuntimeStore((s) => s.daemons);
  const daemon = daemons.find((d) => d.id === daemonId);
  if (!daemon) return null;
  return <span className="truncate">{daemon.device_name || daemon.daemon_id}</span>;
}

// ---------------------------------------------------------------------------
// CreateIssueModal
// ---------------------------------------------------------------------------

export function CreateIssueModal({ onClose, data }: { onClose: () => void; data?: Record<string, unknown> | null }) {
  const router = useRouter();
  const workspaceName = useWorkspaceStore((s) => s.workspace?.name);
  const workspaceSlug = useWorkspaceStore((s) => s.workspace?.slug ?? "");
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);
  const { getActorName } = useActorName();

  const draft = useIssueDraftStore((s) => s.draft);
  const setDraft = useIssueDraftStore((s) => s.setDraft);
  const clearDraft = useIssueDraftStore((s) => s.clearDraft);

  const [title, setTitle] = useState(draft.title);
  const descEditorRef = useRef<RichTextEditorRef>(null);
  const [status, setStatus] = useState<IssueStatus>((data?.status as IssueStatus) || draft.status);
  const [priority, setPriority] = useState<IssuePriority>(draft.priority);
  const [parentIssueId] = useState<string | undefined>(data?.parent_issue_id as string | undefined);
  const [submitting, setSubmitting] = useState(false);

  const parentIssue = useIssueStore((s) => s.issues.find((i) => i.id === parentIssueId));

  const defaults = useIssueDefaultsStore.getState();
  const initAssigneeType = draft.assigneeType ?? defaults.assigneeType;
  const initAssigneeId = draft.assigneeId ?? defaults.assigneeId;

  const [assigneeType, setAssigneeType] = useState<IssueAssigneeType | undefined>(initAssigneeType);
  const [assigneeId, setAssigneeId] = useState<string | undefined>(initAssigneeId);
  const [dueDate, setDueDate] = useState<string | null>(draft.dueDate);
  const [verifierAgentId, setVerifierAgentId] = useState<string | undefined>(draft.verifierAgentId);
  const [maxVerificationRounds, setMaxVerificationRounds] = useState<number | undefined>(draft.maxVerificationRounds);
  const [isExpanded, setIsExpanded] = useState(false);

  // Labels
  const [selectedLabels, setSelectedLabels] = useState<Label[]>([]);
  const allLabels = useLabelStore((s) => s.labels);
  const [labelOpen, setLabelOpen] = useState(false);
  const [labelFilter, setLabelFilter] = useState("");

  const initDispatch = (() => {
    if (initAssigneeType !== "agent" || !initAssigneeId) return {};
    const agent = agents.find((a) => a.id === initAssigneeId);
    if (!agent) return {};
    const saved = defaults.getAgentDispatch(initAssigneeId);
    const currentRuntimes = useRuntimeStore.getState().runtimes;
    const dispatch = getAgentDispatchDefaults(agent, currentRuntimes, saved);
    return {
      daemonId: dispatch.daemonId ?? undefined,
      provider: dispatch.provider ?? undefined,
    };
  })();
  const [dispatchProvider, setDispatchProvider] = useState<string | undefined>(initDispatch.provider);
  const [dispatchDaemonId, setDispatchDaemonId] = useState<string | undefined>(initDispatch.daemonId);
  const fetchAllRuntimes = useRuntimeStore((s) => s.fetchAll);

  const selectedAgent = assigneeType === "agent" && assigneeId
    ? agents.find((a) => a.id === assigneeId)
    : null;

  useEffect(() => {
    if (selectedAgent) fetchAllRuntimes();
  }, [selectedAgent, fetchAllRuntimes]);

  useEffect(() => {
    const t = setTimeout(() => descEditorRef.current?.focus(), 0);
    return () => clearTimeout(t);
  }, []);

  // Verifier popover
  const [verifierOpen, setVerifierOpen] = useState(false);
  const [verifierFilter, setVerifierFilter] = useState("");

  // Due date popover
  const [dueDateOpen, setDueDateOpen] = useState(false);

  // File upload
  const { uploadWithToast, uploading } = useFileUpload();
  const handleUpload = (file: File) => uploadWithToast(file);

  const user = useAuthStore((s) => s.user);
  const currentMember = members.find((m) => m.user_id === user?.id);
  const memberRole = currentMember?.role;

  const verifierQuery = verifierFilter.toLowerCase();
  const filteredVerifierAgents = agents.filter(
    (a) => !a.archived_at && a.name.toLowerCase().includes(verifierQuery) && a.id !== assigneeId,
  );

  const assigneeLabel =
    assigneeType && assigneeId
      ? getActorName(assigneeType, assigneeId)
      : "Assignee";

  const verifierLabel = verifierAgentId
    ? getActorName("agent", verifierAgentId)
    : "Verifier";

  const dueDateObj = dueDate ? new Date(dueDate) : undefined;

  // Sync field changes to draft store
  const updateTitle = (v: string) => { setTitle(v); setDraft({ title: v }); };
  const updateStatus = (v: IssueStatus) => { setStatus(v); setDraft({ status: v }); };
  const updatePriority = (v: IssuePriority) => { setPriority(v); setDraft({ priority: v }); };
  const applyAssigneePatch = (patch: Partial<UpdateIssueRequest>) => {
    const newType = (patch.assignee_type ?? undefined) as IssueAssigneeType | undefined;
    const newId = patch.assignee_id ?? undefined;
    setAssigneeType(newType);
    setAssigneeId(newId);
    setDispatchDaemonId(patch.dispatch_daemon_id ?? undefined);
    setDispatchProvider(patch.dispatch_provider ?? undefined);
    setDraft({ assigneeType: newType, assigneeId: newId });

    const draftUpdate: Parameters<typeof setDraft>[0] = {};
    if ("verifier_agent_id" in patch) {
      const v = patch.verifier_agent_id ?? undefined;
      setVerifierAgentId(v);
      draftUpdate.verifierAgentId = v;
    }
    if ("max_verification_rounds" in patch) {
      const m = patch.max_verification_rounds ?? undefined;
      setMaxVerificationRounds(m);
      draftUpdate.maxVerificationRounds = m;
    }
    if (Object.keys(draftUpdate).length > 0) setDraft(draftUpdate);
  };
  const updateDueDate = (v: string | null) => { setDueDate(v); setDraft({ dueDate: v }); };
  const updateVerifier = (id?: string) => { setVerifierAgentId(id); setDraft({ verifierAgentId: id }); };
  const updateMaxRounds = (v?: number) => { setMaxVerificationRounds(v); setDraft({ maxVerificationRounds: v }); };

  const handleSubmit = async () => {
    if (submitting) return;
    setSubmitting(true);
    try {
      let issue = await api.createIssue({
        title: title.trim(),
        description: descEditorRef.current?.getMarkdown()?.trim() || undefined,
        status,
        priority,
        assignee_type: assigneeType,
        assignee_id: assigneeId,
        verifier_agent_id: verifierAgentId,
        max_verification_rounds: maxVerificationRounds,
        due_date: dueDate || undefined,
        dispatch_provider: dispatchProvider,
        dispatch_daemon_id: dispatchDaemonId,
        parent_issue_id: parentIssueId,
      });
      if (selectedLabels.length > 0) {
        try {
          const labels = await api.setIssueLabels(issue.id, selectedLabels.map((l) => l.id));
          issue = { ...issue, labels };
        } catch {
          // Labels failed but issue was created — not fatal.
        }
      }
      useIssueStore.getState().addIssue(issue);
      clearDraft();
      useIssueDefaultsStore.getState().setAssigneeDefaults(assigneeType, assigneeId);
      if (assigneeType === "agent" && assigneeId) {
        useIssueDefaultsStore.getState().setAgentDispatch(assigneeId, {
          daemonId: dispatchDaemonId,
          provider: dispatchProvider,
        });
      }
      onClose();
      toast.custom((t) => (
        <div className="bg-popover text-popover-foreground border rounded-lg shadow-lg p-4 w-[360px]">
          <div className="flex items-center gap-2 mb-2">
            <div className="flex items-center justify-center size-5 rounded-full bg-emerald-500/15 text-emerald-500">
              <Check className="size-3" />
            </div>
            <span className="text-sm font-medium">Issue created</span>
          </div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground ml-7">
            <StatusIcon status={issue.status} className="size-3.5 shrink-0" />
            <span className="truncate">{issue.identifier} – {issue.title}</span>
          </div>
          <button
            type="button"
            className="ml-7 mt-2 text-sm text-primary hover:underline cursor-pointer"
            onClick={() => {
              router.push(issueUrl(issue.id, workspaceSlug));
              toast.dismiss(t);
            }}
          >
            View issue
          </button>
        </div>
      ), { duration: 5000 });
    } catch {
      toast.error("Failed to create issue");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent
        showCloseButton={false}
        className={cn(
          "p-0 gap-0 flex flex-col overflow-hidden",
          "!top-1/2 !left-1/2 !-translate-x-1/2",
          "!transition-all !duration-300 !ease-out",
          isExpanded
            ? "!max-w-4xl !w-full !h-5/6 !-translate-y-1/2"
            : "!max-w-2xl !w-full !h-96 !-translate-y-1/2",
        )}
      >
        <DialogTitle className="sr-only">New Issue</DialogTitle>

        {/* Header */}
        <div className="flex items-center justify-between px-5 pt-3 pb-2 shrink-0">
          <div className="flex items-center gap-1.5 text-xs">
            <span className="text-muted-foreground">{workspaceName}</span>
            {parentIssue && (
              <>
                <ChevronRight className="size-3 text-muted-foreground/50" />
                <span className="text-muted-foreground">{parentIssue.identifier}</span>
              </>
            )}
            <ChevronRight className="size-3 text-muted-foreground/50" />
            <span className="font-medium">{parentIssue ? "New sub-issue" : "New issue"}</span>
          </div>
          <div className="flex items-center gap-1">
            <Tooltip>
              <TooltipTrigger
                render={
                  <button
                    onClick={() => setIsExpanded(!isExpanded)}
                    className="rounded-sm p-1.5 opacity-70 hover:opacity-100 hover:bg-accent/60 transition-all cursor-pointer"
                  >
                    {isExpanded ? <Minimize2 className="size-4" /> : <Maximize2 className="size-4" />}
                  </button>
                }
              />
              <TooltipContent side="bottom">{isExpanded ? "Collapse" : "Expand"}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger
                render={
                  <button
                    onClick={onClose}
                    className="rounded-sm p-1.5 opacity-70 hover:opacity-100 hover:bg-accent/60 transition-all cursor-pointer"
                  >
                    <XIcon className="size-4" />
                  </button>
                }
              />
              <TooltipContent side="bottom">Close</TooltipContent>
            </Tooltip>
          </div>
        </div>

        {/* Title */}
        <div className="px-5 pb-2 shrink-0">
          <TitleEditor
            defaultValue={draft.title}
            placeholder="Issue title"
            className="text-lg font-semibold"
            onChange={(v) => updateTitle(v)}
            onSubmit={handleSubmit}
          />
        </div>

        {/* Description — takes remaining space */}
        <div className="flex-1 min-h-0 overflow-y-auto px-5">
          <RichTextEditor
            ref={descEditorRef}
            defaultValue={draft.description}
            placeholder="Add description..."
            onUpdate={(md) => setDraft({ description: md })}
            onUploadFile={handleUpload}
            debounceMs={500}
          />
        </div>

        {/* Bottom bar */}
        <div className="shrink-0">
          {/* Row 1: Assignee + agent dispatch options + Create button */}
          <div className="flex items-center justify-between px-4 py-2">
            <div className="flex items-center gap-1.5">
              {/* Assignee — uses the shared picker so the agent dispatch
                  confirmation step matches the issue detail entry points.
                  Provider/environment render as secondary text alongside the
                  agent name to keep this row uncluttered. */}
              <AssigneePicker
                assigneeType={assigneeType ?? null}
                assigneeId={assigneeId ?? null}
                verifierAgentId={verifierAgentId ?? null}
                onUpdate={applyAssigneePatch}
                align="start"
                triggerRender={<PillButton />}
                trigger={
                  assigneeType && assigneeId ? (
                    <>
                      <ActorAvatar actorType={assigneeType} actorId={assigneeId} size={16} />
                      <span className="truncate">{assigneeLabel}</span>
                      {selectedAgent && (dispatchProvider || dispatchDaemonId) && (
                        <span className="flex items-center gap-1 text-[10px] text-muted-foreground min-w-0">
                          <span className="text-muted-foreground/70">·</span>
                          {dispatchProvider && (
                            <span className="capitalize shrink-0">{dispatchProvider}</span>
                          )}
                          {dispatchProvider && dispatchDaemonId && (
                            <span className="shrink-0">·</span>
                          )}
                          {dispatchDaemonId && (
                            <span className="inline-flex items-center gap-0.5 min-w-0">
                              <Monitor className="h-2.5 w-2.5 shrink-0" />
                              <DispatchDaemonLabel daemonId={dispatchDaemonId} />
                            </span>
                          )}
                        </span>
                      )}
                    </>
                  ) : (
                    <span className="text-muted-foreground">Assignee</span>
                  )
                }
              />

              {/* Agent-only fields beyond dispatch (verifier) */}
              {selectedAgent && (
                <>
                  {/* Verifier */}
                  <Popover open={verifierOpen} onOpenChange={(v) => { setVerifierOpen(v); if (!v) setVerifierFilter(""); }}>
                    <PopoverTrigger
                      render={
                        <PillButton>
                          {verifierAgentId ? (
                            <>
                              <ShieldCheck className="size-3.5 text-primary" />
                              <span>{verifierLabel}</span>
                            </>
                          ) : (
                            <>
                              <ShieldOff className="size-3.5 text-muted-foreground" />
                              <span className="text-muted-foreground">Verifier</span>
                            </>
                          )}
                        </PillButton>
                      }
                    />
                    <PopoverContent align="start" className="w-52 p-0">
                      <div className="px-2 py-1.5 border-b">
                        <input
                          type="text"
                          value={verifierFilter}
                          onChange={(e) => setVerifierFilter(e.target.value)}
                          placeholder="Select verifier..."
                          className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none"
                        />
                      </div>
                      <div className="p-1 max-h-60 overflow-y-auto">
                        <button
                          type="button"
                          onClick={() => {
                            updateVerifier(undefined);
                            updateMaxRounds(undefined);
                            setVerifierOpen(false);
                          }}
                          className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
                        >
                          <ShieldOff className="h-3.5 w-3.5 text-muted-foreground" />
                          <span className="text-muted-foreground">No verifier</span>
                        </button>

                        {filteredVerifierAgents.length > 0 && (
                          <>
                            <div className="px-2 pt-2 pb-1 text-xs font-medium text-muted-foreground uppercase tracking-wider">Agents</div>
                            {filteredVerifierAgents.map((a) => {
                              const allowed = canAssignAgent(a, user?.id, memberRole);
                              return (
                                <button
                                  type="button"
                                  key={a.id}
                                  disabled={!allowed}
                                  onClick={() => {
                                    if (!allowed) return;
                                    updateVerifier(a.id);
                                    setVerifierOpen(false);
                                  }}
                                  className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors ${allowed ? "hover:bg-accent" : "opacity-50 cursor-not-allowed"}`}
                                >
                                  <ActorAvatar actorType="agent" actorId={a.id} size={16} />
                                  <span>{a.name}</span>
                                </button>
                              );
                            })}
                          </>
                        )}

                        {filteredVerifierAgents.length === 0 && verifierFilter && (
                          <div className="px-2 py-3 text-center text-sm text-muted-foreground">No results</div>
                        )}
                      </div>

                      {verifierAgentId && (
                        <div className="border-t px-3 py-2">
                          <label className="flex items-center justify-between text-xs text-muted-foreground">
                            <span>Max rounds</span>
                            <input
                              type="number"
                              min={1}
                              max={20}
                              value={maxVerificationRounds ?? ""}
                              placeholder="5"
                              onChange={(e) => {
                                const v = e.target.value ? parseInt(e.target.value, 10) : undefined;
                                updateMaxRounds(v && v > 0 ? v : undefined);
                              }}
                              className="w-14 rounded border px-1.5 py-0.5 text-xs text-right bg-transparent outline-none focus:ring-1 focus:ring-ring"
                            />
                          </label>
                        </div>
                      )}
                    </PopoverContent>
                  </Popover>
                </>
              )}
            </div>

            <Button size="sm" onClick={handleSubmit} disabled={submitting}>
              {submitting ? "Creating..." : "Create Issue"}
            </Button>
          </div>

          {/* Row 2: Status, Priority, Labels, Due date, File upload — icon-only, show value when set */}
          <div className="flex items-center gap-1 px-4 py-1.5 border-t">
            {/* Status */}
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <PillButton className="border-none px-1.5">
                    <StatusIcon status={status} className="size-4" />
                  </PillButton>
                }
              />
              <DropdownMenuContent align="start" className="w-44">
                {ALL_STATUSES.map((s) => (
                  <DropdownMenuItem key={s} onClick={() => updateStatus(s)}>
                    <StatusIcon status={s} className="size-3.5" />
                    <span>{STATUS_CONFIG[s].label}</span>
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>

            {/* Priority */}
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <PillButton className={cn("border-none px-1.5", priority !== "none" && "px-2.5")}>
                    <PriorityIcon priority={priority} className="size-4" />
                    {priority !== "none" && <span>{PRIORITY_CONFIG[priority].label}</span>}
                  </PillButton>
                }
              />
              <DropdownMenuContent align="start" className="w-44">
                {PRIORITY_ORDER.map((p) => (
                  <DropdownMenuItem key={p} onClick={() => updatePriority(p)}>
                    <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${PRIORITY_CONFIG[p].badgeBg} ${PRIORITY_CONFIG[p].badgeText}`}>
                      <PriorityIcon priority={p} className="h-3 w-3" inheritColor />
                      {PRIORITY_CONFIG[p].label}
                    </span>
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>

            {/* Labels */}
            <Popover open={labelOpen} onOpenChange={(v) => { setLabelOpen(v); if (!v) setLabelFilter(""); }}>
              <PopoverTrigger
                render={
                  <PillButton className={cn("border-none", selectedLabels.length > 0 ? "px-2.5" : "px-1.5")}>
                    {selectedLabels.length > 0 ? (
                      <div className="flex items-center gap-1 overflow-hidden">
                        {selectedLabels.slice(0, 2).map((l) => (
                          <span
                            key={l.id}
                            className="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs truncate"
                            style={{ backgroundColor: l.color + "15" }}
                          >
                            <span className="h-2 w-2 rounded-full shrink-0" style={{ backgroundColor: l.color }} />
                            <span className="truncate max-w-[60px]">{l.name}</span>
                          </span>
                        ))}
                        {selectedLabels.length > 2 && (
                          <span className="text-muted-foreground">+{selectedLabels.length - 2}</span>
                        )}
                      </div>
                    ) : (
                      <Tag className="size-4 text-muted-foreground" />
                    )}
                  </PillButton>
                }
              />
              <PopoverContent align="start" className="w-52 p-0">
                <div className="px-2 py-1.5 border-b">
                  <input
                    type="text"
                    value={labelFilter}
                    onChange={(e) => setLabelFilter(e.target.value)}
                    placeholder="Filter labels..."
                    className="w-full bg-transparent text-sm placeholder:text-muted-foreground outline-none"
                  />
                </div>
                <div className="p-1 max-h-60 overflow-y-auto">
                  {allLabels
                    .filter((l) => l.name.toLowerCase().includes(labelFilter.toLowerCase()))
                    .map((label) => (
                      <button
                        key={label.id}
                        type="button"
                        onClick={() => {
                          setSelectedLabels((prev) =>
                            prev.some((l) => l.id === label.id)
                              ? prev.filter((l) => l.id !== label.id)
                              : [...prev, label],
                          );
                        }}
                        className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
                      >
                        <span className="h-3 w-3 rounded-full shrink-0" style={{ backgroundColor: label.color }} />
                        <span className="flex-1 text-left truncate">{label.name}</span>
                        {selectedLabels.some((l) => l.id === label.id) && (
                          <Check className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                        )}
                      </button>
                    ))}
                  {allLabels.length === 0 && (
                    <div className="px-2 py-3 text-center text-sm text-muted-foreground">No labels yet</div>
                  )}
                </div>
              </PopoverContent>
            </Popover>

            {/* Due date */}
            <Popover open={dueDateOpen} onOpenChange={setDueDateOpen}>
              <PopoverTrigger
                render={
                  <PillButton className={cn("border-none", dueDateObj ? "px-2.5" : "px-1.5")}>
                    <CalendarDays className="size-4 text-muted-foreground" />
                    {dueDateObj && (
                      <span>{dueDateObj.toLocaleDateString("en-US", { month: "short", day: "numeric" })}</span>
                    )}
                  </PillButton>
                }
              />
              <PopoverContent className="w-auto p-0" align="start">
                <Calendar
                  mode="single"
                  selected={dueDateObj}
                  onSelect={(d: Date | undefined) => {
                    updateDueDate(d ? d.toISOString() : null);
                    setDueDateOpen(false);
                  }}
                />
                {dueDateObj && (
                  <div className="border-t px-3 py-2">
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => {
                        updateDueDate(null);
                        setDueDateOpen(false);
                      }}
                      className="text-muted-foreground hover:text-foreground"
                    >
                      Clear date
                    </Button>
                  </div>
                )}
              </PopoverContent>
            </Popover>

            <FileUploadButton
              busy={uploading}
              onSelect={(file) => descEditorRef.current?.uploadFile(file)}
            />
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
