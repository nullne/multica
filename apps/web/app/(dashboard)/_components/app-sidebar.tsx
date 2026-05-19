"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Inbox,
  ListTodo,
  Bot,
  Monitor,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  ChevronsUpDown,
  Settings,
  LogOut,
  Plus,
  Check,
  BookOpenText,
  SquarePen,
  CircleUser,
  UserCog,
  SlidersHorizontal,
  Pin,
  PinOff,
} from "lucide-react";
import { WorkspaceAvatar } from "@/features/workspace";
import { useIssueDraftStore } from "@/features/issues/stores/draft-store";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarFooter,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { useInboxStore } from "@/features/inbox";
import { useModalStore } from "@/features/modals";
import { useIssueStore } from "@/features/issues/store";
import { StatusIcon } from "@/features/issues/components";
import { useRecentsPrefsStore } from "@/features/navigation/recents-prefs-store";
import { api } from "@/shared/api";
import type { Issue } from "@/shared/types";

const MAX_VISIBLE_ROWS_PER_GROUP = 10;
const ROW_HEIGHT_PX = 28;
const ROW_GAP_PX = 2;
const MAX_GROUP_BODY_HEIGHT_PX =
  MAX_VISIBLE_ROWS_PER_GROUP * ROW_HEIGHT_PX +
  (MAX_VISIBLE_ROWS_PER_GROUP - 1) * ROW_GAP_PX;

const primaryNav = [
  { href: "/inbox", label: "Inbox", icon: Inbox },
  { href: "/my-issues", label: "My Issues", icon: CircleUser },
  { href: "/issues", label: "Issues", icon: ListTodo },
];

const workspaceNav = [
  { href: "/agents", label: "Agents", icon: Bot },
  { href: "/daemons", label: "Daemons", icon: Monitor },
  { href: "/skills", label: "Skills", icon: BookOpenText },
  { href: "/settings", label: "Settings", icon: Settings },
];

function DraftDot() {
  const hasDraft = useIssueDraftStore((s) => !!(s.draft.title || s.draft.description));
  if (!hasDraft) return null;
  return <span className="absolute top-0 right-0 size-1.5 rounded-full bg-brand" />;
}

