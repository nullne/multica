"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { toast } from "sonner";
import { CornerDownLeft, Loader2 } from "lucide-react";
import { Textarea } from "@/components/ui/textarea";
import type {
  Issue,
  IssueAssigneeType,
  IssuePriority,
  IssueStatus,
  Label,
  UpdateIssueRequest,
} from "@/shared/types";
import { api } from "@/shared/api";
import { useIssueStore } from "@/features/issues/store";
import { useActorName, useWorkspaceStore, WorkspaceAvatar } from "@/features/workspace";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useFileUpload } from "@/shared/hooks/use-file-upload";
import {
  CreateIssuePillButton,
  CreateIssueToolbar,
} from "./create-issue-toolbar";
import { AssigneePicker, PickerItem, PropertyPicker } from "./pickers";
import { IssueDetail } from "./issue-detail";

function titleFromDescription(value: string): string {
  const firstLine = value.trim().split(/\r?\n/).find(Boolean) ?? "";
  return firstLine.slice(0, 80) || "Untitled issue";
}

function CreateIssuePanel({
  selectedWorkspaceId,
  onWorkspaceChange,
  onCreated,
}: {
  selectedWorkspaceId: string;
  onWorkspaceChange: (workspaceId: string) => void;
  onCreated: (issue: Issue) => void;
}) {
  const workspaces = useWorkspaceStore((s) => s.workspaces);
  const { getActorName } = useActorName();
  const selectedWorkspace = workspaces.find(
    (workspace) => workspace.id === selectedWorkspaceId
  );
  const [description, setDescription] = useState("");
  const [workspaceOpen, setWorkspaceOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [assigneeType, setAssigneeType] = useState<IssueAssigneeType | null>(null);
  const [assigneeId, setAssigneeId] = useState<string | null>(null);
  const [dispatchProvider, setDispatchProvider] = useState<string | null>(null);
  const [dispatchDaemonId, setDispatchDaemonId] = useState<string | null>(null);
  const [dispatchDaemonLabel, setDispatchDaemonLabel] = useState<string | null>(null);
  const [status, setStatus] = useState<IssueStatus>("backlog");
  const [priority, setPriority] = useState<IssuePriority>("medium");
  const [dueDate, setDueDate] = useState<string | null>(null);
  const [selectedLabels, setSelectedLabels] = useState<Label[]>([]);
  const { uploadWithToast, uploading } = useFileUpload();

  // Clear workspace-scoped fields when the workspace changes so stale
  // assignees/labels from the previous workspace cannot leak into the
  // submitted payload.
  useEffect(() => {
    setAssigneeType(null);
    setAssigneeId(null);
    setDispatchProvider(null);
    setDispatchDaemonId(null);
    setDispatchDaemonLabel(null);
    setSelectedLabels([]);
  }, [selectedWorkspaceId]);
  const assigneeLabel =
    assigneeType && assigneeId ? getActorName(assigneeType, assigneeId) : "Assignee";

  const applyAssigneePatch = (patch: Partial<UpdateIssueRequest>) => {
    if ("assignee_type" in patch) {
      setAssigneeType(patch.assignee_type ?? null);
    }
    if ("assignee_id" in patch) {
      setAssigneeId(patch.assignee_id ?? null);
    }
    if ("dispatch_provider" in patch) {
      setDispatchProvider(patch.dispatch_provider ?? null);
    }
    if ("dispatch_daemon_id" in patch) {
      setDispatchDaemonId(patch.dispatch_daemon_id ?? null);
    }
    if ("dispatch_daemon_label" in patch) {
      setDispatchDaemonLabel(patch.dispatch_daemon_label ?? null);
    }
  };

  const applyIssuePatch = (patch: Partial<UpdateIssueRequest>) => {
    if ("status" in patch && patch.status) {
      setStatus(patch.status);
    }
    if ("priority" in patch && patch.priority) {
      setPriority(patch.priority);
    }
    if ("due_date" in patch) {
      setDueDate(patch.due_date ?? null);
    }
  };

  const handleUpload = async (file: File) => {
    const uploaded = await uploadWithToast(file);
    if (!uploaded) return;
    const attachmentMarkdown = `[${uploaded.filename}](${uploaded.link})`;
    setDescription((current) =>
      current.trim()
        ? `${current.trimEnd()}\n\n${attachmentMarkdown}`
        : attachmentMarkdown
    );
  };

  const handleSubmit = async () => {
    const trimmed = description.trim();
    if (!trimmed || submitting) return;

    setSubmitting(true);
    try {
      api.setWorkspaceId(selectedWorkspaceId);
      let issue = await api.createIssue({
        title: titleFromDescription(trimmed),
        description: trimmed,
        status,
        priority,
        due_date: dueDate || undefined,
        assignee_type: assigneeType ?? undefined,
        assignee_id: assigneeId ?? undefined,
        dispatch_provider: dispatchProvider ?? undefined,
        dispatch_daemon_id: dispatchDaemonId ?? undefined,
        dispatch_daemon_label: dispatchDaemonLabel ?? undefined,
      });
      if (selectedLabels.length > 0) {
        const labels = await api.setIssueLabels(
          issue.id,
          selectedLabels.map((label) => label.id)
        );
        issue = { ...issue, labels };
      }
      useIssueStore.getState().addIssue(issue);
      setDescription("");
      onCreated(issue);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to create issue");
    } finally {
      setSubmitting(false);
    }
  };

  const submitDisabled = submitting || !description.trim();

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <div className="flex flex-1 items-end px-5 pb-5">
        <div className="mx-auto w-full max-w-4xl">
          <div className="flex items-center gap-1.5 px-2 pb-2">
            <div className="shrink-0">
              <PropertyPicker
                open={workspaceOpen}
                onOpenChange={setWorkspaceOpen}
                width="w-56"
                align="start"
                triggerRender={<CreateIssuePillButton aria-label="Workspace" />}
                trigger={
                  <>
                    <WorkspaceAvatar name={selectedWorkspace?.name ?? "W"} size="sm" />
                    <span>{selectedWorkspace?.name ?? "Workspace"}</span>
                  </>
                }
              >
                {workspaces.map((workspace) => (
                  <PickerItem
                    key={workspace.id}
                    selected={workspace.id === selectedWorkspaceId}
                    onClick={() => {
                      onWorkspaceChange(workspace.id);
                      setWorkspaceOpen(false);
                    }}
                  >
                    <WorkspaceAvatar name={workspace.name} size="sm" />
                    <span className="truncate">{workspace.name}</span>
                  </PickerItem>
                ))}
              </PropertyPicker>
            </div>
            <AssigneePicker
              assigneeType={assigneeType}
              assigneeId={assigneeId}
              onUpdate={applyAssigneePatch}
              triggerRender={<CreateIssuePillButton aria-label="Assign" />}
              trigger={
                assigneeType && assigneeId ? (
                  <>
                    <ActorAvatar actorType={assigneeType} actorId={assigneeId} size={16} />
                    <span>{assigneeLabel}</span>
                  </>
                ) : (
                  <span className="text-muted-foreground">Assignee</span>
                )
              }
              align="start"
            />
          </div>
          <div className="relative overflow-hidden rounded-2xl border bg-card shadow-sm">
            <Textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                  event.preventDefault();
                  void handleSubmit();
                }
              }}
              placeholder="Describe the issue..."
              className="min-h-32 resize-none border-0 bg-transparent px-4 py-3 pr-14 text-base shadow-none focus-visible:ring-0"
            />
            <button
              type="button"
              aria-label={submitting ? "Creating issue" : "Create issue"}
              onClick={handleSubmit}
              disabled={submitDisabled}
              className="absolute bottom-2 right-2 inline-flex h-8 w-8 items-center justify-center rounded-lg border bg-background text-muted-foreground shadow-sm transition-colors hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-45"
            >
              {submitting ? (
                <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
              ) : (
                <CornerDownLeft className="size-3.5" aria-hidden="true" />
              )}
            </button>
          </div>
          <div className="pt-1">
            <CreateIssueToolbar
              assigneeType={assigneeType}
              assigneeId={assigneeId}
              dispatchProvider={dispatchProvider}
              dispatchDaemonId={dispatchDaemonId}
              status={status}
              priority={priority}
              selectedLabels={selectedLabels}
              dueDate={dueDate}
              uploading={uploading}
              submitting={submitting}
              submitDisabled={!description.trim()}
              onAssigneeUpdate={applyAssigneePatch}
              onStatusChange={setStatus}
              onPriorityChange={setPriority}
              onLabelsChange={setSelectedLabels}
              onDueDateChange={setDueDate}
              onUploadFile={handleUpload}
              onSubmit={handleSubmit}
              submitVariant="icon"
              showAssignee={false}
              hideSubmit
            />
          </div>
        </div>
      </div>
    </div>
  );
}

