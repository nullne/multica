"use client";

import { use, useEffect } from "react";
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

  // Resolve the issue's workspace in background. If it belongs to a different
  // workspace than the active one, redirect to the canonical workspace-aware URL
  // so the correct workspace context is loaded before rendering.
  useEffect(() => {
    if (!workspace?.slug) return;

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
  }, [workspace?.id, id]); // eslint-disable-line react-hooks/exhaustive-deps

  return <IssueDetail issueId={id} />;
}
