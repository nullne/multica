"use client";

import { use, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { IssueDetail } from "@/features/issues/components";
import { useWorkspaceStore } from "@/features/workspace";
import { MulticaIcon } from "@/components/multica-icon";

export default function WorkspaceIssueDetailPage({
  params,
}: {
  params: Promise<{ workspaceSlug: string; id: string }>;
}) {
  const { workspaceSlug, id } = use(params);
  const router = useRouter();
  const workspace = useWorkspaceStore((s) => s.workspace);
  const workspaces = useWorkspaceStore((s) => s.workspaces);
  const switchWorkspace = useWorkspaceStore((s) => s.switchWorkspace);

  // Tracks the URL slug we've already reconciled with the active workspace.
  // Used to distinguish URL-driven sync (direct navigation) from a
  // user-driven workspace switch via the sidebar — only the former should
  // pull the active workspace toward the URL.
  const syncedSlugRef = useRef<string | null>(null);

  useEffect(() => {
    if (!workspace || workspaces.length === 0) return;

    if (workspace.slug === workspaceSlug) {
      syncedSlugRef.current = workspaceSlug;
      return;
    }

    // URL slug differs from the active workspace. If we haven't synced
    // this slug yet, it's a fresh navigation — align the active workspace
    // to match the URL.
    if (syncedSlugRef.current !== workspaceSlug) {
      const target = workspaces.find((w) => w.slug === workspaceSlug);
      if (target) {
        syncedSlugRef.current = workspaceSlug;
        void switchWorkspace(target.id);
      }
      return;
    }

    // Already synced once — the user has switched workspaces while sitting
    // on this issue page. Leave the stale issue context and land in the
    // newly selected workspace's issue list instead of reverting.
    router.replace("/issues");
  }, [workspace, workspaces, workspaceSlug, switchWorkspace, router]);

  if (!workspace || workspace.slug !== workspaceSlug) {
    return (
      <div className="flex h-full items-center justify-center">
        <MulticaIcon className="size-6 animate-pulse" />
      </div>
    );
  }

  return <IssueDetail issueId={id} />;
}
