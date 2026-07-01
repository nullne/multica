"use client";

import { useEffect, useState } from "react";
import { Save, LogOut, Send, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
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
import { toast } from "sonner";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import {
  normalizeTelegramPreferences,
  telegramPreferenceCategories,
  type TelegramPreferenceCategory,
  type WorkspaceTelegramSettings,
} from "@/shared/types";

export function WorkspaceTab() {
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);
  const updateWorkspace = useWorkspaceStore((s) => s.updateWorkspace);
  const leaveWorkspace = useWorkspaceStore((s) => s.leaveWorkspace);
  const deleteWorkspace = useWorkspaceStore((s) => s.deleteWorkspace);

  const [name, setName] = useState(workspace?.name ?? "");
  const [description, setDescription] = useState(workspace?.description ?? "");
  const [context, setContext] = useState(workspace?.context ?? "");
  const [saving, setSaving] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);

  const [telegramGroup, setTelegramGroup] = useState<WorkspaceTelegramSettings | null>(null);
  const [telegramChatId, setTelegramChatId] = useState("");
  const [telegramSaving, setTelegramSaving] = useState(false);
  const [telegramLoading, setTelegramLoading] = useState(true);
  const [confirmAction, setConfirmAction] = useState<{
    title: string;
    description: string;
    variant?: "destructive";
    onConfirm: () => Promise<void>;
  } | null>(null);

  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManageWorkspace = currentMember?.role === "owner" || currentMember?.role === "admin";
  const isOwner = currentMember?.role === "owner";

  useEffect(() => {
    setName(workspace?.name ?? "");
    setDescription(workspace?.description ?? "");
    setContext(workspace?.context ?? "");
  }, [workspace]);

  useEffect(() => {
    if (!workspace) return;
    api.getWorkspaceTelegramNotifications(workspace.id).then((s) => {
      setTelegramGroup(s);
      if (s.configured && s.chat_id) setTelegramChatId(s.chat_id);
    }).catch(() => {
      // silently ignore
    }).finally(() => setTelegramLoading(false));
  }, [workspace?.id]);

  const handleTelegramGroupSave = async () => {
    if (!workspace) return;
    const chatId = telegramChatId.trim();
    if (!chatId) return;
    setTelegramSaving(true);
    try {
      const updated = await api.upsertWorkspaceTelegramNotifications(workspace.id, {
        chat_id: chatId,
        enabled: telegramGroup?.enabled ?? true,
        preferences: normalizeTelegramPreferences(telegramGroup?.preferences),
      });
      setTelegramGroup(updated);
      toast.success("Telegram group saved");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save Telegram group");
    } finally {
      setTelegramSaving(false);
    }
  };

  const handleTelegramGroupToggle = async (enabled: boolean) => {
    if (!workspace || !telegramGroup?.configured || !telegramGroup.chat_id) return;
    try {
      const updated = await api.upsertWorkspaceTelegramNotifications(workspace.id, {
        chat_id: telegramGroup.chat_id,
        enabled,
        preferences: normalizeTelegramPreferences(telegramGroup.preferences),
      });
      setTelegramGroup(updated);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update Telegram group");
    }
  };

  const handleTelegramGroupDelete = async () => {
    if (!workspace) return;
    try {
      await api.deleteWorkspaceTelegramNotifications(workspace.id);
      setTelegramGroup({ configured: false });
      setTelegramChatId("");
      toast.success("Telegram group removed");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to remove Telegram group");
    }
  };

  const handleTelegramGroupPreferenceToggle = async (
    category: TelegramPreferenceCategory,
    enabled: boolean,
  ) => {
    if (!workspace || !telegramGroup?.configured || !telegramGroup.chat_id) return;
    const preferences = {
      ...normalizeTelegramPreferences(telegramGroup.preferences),
      [category]: enabled,
    };
    setTelegramSaving(true);
    try {
      const updated = await api.upsertWorkspaceTelegramNotifications(workspace.id, {
        chat_id: telegramGroup.chat_id,
        enabled: telegramGroup.enabled ?? true,
        preferences,
      });
      setTelegramGroup(updated);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update Telegram group preferences");
    } finally {
      setTelegramSaving(false);
    }
  };

  const handleSave = async () => {
    if (!workspace) return;
    setSaving(true);
    try {
      const updated = await api.updateWorkspace(workspace.id, {
        name,
        description,
        context,
      });
      updateWorkspace(updated);
      toast.success("Workspace settings saved");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save workspace settings");
    } finally {
      setSaving(false);
    }
  };

  const handleLeaveWorkspace = () => {
    if (!workspace) return;
    setConfirmAction({
      title: "Leave workspace",
      description: `Leave ${workspace.name}? You will lose access until re-invited.`,
      variant: "destructive",
      onConfirm: async () => {
        setActionId("leave");
        try {
          await leaveWorkspace(workspace.id);
        } catch (e) {
          toast.error(e instanceof Error ? e.message : "Failed to leave workspace");
        } finally {
          setActionId(null);
        }
      },
    });
  };

  const handleDeleteWorkspace = () => {
    if (!workspace) return;
    setConfirmAction({
      title: "Delete workspace",
      description: `Delete ${workspace.name}? This cannot be undone. All issues, agents, and data will be permanently removed.`,
      variant: "destructive",
      onConfirm: async () => {
        setActionId("delete-workspace");
        try {
          await deleteWorkspace(workspace.id);
        } catch (e) {
          toast.error(e instanceof Error ? e.message : "Failed to delete workspace");
        } finally {
          setActionId(null);
        }
      },
    });
  };

  if (!workspace) return null;

  return (
    <div className="space-y-8">
      {/* Workspace settings */}
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">General</h2>

        <Card>
          <CardContent className="space-y-3">
            <div>
              <Label className="text-xs text-muted-foreground">Name</Label>
              <Input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={!canManageWorkspace}
                className="mt-1"
              />
            </div>
            <div>
              <Label className="text-xs text-muted-foreground">Description</Label>
              <Textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
                disabled={!canManageWorkspace}
                className="mt-1 resize-none"
                placeholder="What does this workspace focus on?"
              />
            </div>
            <div>
              <Label className="text-xs text-muted-foreground">Context</Label>
              <Textarea
                value={context}
                onChange={(e) => setContext(e.target.value)}
                rows={4}
                disabled={!canManageWorkspace}
                className="mt-1 resize-none"
                placeholder="Background information and context for AI agents working in this workspace"
              />
            </div>
            <div>
              <Label className="text-xs text-muted-foreground">Slug</Label>
              <div className="mt-1 rounded-md border bg-muted/50 px-3 py-2 text-sm text-muted-foreground">
                {workspace.slug}
              </div>
            </div>
            <div className="flex items-center justify-end gap-2 pt-1">
              <Button
                size="sm"
                onClick={handleSave}
                disabled={saving || !name.trim() || !canManageWorkspace}
              >
                <Save className="h-3 w-3" />
                {saving ? "Saving..." : "Save"}
              </Button>
            </div>
            {!canManageWorkspace && (
              <p className="text-xs text-muted-foreground">
                Only admins and owners can update workspace settings.
              </p>
            )}
          </CardContent>
        </Card>
      </section>

      {/* Telegram Group Notifications */}
      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <Send className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">Telegram Group Notifications</h2>
        </div>

        <Card>
          <CardContent className="space-y-3">
            {telegramLoading ? (
              <p className="text-xs text-muted-foreground">Loading…</p>
            ) : (
              <>
                <p className="text-xs text-muted-foreground">
                  Send workspace activity (new issues, comments, status changes, task results) to a Telegram group chat.
                  Add the bot to your group and enter the group chat ID below.
                </p>
                <div>
                  <Label className="text-xs text-muted-foreground">Group Chat ID</Label>
                  <div className="mt-1 flex items-center gap-2">
                    <Input
                      type="text"
                      value={telegramChatId}
                      onChange={(e) => setTelegramChatId(e.target.value)}
                      placeholder="-1001234567890"
                      disabled={!canManageWorkspace}
                      className="flex-1"
                    />
                    {telegramGroup?.configured && (
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={handleTelegramGroupDelete}
                        disabled={!canManageWorkspace}
                        title="Remove Telegram group"
                      >
                        <Trash2 className="h-4 w-4 text-muted-foreground" />
                      </Button>
                    )}
                  </div>
                </div>

                {telegramGroup?.configured && (
                  <div className="space-y-4 border-t pt-4">
                    <div className="flex items-center gap-2">
                      <Switch
                        checked={telegramGroup.enabled ?? true}
                        onCheckedChange={handleTelegramGroupToggle}
                        disabled={!canManageWorkspace || telegramSaving}
                        id="telegram-group-enabled"
                      />
                      <Label htmlFor="telegram-group-enabled" className="text-xs text-muted-foreground cursor-pointer">
                        Notifications enabled
                      </Label>
                    </div>

                    <div className="space-y-3">
                      <div>
                        <p className="text-xs font-medium">Notification types</p>
                        <p className="text-xs text-muted-foreground">
                          Choose which updates are posted to the workspace Telegram group.
                        </p>
                      </div>
                      {telegramPreferenceCategories.map((category) => {
                        const preferences = normalizeTelegramPreferences(telegramGroup.preferences);
                        return (
                          <div key={category.id} className="flex items-start justify-between gap-4">
                            <div className="space-y-0.5">
                              <Label className="text-xs">{category.label}</Label>
                              <p className="text-xs text-muted-foreground">{category.description}</p>
                            </div>
                            <Switch
                              checked={preferences[category.id]}
                              onCheckedChange={(nextEnabled) => (
                                handleTelegramGroupPreferenceToggle(category.id, nextEnabled)
                              )}
                              disabled={!canManageWorkspace || telegramSaving}
                              aria-label={`Toggle ${category.label}`}
                            />
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}

                <div className="flex items-center justify-end gap-2 pt-1">
                  <Button
                    size="sm"
                    onClick={handleTelegramGroupSave}
                    disabled={telegramSaving || !telegramChatId.trim() || !canManageWorkspace}
                  >
                    <Save className="h-3 w-3" />
                    {telegramSaving ? "Saving…" : "Save"}
                  </Button>
                </div>

                {!canManageWorkspace && (
                  <p className="text-xs text-muted-foreground">
                    Only admins and owners can configure workspace notifications.
                  </p>
                )}
              </>
            )}
          </CardContent>
        </Card>
      </section>

      {/* Danger Zone */}
      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <LogOut className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-sm font-semibold">Danger Zone</h2>
        </div>

        <Card>
          <CardContent className="space-y-3">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p className="text-sm font-medium">Leave workspace</p>
                <p className="text-xs text-muted-foreground">
                  Remove yourself from this workspace.
                </p>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={handleLeaveWorkspace}
                disabled={actionId === "leave"}
              >
                {actionId === "leave" ? "Leaving..." : "Leave workspace"}
              </Button>
            </div>

            {isOwner && (
              <div className="flex flex-col gap-2 border-t pt-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p className="text-sm font-medium text-destructive">Delete workspace</p>
                  <p className="text-xs text-muted-foreground">
                    Permanently delete this workspace and its data.
                  </p>
                </div>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleDeleteWorkspace}
                  disabled={actionId === "delete-workspace"}
                >
                  {actionId === "delete-workspace" ? "Deleting..." : "Delete workspace"}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
      </section>

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