export function AppSidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const authLogout = useAuthStore((s) => s.logout);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const workspaces = useWorkspaceStore((s) => s.workspaces);
  const switchWorkspace = useWorkspaceStore((s) => s.switchWorkspace);
  const issues = useIssueStore((s) => s.issues);
  const [recentIssues, setRecentIssues] = useState<Issue[]>(issues);
  const [onlyMyIssues, setOnlyMyIssues] = useState(true);
  const [hideCompleted, setHideCompleted] = useState(false);
  const [groupByWorkspace, setGroupByWorkspace] = useState(true);
  const [recentSortBy, setRecentSortBy] = useState<"updated" | "created">("updated");

  const unreadCount = useInboxStore((s) => s.unreadCount());

  const logout = () => {
    router.push("/");
    authLogout();
    useWorkspaceStore.getState().clearWorkspace();
  };

  const userInitials = (user?.name ?? user?.email ?? "")
    .split(/\s+/)
    .map((w) => w[0])
    .filter(Boolean)
    .join("")
    .toUpperCase()
    .slice(0, 2);

  const isHome = pathname === "/home";
  const workspaceIds = workspaces.map((ws) => ws.id).join(",");

  useEffect(() => {
    if (!isHome) return;

    let cancelled = false;
    setRecentIssues(issues);

    if (workspaces.length === 0) return;

    void Promise.all(
      workspaces.map((ws) =>
        api
          .listIssues({ workspace_id: ws.id, limit: 50 })
          .then((res) => res.issues)
          .catch(() => [] as Issue[])
      )
    ).then((issueLists) => {
      if (!cancelled) {
        setRecentIssues(issueLists.flat());
      }
    });

    return () => {
      cancelled = true;
    };
  }, [isHome, issues, workspaceIds]); // eslint-disable-line react-hooks/exhaustive-deps

  const pinnedWorkspaceIds = useRecentsPrefsStore((s) => s.pinnedWorkspaceIds);
  const collapsedWorkspaceIds = useRecentsPrefsStore(
    (s) => s.collapsedWorkspaceIds,
  );
  const togglePinned = useRecentsPrefsStore((s) => s.togglePinned);
  const toggleCollapsed = useRecentsPrefsStore((s) => s.toggleCollapsed);
  const movePinned = useRecentsPrefsStore((s) => s.movePinned);

  const recentGroups = useMemo(() => {
    const workspaceById = new Map(workspaces.map((ws) => [ws.id, ws]));
    const filtered = recentIssues.filter((i) => {
      if (onlyMyIssues) {
        if (i.creator_type !== "member" || i.creator_id !== user?.id) return false;
      }
      if (hideCompleted && (i.status === "done" || i.status === "cancelled")) {
        return false;
      }
      return true;
    });
    const sortField = recentSortBy === "created" ? "created_at" : "updated_at";
    const sorted = [...filtered].sort((a, b) =>
      b[sortField].localeCompare(a[sortField])
    );
    if (!groupByWorkspace) {
      return [
        { id: "__all__", workspaceName: "", issues: sorted, pinned: false },
      ];
    }

    const grouped = sorted.reduce(
      (groups, issue) => {
        const group = groups.get(issue.workspace_id) ?? {
          id: issue.workspace_id,
          workspaceName:
            workspaceById.get(issue.workspace_id)?.name ?? "Unknown workspace",
          issues: [] as Issue[],
          pinned: false,
        };
        group.issues.push(issue);
        groups.set(issue.workspace_id, group);
        return groups;
      },
      new Map<
        string,
        { id: string; workspaceName: string; issues: Issue[]; pinned: boolean }
      >(),
    );

    const pinned = pinnedWorkspaceIds
      .map((id) => grouped.get(id))
      .filter(
        (g): g is { id: string; workspaceName: string; issues: Issue[]; pinned: boolean } =>
          !!g,
      )
      .map((g) => ({ ...g, pinned: true }));
    const pinnedIdSet = new Set(pinnedWorkspaceIds);
    const unpinned = [...grouped.values()].filter((g) => !pinnedIdSet.has(g.id));

    return [...pinned, ...unpinned];
  }, [
    recentIssues,
    workspaces,
    onlyMyIssues,
    user?.id,
    hideCompleted,
    recentSortBy,
    groupByWorkspace,
    pinnedWorkspaceIds,
  ]);

  if (isHome) {
    return (
      <Sidebar variant="inset">
        <SidebarHeader className="py-3">
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                render={
                  <button
                    type="button"
                    onClick={() => router.push("/home?new=1")}
                  />
                }
                className="font-medium"
              >
                <SquarePen />
                <span>New</span>
                <DraftDot />
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarHeader>

        <SidebarContent>
          <SidebarGroup>
            <div className="flex items-center justify-between px-2 pb-2">
              <span className="text-xs font-medium text-muted-foreground">
                Recents
              </span>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <button
                      type="button"
                      aria-label="Filter recents"
                      className="rounded-sm p-1 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
                    >
                      <SlidersHorizontal className="size-3.5" />
                    </button>
                  }
                />
                <DropdownMenuContent align="end" className="w-52">
                  <DropdownMenuCheckboxItem
                    checked={onlyMyIssues}
                    onCheckedChange={setOnlyMyIssues}
                  >
                    Only my issues
                  </DropdownMenuCheckboxItem>
                  <DropdownMenuCheckboxItem
                    checked={hideCompleted}
                    onCheckedChange={setHideCompleted}
                  >
                    Hide completed
                  </DropdownMenuCheckboxItem>
                  <DropdownMenuCheckboxItem
                    checked={groupByWorkspace}
                    onCheckedChange={setGroupByWorkspace}
                  >
                    Group by workspace
                  </DropdownMenuCheckboxItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuGroup>
                    <DropdownMenuLabel className="text-xs text-muted-foreground">
                      Sort by
                    </DropdownMenuLabel>
                    <DropdownMenuRadioGroup
                      value={recentSortBy}
                      onValueChange={(v) =>
                        setRecentSortBy(v as "updated" | "created")
                      }
                    >
                      <DropdownMenuRadioItem value="updated">
                        Last updated
                      </DropdownMenuRadioItem>
                      <DropdownMenuRadioItem value="created">
                        Last created
                      </DropdownMenuRadioItem>
                    </DropdownMenuRadioGroup>
                  </DropdownMenuGroup>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
            <SidebarGroupContent>
              <SidebarMenu className="gap-1">
                {recentGroups.map((group, groupIndex) => {
                  const isGrouped = group.id !== "__all__";
                  const isCollapsed =
                    isGrouped && collapsedWorkspaceIds.includes(group.id);
                  const pinnedIndex = group.pinned
                    ? pinnedWorkspaceIds.indexOf(group.id)
                    : -1;
                  const canMoveUp = group.pinned && pinnedIndex > 0;
                  const canMoveDown =
                    group.pinned &&
                    pinnedIndex >= 0 &&
                    pinnedIndex < pinnedWorkspaceIds.length - 1;
                  const showScroll =
                    group.issues.length > MAX_VISIBLE_ROWS_PER_GROUP;
                  const issueListId = `recent-group-${group.id}`;
                  return (
                    <div
                      key={group.id}
                      className="space-y-0.5"
                      data-testid={`recents-group-${group.id}`}
                      data-pinned={group.pinned ? "true" : "false"}
                      data-group-index={groupIndex}
                    >
                      {isGrouped && (
                        <div className="group/recents-header flex items-center gap-1 px-2 pt-2">
                          <button
                            type="button"
                            onClick={() => toggleCollapsed(group.id)}
                            aria-expanded={!isCollapsed}
                            aria-controls={issueListId}
                            className="flex min-w-0 flex-1 items-center gap-1 rounded-sm text-left text-xs text-muted-foreground hover:text-foreground transition-colors"
                          >
                            {isCollapsed ? (
                              <ChevronRight className="size-3 shrink-0" />
                            ) : (
                              <ChevronDown className="size-3 shrink-0" />
                            )}
                            <span className="truncate">{group.workspaceName}</span>
                          </button>
                          <div className="flex shrink-0 items-center opacity-0 group-hover/recents-header:opacity-100 focus-within:opacity-100 transition-opacity">
                            {group.pinned && (
                              <>
                                <button
                                  type="button"
                                  onClick={() => movePinned(group.id, "up")}
                                  disabled={!canMoveUp}
                                  aria-label={`Move ${group.workspaceName} up`}
                                  className="rounded-sm p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors disabled:opacity-30 disabled:hover:bg-transparent disabled:cursor-not-allowed"
                                >
                                  <ChevronUp className="size-3" />
                                </button>
                                <button
                                  type="button"
                                  onClick={() => movePinned(group.id, "down")}
                                  disabled={!canMoveDown}
                                  aria-label={`Move ${group.workspaceName} down`}
                                  className="rounded-sm p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors disabled:opacity-30 disabled:hover:bg-transparent disabled:cursor-not-allowed"
                                >
                                  <ChevronDown className="size-3" />
                                </button>
                              </>
                            )}
                            <button
                              type="button"
                              onClick={() => togglePinned(group.id)}
                              aria-label={
                                group.pinned
                                  ? `Unpin ${group.workspaceName}`
                                  : `Pin ${group.workspaceName}`
                              }
                              aria-pressed={group.pinned}
                              className="rounded-sm p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
                            >
                              {group.pinned ? (
                                <PinOff className="size-3" />
                              ) : (
                                <Pin className="size-3" />
                              )}
                            </button>
                          </div>
                        </div>
                      )}
                      {!isCollapsed && (
                        <div
                          id={issueListId}
                          data-testid={`recents-group-body-${group.id}`}
                          data-scrollable={showScroll ? "true" : "false"}
                          className={
                            showScroll
                              ? "space-y-0.5 overflow-y-auto pr-0.5"
                              : "space-y-0.5"
                          }
                          style={
                            showScroll
                              ? { maxHeight: MAX_GROUP_BODY_HEIGHT_PX }
                              : undefined
                          }
                        >
                          {group.issues.map((issue) => {
                            const isAgentRunning =
                              issue.assignee_type === "agent" &&
                              issue.status === "in_progress";
                            return (
                              <SidebarMenuItem key={issue.id}>
                                <Tooltip>
                                  <TooltipTrigger
                                    render={
                                      <SidebarMenuButton
                                        size="sm"
                                        onClick={async () => {
                                          if (issue.workspace_id !== workspace?.id) {
                                            await switchWorkspace(issue.workspace_id);
                                          }
                                          router.push(`/home?issue=${issue.id}`);
                                        }}
                                      >
                                        <span className="relative inline-flex shrink-0">
                                          <StatusIcon
                                            status={issue.status}
                                            className="size-4"
                                          />
                                          {isAgentRunning && (
                                            <span
                                              aria-hidden
                                              className="absolute -top-0.5 -right-0.5 flex size-1.5"
                                            >
                                              <span className="absolute inline-flex size-full animate-ping rounded-full bg-primary opacity-75" />
                                              <span className="relative inline-flex size-1.5 rounded-full bg-primary" />
                                            </span>
                                          )}
                                        </span>
                                        <span className="truncate">{issue.title}</span>
                                      </SidebarMenuButton>
                                    }
                                  />
                                  <TooltipContent side="right">
                                    {issue.identifier}
                                  </TooltipContent>
                                </Tooltip>
                              </SidebarMenuItem>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  );
                })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>

        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <SidebarMenuButton size="lg" className="gap-2">
                      <span className="flex h-7 w-7 shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted text-xs font-semibold text-muted-foreground">
                        {user?.avatar_url ? (
                          <img
                            src={user.avatar_url}
                            alt={user.name}
                            className="h-full w-full object-cover"
                          />
                        ) : (
                          userInitials || "?"
                        )}
                      </span>
                      <span className="flex min-w-0 flex-1 flex-col text-left">
                        <span className="truncate text-sm font-medium">
                          {user?.name ?? "Account"}
                        </span>
                        <span className="truncate text-xs text-muted-foreground">
                          {user?.email}
                        </span>
                      </span>
                      <ChevronsUpDown className="size-3.5 text-muted-foreground" />
                    </SidebarMenuButton>
                  }
                />
                <DropdownMenuContent
                  className="w-(--radix-dropdown-menu-trigger-width) min-w-56"
                  align="start"
                  side="top"
                  sideOffset={4}
                >
                  <div className="px-1.5 py-1 text-xs text-muted-foreground">
                    {user?.email}
                  </div>
                  <DropdownMenuSeparator />
                  <DropdownMenuGroup>
                    <DropdownMenuItem onClick={() => router.push("/account")}>
                      <UserCog className="h-3.5 w-3.5" />
                      Account settings
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onClick={() => router.push("/account?tab=daemons")}
                    >
                      <Monitor className="h-3.5 w-3.5" />
                      My daemons
                    </DropdownMenuItem>
                  </DropdownMenuGroup>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem variant="destructive" onClick={logout}>
                    <LogOut className="h-3.5 w-3.5" />
                    Log out
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>
    );
  }

  return (
      <Sidebar variant="inset">
        {/* Workspace Switcher */}
        <SidebarHeader className="py-3">
          <div className="flex items-center gap-4">
            <SidebarMenu className="min-w-0 flex-1">
              <SidebarMenuItem>
                <DropdownMenu>
                  <DropdownMenuTrigger
                    render={
                      <SidebarMenuButton>
                        <WorkspaceAvatar name={workspace?.name ?? "M"} size="sm" />
                        <span className="flex-1 truncate font-medium">
                          {workspace?.name ?? "Multica"}
                        </span>
                        <ChevronDown className="size-3 text-muted-foreground" />
                      </SidebarMenuButton>
                    }
                  />
                <DropdownMenuContent
                  className="w-52"
                  align="start"
                  side="bottom"
                  sideOffset={4}
                >
                  <DropdownMenuGroup className="group/ws-section">
                    <DropdownMenuLabel className="flex items-center text-xs text-muted-foreground">
                      Workspaces
                      <Tooltip>
                        <TooltipTrigger
                          className="ml-auto opacity-0 group-hover/ws-section:opacity-100 transition-opacity rounded hover:bg-accent p-0.5"
                          onClick={() => useModalStore.getState().open("create-workspace")}
                        >
                          <Plus className="h-3.5 w-3.5" />
                        </TooltipTrigger>
                        <TooltipContent side="right">
                          Create workspace
                        </TooltipContent>
                      </Tooltip>
                    </DropdownMenuLabel>
                    {workspaces.map((ws) => (
                      <DropdownMenuItem
                        key={ws.id}
                        onClick={() => {
                          if (ws.id !== workspace?.id) {
                            switchWorkspace(ws.id);
                          }
                        }}
                      >
                        <WorkspaceAvatar name={ws.name} size="sm" />
                        <span className="flex-1 truncate">{ws.name}</span>
                        {ws.id === workspace?.id && (
                          <Check className="h-3.5 w-3.5 text-primary" />
                        )}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuGroup>
                </DropdownMenuContent>
                </DropdownMenu>
              </SidebarMenuItem>
            </SidebarMenu>
            <Tooltip>
              <TooltipTrigger
                className="relative flex h-7 w-7 items-center justify-center rounded-lg bg-background text-foreground shadow-sm hover:bg-accent"
                onClick={() => useModalStore.getState().open("create-issue")}
              >
                <SquarePen className="size-3.5" />
                <DraftDot />
              </TooltipTrigger>
              <TooltipContent side="bottom">New issue (⇧⌘O)</TooltipContent>
            </Tooltip>
          </div>
        </SidebarHeader>

        {/* Navigation */}
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu className="gap-0.5">
                {primaryNav.map((item) => {
                  const isActive = pathname === item.href;
                  return (
                    <SidebarMenuItem key={item.href}>
                      <SidebarMenuButton
                        isActive={isActive}
                        render={<Link href={item.href} />}
                        className="text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground"
                      >
                        <item.icon />
                        <span>{item.label}</span>
                        {item.label === "Inbox" && unreadCount > 0 && (
                          <span className="ml-auto text-xs">
                            {unreadCount > 99 ? "99+" : unreadCount}
                          </span>
                        )}
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>

          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu className="gap-0.5">
                {workspaceNav.map((item) => {
                  const isActive = pathname === item.href;
                  return (
                    <SidebarMenuItem key={item.href}>
                      <SidebarMenuButton
                        isActive={isActive}
                        render={<Link href={item.href} />}
                        className="text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground"
                      >
                        <item.icon />
                        <span>{item.label}</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>

        {/* User menu — opens upward */}
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <SidebarMenuButton size="lg" className="gap-2">
                      <span className="flex h-7 w-7 shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted text-xs font-semibold text-muted-foreground">
                        {user?.avatar_url ? (
                          <img
                            src={user.avatar_url}
                            alt={user.name}
                            className="h-full w-full object-cover"
                          />
                        ) : (
                          userInitials || "?"
                        )}
                      </span>
                      <span className="flex min-w-0 flex-1 flex-col text-left">
                        <span className="truncate text-sm font-medium">
                          {user?.name ?? "Account"}
                        </span>
                        <span className="truncate text-xs text-muted-foreground">
                          {user?.email}
                        </span>
                      </span>
                      <ChevronsUpDown className="size-3.5 text-muted-foreground" />
                    </SidebarMenuButton>
                  }
                />
                <DropdownMenuContent
                  className="w-(--radix-dropdown-menu-trigger-width) min-w-56"
                  align="start"
                  side="top"
                  sideOffset={4}
                >
                  <div className="px-1.5 py-1 text-xs text-muted-foreground">
                    {user?.email}
                  </div>
                  <DropdownMenuSeparator />
                  <DropdownMenuGroup>
                    <DropdownMenuItem
                      onClick={() => router.push("/account")}
                    >
                      <UserCog className="h-3.5 w-3.5" />
                      Account settings
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onClick={() => router.push("/account?tab=daemons")}
                    >
                      <Monitor className="h-3.5 w-3.5" />
                      My daemons
                    </DropdownMenuItem>
                  </DropdownMenuGroup>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem variant="destructive" onClick={logout}>
                    <LogOut className="h-3.5 w-3.5" />
                    Log out
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>
  );
}
