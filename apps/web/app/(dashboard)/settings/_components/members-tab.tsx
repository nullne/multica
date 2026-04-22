"use client";

import { useCallback, useEffect, useState } from "react";
import { Bot, Crown, MoreHorizontal, Plus, Shield, Trash2, User, UserMinus, Users } from "lucide-react";
import { ActorAvatar } from "@/components/common/actor-avatar";
import type { BotUser, MemberWithUser, MemberRole } from "@/shared/types";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from "@/components/ui/alert-dialog";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
} from "@/components/ui/dropdown-menu";
import { Label } from "@/components/ui/label";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { toast } from "sonner";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";

const roleConfig: Record<MemberRole, { label: string; icon: typeof Crown; description: string }> = {
  owner: { label: "Owner", icon: Crown, description: "Full access, manage all settings" },
  admin: { label: "Admin", icon: Shield, description: "Manage members and settings" },
  member: { label: "Member", icon: User, description: "Create and work on issues" },
};

function MemberRow({
  member,
  canManage,
  canManageOwners,
  isSelf,
  busy,
  onRoleChange,
  onRemove,
}: {
  member: MemberWithUser;
  canManage: boolean;
  canManageOwners: boolean;
  isSelf: boolean;
  busy: boolean;
  onRoleChange: (role: MemberRole) => void;
  onRemove: () => void;
}) {
  const rc = roleConfig[member.role];
  const RoleIcon = rc.icon;
  const canEditRole = canManage && !isSelf && (member.role !== "owner" || canManageOwners);
  const canRemove = canManage && !isSelf && (member.role !== "owner" || canManageOwners);
  const showMenu = canEditRole || canRemove;

  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <ActorAvatar actorType="member" actorId={member.user_id} size={32} />
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium truncate">{member.name}</div>
        <div className="text-xs text-muted-foreground truncate">{member.email}</div>
      </div>
      {showMenu && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon-sm" disabled={busy}>
                <MoreHorizontal className="h-4 w-4 text-muted-foreground" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-auto">
            {canEditRole && (
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <Shield className="h-3.5 w-3.5" />
                  Change role
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent className="w-auto">
                  {(Object.entries(roleConfig) as [MemberRole, (typeof roleConfig)[MemberRole]][]).map(
                    ([role, config]) => {
                      if (role === "owner" && !canManageOwners) return null;
                      const Icon = config.icon;
                      return (
                        <DropdownMenuItem
                          key={role}
                          onClick={() => onRoleChange(role)}
                        >
                          <Icon className="h-3.5 w-3.5" />
                          <div className="flex flex-col">
                            <span>{config.label}</span>
                            <span className="text-xs text-muted-foreground font-normal">
                              {config.description}
                            </span>
                          </div>
                          {member.role === role && (
                            <span className="ml-auto text-xs text-muted-foreground">&#10003;</span>
                          )}
                        </DropdownMenuItem>
                      );
                    }
                  )}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            )}
            {canEditRole && canRemove && <DropdownMenuSeparator />}
            {canRemove && (
              <DropdownMenuItem variant="destructive" onClick={onRemove}>
                <UserMinus className="h-3.5 w-3.5" />
                Remove from workspace
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <Badge variant="secondary">
        <RoleIcon className="h-3 w-3" />
        {rc.label}
      </Badge>
    </div>
  );
}

function BotRow({
  bot,
  canManage,
  busy,
  onDelete,
}: {
  bot: BotUser;
  canManage: boolean;
  busy: boolean;
  onDelete: () => void;
}) {
  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <ActorAvatar actorType="member" actorId={bot.id} size={32} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium truncate">{bot.name}</span>
          <Badge variant="outline" className="text-[10px] px-1.5 py-0">bot</Badge>
        </div>
        <div className="text-xs text-muted-foreground truncate">
          Used by {bot.webhook_count} webhook{bot.webhook_count === 1 ? "" : "s"}
        </div>
      </div>
      {canManage && (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button variant="ghost" size="icon-sm" disabled={busy} onClick={onDelete}>
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            }
          />
          <TooltipContent>Delete bot</TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}

export function MembersTab() {
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);
  const refreshMembers = useWorkspaceStore((s) => s.refreshMembers);

  const [bots, setBots] = useState<BotUser[]>([]);
  const [botsLoading, setBotsLoading] = useState(true);

  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<MemberRole>("member");
  const [inviteLoading, setInviteLoading] = useState(false);
  const [memberActionId, setMemberActionId] = useState<string | null>(null);
  const [confirmAction, setConfirmAction] = useState<{
    title: string;
    description: string;
    variant?: "destructive";
    onConfirm: () => Promise<void>;
  } | null>(null);

  // Bot creation dialog state
  const [createBotOpen, setCreateBotOpen] = useState(false);
  const [newBotName, setNewBotName] = useState("");
  const [creatingBot, setCreatingBot] = useState(false);

  const reloadBots = useCallback(async () => {
    if (!workspace?.id) return;
    try {
      const list = await api.listBotUsers(workspace.id);
      setBots(list);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to load bot users");
    } finally {
      setBotsLoading(false);
    }
  }, [workspace?.id]);

  useEffect(() => { reloadBots(); }, [reloadBots]);

  // Member list excludes bots — they live in their own section below.
  const humanMembers = members.filter((m) => m.kind !== "bot");

  const currentMember = humanMembers.find((m) => m.user_id === user?.id) ?? null;
  const canManageWorkspace = currentMember?.role === "owner" || currentMember?.role === "admin";
  const isOwner = currentMember?.role === "owner";

  const handleAddMember = async () => {
    if (!workspace) return;
    setInviteLoading(true);
    try {
      await api.createMember(workspace.id, {
        email: inviteEmail,
        role: inviteRole,
      });
      setInviteEmail("");
      setInviteRole("member");
      await refreshMembers();
      toast.success("Member added");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to add member");
    } finally {
      setInviteLoading(false);
    }
  };

  const handleRoleChange = async (memberId: string, role: MemberRole) => {
    if (!workspace) return;
    setMemberActionId(memberId);
    try {
      await api.updateMember(workspace.id, memberId, { role });
      await refreshMembers();
      toast.success("Role updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update member");
    } finally {
      setMemberActionId(null);
    }
  };

  const handleRemoveMember = (member: MemberWithUser) => {
    if (!workspace) return;
    setConfirmAction({
      title: `Remove ${member.name}`,
      description: `Remove ${member.name} from ${workspace.name}? They will lose access to this workspace.`,
      variant: "destructive",
      onConfirm: async () => {
        setMemberActionId(member.id);
        try {
          await api.deleteMember(workspace.id, member.id);
          await refreshMembers();
          toast.success("Member removed");
        } catch (e) {
          toast.error(e instanceof Error ? e.message : "Failed to remove member");
        } finally {
          setMemberActionId(null);
        }
      },
    });
  };

  const handleCreateBot = async () => {
    if (!workspace?.id || !newBotName.trim()) return;
    setCreatingBot(true);
    try {
      await api.createBotUser(workspace.id, { name: newBotName.trim() });
      toast.success("Bot user created");
      setNewBotName("");
      setCreateBotOpen(false);
      await reloadBots();
      // Bots are also workspace members, so refresh that list too.
      await refreshMembers();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to create bot");
    } finally {
      setCreatingBot(false);
    }
  };

  const handleDeleteBot = (bot: BotUser) => {
    if (!workspace?.id) return;
    const usage = bot.webhook_count > 0
      ? ` ${bot.webhook_count} webhook${bot.webhook_count === 1 ? "" : "s"} bound to this bot will lose their author and any "comment on linked issue" actions will fail until you assign a new bot.`
      : "";
    setConfirmAction({
      title: `Delete bot ${bot.name}`,
      description: `Permanently delete this bot user.${usage}`,
      variant: "destructive",
      onConfirm: async () => {
        setMemberActionId(bot.id);
        try {
          await api.deleteBotUser(workspace.id, bot.id);
          toast.success("Bot user deleted");
          await reloadBots();
          await refreshMembers();
        } catch (e) {
          toast.error(e instanceof Error ? e.message : "Failed to delete bot");
        } finally {
          setMemberActionId(null);
        }
      },
    });
  };

  if (!workspace) return null;

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <Users className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">Members ({humanMembers.length})</h2>
        </div>

        {canManageWorkspace && (
          <Card>
            <CardContent className="space-y-3">
              <div className="flex items-center gap-2">
                <Plus className="h-4 w-4 text-muted-foreground" />
                <h3 className="text-sm font-medium">Add member</h3>
              </div>
              <div className="grid gap-3 sm:grid-cols-[1fr_120px_auto]">
                <Input
                  type="email"
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  placeholder="user@company.com"
                />
                <Select value={inviteRole} onValueChange={(value) => setInviteRole(value as MemberRole)}>
                  <SelectTrigger size="sm"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="member">Member</SelectItem>
                    <SelectItem value="admin">Admin</SelectItem>
                    {isOwner && <SelectItem value="owner">Owner</SelectItem>}
                  </SelectContent>
                </Select>
                <Button
                  onClick={handleAddMember}
                  disabled={inviteLoading || !inviteEmail.trim()}
                >
                  {inviteLoading ? "Adding..." : "Add"}
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        {humanMembers.length > 0 ? (
          <div className="overflow-hidden rounded-xl ring-1 ring-foreground/10">
            {humanMembers.map((m, i) => (
              <div key={m.id} className={i > 0 ? "border-t border-border/50" : ""}>
                <MemberRow
                  member={m}
                  canManage={canManageWorkspace}
                  canManageOwners={isOwner}
                  isSelf={m.user_id === user?.id}
                  busy={memberActionId === m.id}
                  onRoleChange={(role) => handleRoleChange(m.id, role)}
                  onRemove={() => handleRemoveMember(m)}
                />
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No members found.</p>
        )}
      </section>

      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Bot className="h-4 w-4 text-muted-foreground" />
            <h2 className="text-sm font-semibold">Bots ({bots.length})</h2>
          </div>
          {canManageWorkspace && (
            <Button size="sm" variant="outline" onClick={() => setCreateBotOpen(true)}>
              <Plus className="h-3.5 w-3.5 mr-1" />
              Add bot
            </Button>
          )}
        </div>
        <p className="text-xs text-muted-foreground">
          Bot users post comments on behalf of webhooks (e.g. the GitHub App webhook&apos;s
          &ldquo;comment on linked issue&rdquo; action). They appear in mention lists like
          regular members but cannot log in.
        </p>

        {botsLoading ? (
          <Card>
            <CardContent className="py-4 text-center text-xs text-muted-foreground">
              Loading...
            </CardContent>
          </Card>
        ) : bots.length === 0 ? (
          <Card>
            <CardContent className="py-4 text-center text-xs text-muted-foreground">
              No bot users yet.
            </CardContent>
          </Card>
        ) : (
          <div className="overflow-hidden rounded-xl ring-1 ring-foreground/10">
            {bots.map((b, i) => (
              <div key={b.id} className={i > 0 ? "border-t border-border/50" : ""}>
                <BotRow
                  bot={b}
                  canManage={canManageWorkspace}
                  busy={memberActionId === b.id}
                  onDelete={() => handleDeleteBot(b)}
                />
              </div>
            ))}
          </div>
        )}
      </section>

      <Dialog open={createBotOpen} onOpenChange={setCreateBotOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Create bot user</DialogTitle>
            <DialogDescription>
              The bot is added to this workspace as a regular member with the Member role.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="new-bot-name">Name</Label>
            <Input
              id="new-bot-name"
              value={newBotName}
              onChange={(e) => setNewBotName(e.target.value)}
              placeholder="e.g. GitHub Bot"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateBotOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateBot} disabled={creatingBot || !newBotName.trim()}>
              {creatingBot ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!confirmAction} onOpenChange={(v) => { if (!v) setConfirmAction(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmAction?.title}</AlertDialogTitle>
            <AlertDialogDescription>{confirmAction?.description}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant={confirmAction?.variant === "destructive" ? "destructive" : "default"}
              onClick={async () => {
                await confirmAction?.onConfirm();
                setConfirmAction(null);
              }}
            >
              Confirm
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
