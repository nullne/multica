"use client";

import { useState, useRef, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Check, ChevronRight, Maximize2, Minimize2, X as XIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type { IssueStatus, IssuePriority, IssueAssigneeType, Label, UpdateIssueRequest } from "@/shared/types";
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { Button } from "@/components/ui/button";
import { RichTextEditor, type RichTextEditorRef } from "@/components/common/rich-text-editor";
import { TitleEditor } from "@/components/common/title-editor";
import { StatusIcon } from "@/features/issues/components";
import { useRuntimeStore } from "@/features/runtimes";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore, useActorName } from "@/features/workspace";
import { useIssueStore } from "@/features/issues";
import { useIssueDraftStore } from "@/features/issues/stores/draft-store";
import { useIssueDefaultsStore } from "@/features/issues/stores/defaults-store";
import { issueUrl } from "@/features/issues/utils/url";
import { getAgentDispatchDefaults } from "@/features/issues/utils/dispatch";
import { api } from "@/shared/api";
import { useFileUpload } from "@/shared/hooks/use-file-upload";
import { CreateIssueToolbar } from "@/features/issues/components/create-issue-toolbar";

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
  const [dispatchAfter, setDispatchAfter] = useState<string | null>(null);
  const [verifierAgentId, setVerifierAgentId] = useState<string | undefined>(draft.verifierAgentId);
  const [maxVerificationRounds, setMaxVerificationRounds] = useState<number | undefined>(draft.maxVerificationRounds);
  const [isExpanded, setIsExpanded] = useState(false);

  const [selectedLabels, setSelectedLabels] = useState<Label[]>([]);

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

  const { uploadWithToast, uploading } = useFileUpload();
  const handleUpload = (file: File) => uploadWithToast(file);

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
        dispatch_after: dispatchAfter || undefined,
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
        <CreateIssueToolbar
          assigneeType={assigneeType ?? null}
          assigneeId={assigneeId ?? null}
          verifierAgentId={verifierAgentId ?? null}
          maxVerificationRounds={maxVerificationRounds}
          dispatchProvider={dispatchProvider ?? null}
          dispatchDaemonId={dispatchDaemonId ?? null}
          status={status}
          priority={priority}
          selectedLabels={selectedLabels}
          dueDate={dueDate}
          uploading={uploading}
          submitting={submitting}
          onAssigneeUpdate={applyAssigneePatch}
          onVerifierChange={updateVerifier}
          onMaxVerificationRoundsChange={updateMaxRounds}
          onStatusChange={updateStatus}
          onPriorityChange={updatePriority}
          onLabelsChange={setSelectedLabels}
          onDueDateChange={updateDueDate}
          dispatchAfter={dispatchAfter}
          onDispatchAfterChange={setDispatchAfter}
          onUploadFile={(file) => descEditorRef.current?.uploadFile(file)}
          onSubmit={handleSubmit}
        />
      </DialogContent>
    </Dialog>
  );
}
