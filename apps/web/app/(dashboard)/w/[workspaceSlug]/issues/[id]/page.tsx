"use client";

import { use, useEffect, useRef, useState } from "react";
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

  // Prevent re-running the URL-based workspace sync after the initial load.
  // Without this guard, a user-initiated workspace switch would be undone
  // because the stale URL slug would trigger another switchWorkspace call.
  const syncInitiated = useRef(false);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (syncInitiated.current) return;
    if (!workspace || workspaces.length === 0) return;

    syncInitiated.current = true;

    if (workspace.slug === workspaceSlug) {
      setReady(true);
      return;
    }

    const target = workspaces.find((w) => w.slug === workspaceSlug);
    if (target) {
      void switchWorkspace(target.id).then(() => setReady(true));
    } else {
      setReady(true);
    }
  }, [workspace, workspaces, workspaceSlug, switchWorkspace]);

  if (!ready) {
    return (
      <div className="flex h-full items-center justify-center">
        <MulticaIcon className="size-6 animate-pulse" />
      </div>
    );
  }

  return <IssueDetail issueId={id} />;
}
