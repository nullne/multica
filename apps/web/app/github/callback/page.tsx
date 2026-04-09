"use client";

import { useEffect, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/shared/api";
import { useWorkspaceStore } from "@/features/workspace";

export default function GitHubCallbackPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const attempted = useRef(false);

  useEffect(() => {
    if (attempted.current) return;
    attempted.current = true;

    const installationId = searchParams.get("installation_id");
    const workspaceId = searchParams.get("state");

    if (!installationId || !workspaceId) {
      router.replace("/settings");
      return;
    }

    api
      .connectGitHub(workspaceId, Number(installationId))
      .then((updated) => {
        useWorkspaceStore.getState().updateWorkspace(updated);
        router.replace("/settings");
      })
      .catch(() => {
        router.replace("/settings");
      });
  }, [searchParams, router]);

  return (
    <div className="flex h-screen items-center justify-center">
      <p className="text-sm text-muted-foreground">Connecting GitHub…</p>
    </div>
  );
}
