"use client";

import { useState } from "react";
import {
  CalendarDays,
  Check,
  CornerDownLeft,
  Loader2,
  Monitor,
  ShieldCheck,
  ShieldOff,
  Tag,
  Timer,
} from "lucide-react";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Calendar } from "@/components/ui/calendar";
import { Button } from "@/components/ui/button";
import { FileUploadButton } from "@/components/common/file-upload-button";
import { ActorAvatar } from "@/components/common/actor-avatar";
import type {
  IssueAssigneeType,
  IssuePriority,
  IssueStatus,
  Label,
  UpdateIssueRequest,
} from "@/shared/types";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { useLabelStore } from "@/features/labels";
import { useRuntimeStore } from "@/features/runtimes";
import { formatAbsoluteTime } from "@/shared/utils";
import { canAssignAgent, AssigneePicker } from "./pickers/assignee-picker";
import { DispatchAfterPicker } from "./pickers/dispatch-after-picker";
import { StatusIcon } from "./status-icon";
import { PriorityIcon } from "./priority-icon";
import {
  ALL_STATUSES,
  STATUS_CONFIG,
  PRIORITY_ORDER,
  PRIORITY_CONFIG,
} from "@/features/issues/config";

export function CreateIssuePillButton({
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

export function CreateIssueToolbar({
  assigneeType,
  assigneeId,
  verifierAgentId,
  maxVerificationRounds,
  dispatchProvider,
  dispatchDaemonId,
  status,
  priority,
  selectedLabels,
  dueDate,
  dispatchAfter,
  uploading,
  submitting,
  onAssigneeUpdate,
  onVerifierChange,
  onMaxVerificationRoundsChange,
  onStatusChange,
  onPriorityChange,
  onLabelsChange,
  onDueDateChange,
  onDispatchAfterChange,
  onUploadFile,
  onSubmit,
  submitDisabled,
  submitLabel = "Create Issue",
  submitVariant = "default",
  showAssignee = true,
  hideSubmit = false,
}: {
  assigneeType: IssueAssigneeType | null | undefined;
  assigneeId: string | null | undefined;
  verifierAgentId?: string | null;
  maxVerificationRounds?: number;
  dispatchProvider?: string | null;
  dispatchDaemonId?: string | null;
  status: IssueStatus;
  priority: IssuePriority;
  selectedLabels: Label[];
  dueDate: string | null;
  dispatchAfter?: string | null;
  uploading?: boolean;
  submitting?: boolean;
  onAssigneeUpdate: (patch: Partial<UpdateIssueRequest>) => void;
  onVerifierChange?: (id?: string) => void;
  onMaxVerificationRoundsChange?: (rounds?: number) => void;
  onStatusChange: (status: IssueStatus) => void;
  onPriorityChange: (priority: IssuePriority) => void;
  onLabelsChange: (labels: Label[]) => void;
  onDueDateChange: (value: string | null) => void;
  onDispatchAfterChange?: (value: string | null) => void;
  onUploadFile: (file: File) => void;
  onSubmit: () => void;
  submitDisabled?: boolean;
  submitLabel?: string;
  submitVariant?: "default" | "icon";
  showAssignee?: boolean;
  hideSubmit?: boolean;
}) {
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);
  const allLabels = useLabelStore((s) => s.labels);
  const user = useAuthStore((s) => s.user);
  const { getActorName } = useActorName();

  const [labelOpen, setLabelOpen] = useState(false);
  const [labelFilter, setLabelFilter] = useState("");
  const [dueDateOpen, setDueDateOpen] = useState(false);
  const [verifierOpen, setVerifierOpen] = useState(false);
  const [verifierFilter, setVerifierFilter] = useState("");

  const currentMember = members.find((m) => m.user_id === user?.id);
  const memberRole = currentMember?.role;
  const selectedAgent =
    assigneeType === "agent" && assigneeId
      ? agents.find((a) => a.id === assigneeId)
      : null;
  const assigneeLabel =
    assigneeType && assigneeId ? getActorName(assigneeType, assigneeId) : "Assignee";
  const verifierLabel = verifierAgentId
    ? getActorName("agent", verifierAgentId)
    : "Verifier";
  const dueDateObj = dueDate ? new Date(dueDate) : undefined;
  const verifierQuery = verifierFilter.toLowerCase();
  const filteredVerifierAgents = agents.filter(
    (a) =>
      !a.archived_at &&
      a.name.toLowerCase().includes(verifierQuery) &&
      a.id !== assigneeId,
  );
  const isCompactSubmit = submitVariant === "icon";
  const isSubmitDisabled = submitting || submitDisabled;
  const canShowVerifierPicker = Boolean(
    selectedAgent && onVerifierChange && onMaxVerificationRoundsChange
  );
  const hasTopRow =
    showAssignee || canShowVerifierPicker || (!isCompactSubmit && !hideSubmit);
  const submitButton = isCompactSubmit ? (
    <button
      type="button"
      aria-label={submitting ? "Creating issue" : "Create issue"}
      onClick={onSubmit}
      disabled={isSubmitDisabled}
      className="ml-auto inline-flex h-7 w-7 items-center justify-center rounded-lg border bg-background text-muted-foreground shadow-sm transition-colors hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-45"
    >
      {submitting ? (
        <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
      ) : (
        <CornerDownLeft className="size-3.5" aria-hidden="true" />
      )}
    </button>
  ) : (
    <Button size="sm" onClick={onSubmit} disabled={isSubmitDisabled}>
      {submitting ? "Creating..." : submitLabel}
    </Button>
  );

  return (
    <div className="shrink-0">
      {hasTopRow && (
        <div className="flex items-center justify-between px-4 py-2">
          <div className="flex items-center gap-1.5">
          {showAssignee ? (
            <AssigneePicker
              assigneeType={assigneeType ?? null}
              assigneeId={assigneeId ?? null}
              verifierAgentId={verifierAgentId ?? null}
              onUpdate={onAssigneeUpdate}
              align="start"
              triggerRender={<CreateIssuePillButton aria-label="Assign" />}
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
          ) : null}

          {selectedAgent && onVerifierChange && onMaxVerificationRoundsChange && (
            <Popover
              open={verifierOpen}
              onOpenChange={(v) => {
                setVerifierOpen(v);
                if (!v) setVerifierFilter("");
              }}
            >
              <PopoverTrigger
                render={
                  <CreateIssuePillButton>
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
                  </CreateIssuePillButton>
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
                      onVerifierChange(undefined);
                      onMaxVerificationRoundsChange(undefined);
                      setVerifierOpen(false);
                    }}
                    className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
                  >
                    <ShieldOff className="h-3.5 w-3.5 text-muted-foreground" />
                    <span className="text-muted-foreground">No verifier</span>
                  </button>
                  {filteredVerifierAgents.length > 0 && (
                    <>
                      <div className="px-2 pt-2 pb-1 text-xs font-medium text-muted-foreground uppercase tracking-wider">
                        Agents
                      </div>
                      {filteredVerifierAgents.map((agent) => {
                        const allowed = canAssignAgent(agent, user?.id, memberRole);
                        return (
                          <button
                            type="button"
                            key={agent.id}
                            disabled={!allowed}
                            onClick={() => {
                              if (!allowed) return;
                              onVerifierChange(agent.id);
                              setVerifierOpen(false);
                            }}
                            className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors ${allowed ? "hover:bg-accent" : "opacity-50 cursor-not-allowed"}`}
                          >
                            <ActorAvatar actorType="agent" actorId={agent.id} size={16} />
                            <span>{agent.name}</span>
                          </button>
                        );
                      })}
                    </>
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
                          const value = e.target.value
                            ? parseInt(e.target.value, 10)
                            : undefined;
                          onMaxVerificationRoundsChange(
                            value && value > 0 ? value : undefined
                          );
                        }}
                        className="w-14 rounded border px-1.5 py-0.5 text-xs text-right bg-transparent outline-none focus:ring-1 focus:ring-ring"
                      />
                    </label>
                  </div>
                )}
              </PopoverContent>
            </Popover>
          )}
          </div>

          {!isCompactSubmit && !hideSubmit ? submitButton : null}
        </div>
      )}

      <div className={cn("flex items-center gap-1 px-4 py-1.5", hasTopRow && "border-t")}>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <CreateIssuePillButton className="border-none px-1.5">
                <StatusIcon status={status} className="size-4" />
              </CreateIssuePillButton>
            }
          />
          <DropdownMenuContent align="start" className="w-44">
            {ALL_STATUSES.map((nextStatus) => (
              <DropdownMenuItem
                key={nextStatus}
                onClick={() => onStatusChange(nextStatus)}
              >
                <StatusIcon status={nextStatus} className="size-3.5" />
                <span>{STATUS_CONFIG[nextStatus].label}</span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <CreateIssuePillButton
                className={cn("border-none px-1.5", priority !== "none" && "px-2.5")}
              >
                <PriorityIcon priority={priority} className="size-4" />
                {priority !== "none" && <span>{PRIORITY_CONFIG[priority].label}</span>}
              </CreateIssuePillButton>
            }
          />
          <DropdownMenuContent align="start" className="w-44">
            {PRIORITY_ORDER.map((nextPriority) => (
              <DropdownMenuItem
                key={nextPriority}
                onClick={() => onPriorityChange(nextPriority)}
              >
                <span
                  className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium ${PRIORITY_CONFIG[nextPriority].badgeBg} ${PRIORITY_CONFIG[nextPriority].badgeText}`}
                >
                  <PriorityIcon priority={nextPriority} className="h-3 w-3" inheritColor />
                  {PRIORITY_CONFIG[nextPriority].label}
                </span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

        <Popover
          open={labelOpen}
          onOpenChange={(v) => {
            setLabelOpen(v);
            if (!v) setLabelFilter("");
          }}
        >
          <PopoverTrigger
            render={
              <CreateIssuePillButton
                className={cn("border-none", selectedLabels.length > 0 ? "px-2.5" : "px-1.5")}
              >
                {selectedLabels.length > 0 ? (
                  <div className="flex items-center gap-1 overflow-hidden">
                    {selectedLabels.slice(0, 2).map((label) => (
                      <span
                        key={label.id}
                        className="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs truncate"
                        style={{ backgroundColor: label.color + "15" }}
                      >
                        <span
                          className="h-2 w-2 rounded-full shrink-0"
                          style={{ backgroundColor: label.color }}
                        />
                        <span className="truncate max-w-[60px]">{label.name}</span>
                      </span>
                    ))}
                    {selectedLabels.length > 2 && (
                      <span className="text-muted-foreground">+{selectedLabels.length - 2}</span>
                    )}
                  </div>
                ) : (
                  <Tag className="size-4 text-muted-foreground" />
                )}
              </CreateIssuePillButton>
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
                .filter((label) =>
                  label.name.toLowerCase().includes(labelFilter.toLowerCase())
                )
                .map((label) => (
                  <button
                    key={label.id}
                    type="button"
                    onClick={() =>
                      onLabelsChange(
                        selectedLabels.some((item) => item.id === label.id)
                          ? selectedLabels.filter((item) => item.id !== label.id)
                          : [...selectedLabels, label]
                      )
                    }
                    className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors"
                  >
                    <span
                      className="h-3 w-3 rounded-full shrink-0"
                      style={{ backgroundColor: label.color }}
                    />
                    <span className="flex-1 text-left truncate">{label.name}</span>
                    {selectedLabels.some((item) => item.id === label.id) && (
                      <Check className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    )}
                  </button>
                ))}
              {allLabels.length === 0 && (
                <div className="px-2 py-3 text-center text-sm text-muted-foreground">
                  No labels yet
                </div>
              )}
            </div>
          </PopoverContent>
        </Popover>

        <Popover open={dueDateOpen} onOpenChange={setDueDateOpen}>
          <PopoverTrigger
            render={
              <CreateIssuePillButton
                className={cn("border-none", dueDateObj ? "px-2.5" : "px-1.5")}
              >
                <CalendarDays className="size-4 text-muted-foreground" />
                {dueDateObj && (
                  <span>
                    {formatAbsoluteTime(dueDate, "shortDate")}
                  </span>
                )}
              </CreateIssuePillButton>
            }
          />
          <PopoverContent className="w-auto p-0" align="start">
            <Calendar
              mode="single"
              selected={dueDateObj}
              onSelect={(date: Date | undefined) => {
                onDueDateChange(date ? date.toISOString() : null);
                setDueDateOpen(false);
              }}
            />
            {dueDateObj && (
              <div className="border-t px-3 py-2">
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => {
                    onDueDateChange(null);
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

        {/* Dispatch after — only meaningful when an agent is assigned */}
        {selectedAgent && onDispatchAfterChange && (
          <DispatchAfterPicker
            dispatchAfter={dispatchAfter ?? null}
            onChange={onDispatchAfterChange}
            triggerRender={
              <CreateIssuePillButton
                className={cn("border-none", dispatchAfter ? "px-2.5" : "px-1.5")}
              />
            }
            trigger={
              <>
                <Timer className="size-4 text-muted-foreground" />
                {dispatchAfter && (
                  <span>{formatAbsoluteTime(dispatchAfter, "shortDateTime")}</span>
                )}
              </>
            }
          />
        )}

        <FileUploadButton busy={uploading} onSelect={onUploadFile} />
        {isCompactSubmit && !hideSubmit ? submitButton : null}
      </div>
    </div>
  );
}
