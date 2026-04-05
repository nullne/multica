"use client";

import { useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import { MulticaIcon } from "@/components/multica-icon";
import { useNavigationStore } from "@/features/navigation";
import { SidebarProvider, SidebarInset, SidebarTrigger } from "@/components/ui/sidebar";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { AppSidebar } from "./_components/app-sidebar";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const workspace = useWorkspaceStore((s) => s.workspace);

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/");
    }
  }, [user, isLoading, router]);

  useEffect(() => {
    useNavigationStore.getState().onPathChange(pathname);
  }, [pathname]);

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <MulticaIcon className="size-6" />
      </div>
    );
  }

  if (!user) return null;

  return (
    <SidebarProvider className="h-svh">
      <AppSidebar />
      <SidebarInset className="overflow-hidden">
        {/* Mobile sidebar trigger — fixed at bottom-left */}
        <SidebarTrigger className="fixed bottom-4 left-4 z-50 md:hidden h-10 w-10 rounded-full bg-primary text-primary-foreground shadow-lg" />
        {workspace ? (
          children
        ) : (
          <div className="flex flex-1 items-center justify-center">
            <MulticaIcon className="size-6 animate-pulse" />
          </div>
        )}
      </SidebarInset>
    </SidebarProvider>
  );
}
