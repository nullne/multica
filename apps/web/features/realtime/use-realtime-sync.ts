"use client";

import { useEffect } from "react";
import type { WSClient } from "@/shared/api";
import { toast } from "sonner";
import { useIssueStore } from "@/features/issues";
import { useInboxStore } from "@/features/inbox";
import { useLabelStore } from "@/features/labels";
import { useWorkspaceStore } from "@/features/workspace";
import { useAuthStore } from "@/features/auth";
import { useRecentsStore } from "@/features/navigation/recents-store";
import { createLogger } from "@/shared/logger";
import { api } from "@/shared/api";
import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";
import type {
  MemberAddedPayload,
  WorkspaceDeletedPayload,
  MemberRemovedPayload,
  IssueUpdatedPayload,
  IssueCreatedPayload,
  IssueDeletedPayload,
  InboxNewPayload,
  LabelCreatedPayload,
  LabelUpdatedPayload,
  LabelDeletedPayload,
  TaskDispatchPayload,
  TaskCompletedPayload,
  TaskFailedPayload,
  TaskCancelledPayload,
} from "@/shared/types";

const logger = createLogger("realtime-sync");

function isCurrentWorkspace(workspaceId?: string | null): boolean {
  const currentWsId = useWorkspaceStore.getState().workspace?.id;
  return Boolean(workspaceId && workspaceId === currentWsId);
}

function currentWorkspaceId(): string | null {
  return useWorkspaceStore.getState().workspace?.id ?? null;
}

async function refreshActiveTasksForWorkspace(workspaceId: string | null) {
  const tasks = await api.listActiveTasks();
  if (currentWorkspaceId() !== workspaceId) return;
  useActiveTaskStore.getState().setTasks(tasks);
}

/**
 * Centralized WS → store sync. Called once from WSProvider.
 *
 * Uses the "WS as invalidation signal + refetch" pattern:
 * - onAny handler extracts event prefix and calls the matching store refresh
 * - Debounce per-prefix prevents rapid-fire refetches (e.g. bulk issue updates)
 * - Precise handlers only for side effects (toast, navigation, self-check)
 *
 * Per-page events (comments, activity, subscribers, daemon) are still handled
 * by individual components via useWSEvent — not here.
 */
