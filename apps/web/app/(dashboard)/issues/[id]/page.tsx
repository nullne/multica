"use client";

import { use, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { IssueDetail } from "@/features/issues/components";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";

export default function LegacyIssueDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const router = useRouter();
  const workspace = useWorkspaceStore((s) => s.workspace);

  // Capture the workspace under which we first rendered this issue. If the
  // active workspace changes after that (user switched via the sidebar), we
  // should leave the stale issue context instead of fighting the switch by
  // redirecting to the issue's original workspace.
  const mountWorkspaceIdRef = useRef<string | null>(null);

  // Resolve the issue's workspace in background. If it belongs to a different
  // workspace than the active one, redirect to the canonical workspace-aware URL
  // so the correct workspace context is loaded before rendering.
  useEffect(() => {
    if (!workspace?.slug) return;

    if (mountWorkspaceIdRef.current === null) {
      mountWorkspaceIdRef.current = workspace.id;
    } else if (mountWorkspaceIdRef.current !== workspace.id) {
      // User switched workspaces while viewing this legacy URL — leave the
      // stale issue context and land in the new workspace's issue list.
      router.replace("/issues");
      return;
    }

    api
      .resolveIssueWorkspace(id)
      .then(({ workspace_slug }) => {
        if (workspace_slug !== workspace.slug) {
          router.replace(`/w/${workspace_slug}/issues/${id}`);
        }
      })
      .catch(() => {
        // Issue not found or no access — let IssueDetail render the not-found state.
      });
  }, [workspace?.id, workspace?.slug, id, router]);

  return <IssueDetail issueId={id} />;
}
