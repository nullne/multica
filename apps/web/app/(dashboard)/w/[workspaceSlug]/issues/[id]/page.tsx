"use client";

import { use, useEffect } from "react";
import { IssueDetail } from "@/features/issues/components";
import { useWorkspaceStore } from "@/features/workspace";
import { MulticaIcon } from "@/components/multica-icon";

export default function WorkspaceIssueDetailPage({
  params,
}: {
  params: Promise<{ workspaceSlug: string; id: string }>;
}) {
  const { workspaceSlug, id } = use(params);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const workspaces = useWorkspaceStore((s) => s.workspaces);
  const switchWorkspace = useWorkspaceStore((s) => s.switchWorkspace);

  useEffect(() => {
    if (!workspace || workspaces.length === 0) return;
    if (workspace.slug === workspaceSlug) return;

    const target = workspaces.find((w) => w.slug === workspaceSlug);
    if (target) {
      void switchWorkspace(target.id);
    }
  }, [workspace, workspaces, workspaceSlug, switchWorkspace]);

  if (!workspace || workspace.slug !== workspaceSlug) {
    return (
      <div className="flex h-full items-center justify-center">
        <MulticaIcon className="size-6 animate-pulse" />
      </div>
    );
  }

  return <IssueDetail issueId={id} />;
}