export function useRealtimeSync(ws: WSClient | null) {
  // Main sync: onAny → refreshMap with debounce
  useEffect(() => {
    if (!ws) return;

    // Event types handled by specific handlers below — skip generic refresh
    const specificEvents = new Set([
      "issue:updated", "issue:created", "issue:deleted", "inbox:new",
      "label:created", "label:updated", "label:deleted",
      "task:dispatch", "task:completed", "task:failed", "task:cancelled",
    ]);

    const refreshMap: Record<string, () => void> = {
      inbox: () => void useInboxStore.getState().fetch(),
      agent: () => void useWorkspaceStore.getState().refreshAgents(),
      member: () => void useWorkspaceStore.getState().refreshMembers(),
      workspace: () => {
        // Lightweight: only re-fetch workspace list, don't hydrate everything.
        // workspace:deleted is handled by a precise side-effect handler below.
        api.listWorkspaces().then((wsList) => {
          const current = useWorkspaceStore.getState().workspace;
          const updated = current
            ? wsList.find((w) => w.id === current.id)
            : null;
          if (updated) useWorkspaceStore.getState().updateWorkspace(updated);
        }).catch((err) => {
          logger.error("workspace refresh failed", err);
        });
      },
      skill: () => void useWorkspaceStore.getState().refreshSkills(),
    };

    const timers = new Map<string, ReturnType<typeof setTimeout>>();
    const debouncedRefresh = (prefix: string, fn: () => void) => {
      const existing = timers.get(prefix);
      if (existing) clearTimeout(existing);
      timers.set(
        prefix,
        setTimeout(() => {
          timers.delete(prefix);
          fn();
        }, 100),
      );
    };

    const unsubAny = ws.onAny((msg) => {
      const myUserId = useAuthStore.getState().user?.id;
      if (msg.actor_id && msg.actor_id === myUserId) {
        logger.debug("skipping self-event", msg.type);
        return;
      }
      if (specificEvents.has(msg.type)) return;
      const prefix = msg.type.split(":")[0] ?? "";
      const refresh = refreshMap[prefix];
      if (refresh) debouncedRefresh(prefix, refresh);
    });

    // --- Specific event handlers (granular updates, no full refetch) ---

    const unsubIssueUpdated = ws.on("issue:updated", (p) => {
      const { issue } = p as IssueUpdatedPayload;
      if (!issue?.id) return;
      if (!isCurrentWorkspace(issue.workspace_id)) return;
      useIssueStore.getState().updateIssue(issue.id, issue);
      useRecentsStore.getState().upsertIssue(issue);
      if (issue.status) {
        useInboxStore.getState().updateIssueStatus(issue.id, issue.status);
      }
    });

    const unsubIssueCreated = ws.on("issue:created", (p) => {
      const { issue } = p as IssueCreatedPayload;
      if (!issue) return;
      if (!isCurrentWorkspace(issue.workspace_id)) return;
      useIssueStore.getState().addIssue(issue);
      useRecentsStore.getState().upsertIssue(issue);
    });

    const unsubIssueDeleted = ws.on("issue:deleted", (p) => {
      const { issue_id } = p as IssueDeletedPayload;
      if (!issue_id) return;
      useIssueStore.getState().removeIssue(issue_id);
      useRecentsStore.getState().removeIssue(issue_id);
    });

    const unsubLabelCreated = ws.on("label:created", (p) => {
      const { label } = p as LabelCreatedPayload;
      if (label && !isCurrentWorkspace(label.workspace_id)) return;
      if (label) useLabelStore.getState().addLabel(label);
    });

    const unsubLabelUpdated = ws.on("label:updated", (p) => {
      const { label } = p as LabelUpdatedPayload;
      if (label && !isCurrentWorkspace(label.workspace_id)) return;
      if (label?.id) useLabelStore.getState().updateLabel(label.id, label);
    });

    const unsubLabelDeleted = ws.on("label:deleted", (p) => {
      const { label_id } = p as LabelDeletedPayload;
      if (label_id) useLabelStore.getState().removeLabel(label_id);
    });

    const unsubInboxNew = ws.on("inbox:new", (p) => {
      const { item } = p as InboxNewPayload;
      if (!item) return;
      const currentWsId = useWorkspaceStore.getState().workspace?.id;
      if (item.workspace_id && item.workspace_id !== currentWsId) return;
      useInboxStore.getState().addItem(item);
    });

    // --- Active task tracking ---

    const unsubTaskDispatch = ws.on("task:dispatch", (p) => {
      const { task_id, issue_id } = p as TaskDispatchPayload;
      if (!issue_id || !task_id) return;
      const workspaceId = currentWorkspaceId();
      api.getActiveTaskForIssue(issue_id).then(({ task }) => {
        if (currentWorkspaceId() !== workspaceId) return;
        if (task) useActiveTaskStore.getState().setTask(issue_id, task);
      }).catch(console.error);
    });

    const unsubTaskCompleted = ws.on("task:completed", (p) => {
      const { issue_id } = p as TaskCompletedPayload;
      if (issue_id) useActiveTaskStore.getState().clearTask(issue_id);
    });

    const unsubTaskFailed = ws.on("task:failed", (p) => {
      const { issue_id } = p as TaskFailedPayload;
      if (issue_id) useActiveTaskStore.getState().clearTask(issue_id);
    });

    const unsubTaskCancelled = ws.on("task:cancelled", (p) => {
      const { issue_id } = p as TaskCancelledPayload;
      if (issue_id) useActiveTaskStore.getState().clearTask(issue_id);
    });

    // --- Side-effect handlers (toast, navigation) ---

    const unsubWsDeleted = ws.on("workspace:deleted", (p) => {
      const { workspace_id } = p as WorkspaceDeletedPayload;
      const currentWs = useWorkspaceStore.getState().workspace;
      if (currentWs?.id === workspace_id) {
        logger.warn("current workspace deleted, switching");
        toast.info("This workspace was deleted");
        useWorkspaceStore.getState().refreshWorkspaces();
      }
    });

    const unsubMemberRemoved = ws.on("member:removed", (p) => {
      const { user_id } = p as MemberRemovedPayload;
      const myUserId = useAuthStore.getState().user?.id;
      if (user_id === myUserId) {
        logger.warn("removed from workspace, switching");
        toast.info("You were removed from this workspace");
        useWorkspaceStore.getState().refreshWorkspaces();
      }
    });

    const unsubMemberAdded = ws.on("member:added", (p) => {
      const { member, workspace_name } = p as MemberAddedPayload;
      const myUserId = useAuthStore.getState().user?.id;
      if (member.user_id === myUserId) {
        useWorkspaceStore.getState().refreshWorkspaces();
        toast.info(
          `You were invited to ${workspace_name ?? "a workspace"}`,
        );
      }
    });

    return () => {
      unsubAny();
      unsubIssueUpdated();
      unsubIssueCreated();
      unsubIssueDeleted();
      unsubLabelCreated();
      unsubLabelUpdated();
      unsubLabelDeleted();
      unsubInboxNew();
      unsubTaskDispatch();
      unsubTaskCompleted();
      unsubTaskFailed();
      unsubTaskCancelled();
      unsubWsDeleted();
      unsubMemberRemoved();
      unsubMemberAdded();
      timers.forEach(clearTimeout);
      timers.clear();
    };
  }, [ws]);

  // Initial fetch of active tasks so board/list views show the correct state on load
  useEffect(() => {
    if (!ws) return;
    refreshActiveTasksForWorkspace(currentWorkspaceId()).catch(console.error);
  }, [ws]);

  // Reconnect → refetch all data to recover missed events
  useEffect(() => {
    if (!ws) return;

    const unsub = ws.onReconnect(async () => {
      logger.info("reconnected, refetching all data");
      try {
        await Promise.all([
          useIssueStore.getState().fetch(),
          useInboxStore.getState().fetch(),
          useLabelStore.getState().fetch(),
          useWorkspaceStore.getState().refreshAgents(),
          useWorkspaceStore.getState().refreshMembers(),
          useWorkspaceStore.getState().refreshSkills(),
          // The home sidebar's Recents list lives in its own store now, so
          // it needs an explicit recovery here — otherwise an `issue:*`
          // event missed during disconnect would leave stale rows until
          // the user manually toggles a filter or re-enters /home.
          useRecentsStore.getState().refresh(),
          refreshActiveTasksForWorkspace(currentWorkspaceId()).catch(console.error),
        ]);
      } catch (e) {
        logger.error("reconnect refetch failed", e);
      }
    });

    return unsub;
  }, [ws]);
}
