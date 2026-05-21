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

  // Resolve the issue's workspace once on initial load. The ref prevents
  // re-running after a user-initiated workspace switch, which would otherwise
  // redirect back to the issue's original workspace.
  const resolved = useRef(false);

  useEffect(() => {
    if (resolved.current) return;
    if (!workspace?.slug) return;

    resolved.current = true;

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
