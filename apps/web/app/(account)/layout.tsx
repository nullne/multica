"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { MulticaIcon } from "@/components/multica-icon";
import { useAuthStore } from "@/features/auth";
import { useNavigationStore } from "@/features/navigation";
import { useWorkspaceStore } from "@/features/workspace";

export default function AccountLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const lastPath = useNavigationStore((s) => s.lastPath);
  const workspace = useWorkspaceStore((s) => s.workspace);

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/");
    }
  }, [user, isLoading, router]);

  if (isLoading) {
    return (
      <div className="flex h-dvh items-center justify-center">
        <MulticaIcon className="size-6" />
      </div>
    );
  }

  if (!user) return null;

  const backHref = lastPath && !lastPath.startsWith("/account") ? lastPath : "/issues";
  const backLabel = workspace?.name ? `Back to ${workspace.name}` : "Back to workspace";

  return (
    <div className="flex h-dvh flex-col bg-background">
      <header className="flex h-12 shrink-0 items-center border-b px-4">
        <Link
          href={backHref}
          className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-4" />
          <span className="truncate">{backLabel}</span>
        </Link>
      </header>
      {children}
    </div>
  );
}