export function RecentIssuesPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentWorkspace = useWorkspaceStore((s) => s.workspace);
  const switchWorkspace = useWorkspaceStore((s) => s.switchWorkspace);

  const issueIdFromUrl = searchParams.get("issue");
  const showCreate = searchParams.get("new") === "1" || !issueIdFromUrl;
  // Trust the URL for the active issue id and let IssueDetail load it — falling
  // back to the first issue in the current workspace's store would render the
  // wrong issue when the user picks a recent from another workspace and would
  // briefly flash the wrong content while the workspace switch is in flight.
  const detailIssueId = issueIdFromUrl;

  return (
    <div className="flex flex-1 min-h-0 bg-background">
      <main className="min-w-0 flex-1">
        {showCreate ? (
          <CreateIssuePanel
            selectedWorkspaceId={currentWorkspace?.id ?? ""}
            onWorkspaceChange={(workspaceId) => {
              if (workspaceId === currentWorkspace?.id) return;
              void switchWorkspace(workspaceId);
            }}
            onCreated={(issue) => {
              router.push(`/home?issue=${issue.id}`);
            }}
          />
        ) : detailIssueId ? (
          <IssueDetail issueId={detailIssueId} />
        ) : (
          <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
            Select an issue or create a new one.
          </div>
        )}
      </main>
    </div>
  );
}
