"use client";

import { useEffect, useRef, useState } from "react";
import { Camera, Loader2, Save, Send, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";
import { useAuthStore } from "@/features/auth";
import { api } from "@/shared/api";
import { useFileUpload } from "@/shared/hooks/use-file-upload";
import type { NotificationChannel } from "@/shared/types";

export function AccountTab() {
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);

  const [profileName, setProfileName] = useState(user?.name ?? "");
  const [profileSaving, setProfileSaving] = useState(false);
  const { upload, uploading } = useFileUpload();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [telegramChannel, setTelegramChannel] = useState<NotificationChannel | null>(null);
  const [telegramChatId, setTelegramChatId] = useState("");
  const [telegramSaving, setTelegramSaving] = useState(false);
  const [telegramLoading, setTelegramLoading] = useState(true);

  useEffect(() => {
    api.listNotificationChannels().then((channels) => {
      const tg = channels.find((c) => c.channel_type === "telegram") ?? null;
      setTelegramChannel(tg);
      if (tg) setTelegramChatId(tg.channel_id);
    }).catch(() => {
      // silently ignore — not critical
    }).finally(() => setTelegramLoading(false));
  }, []);

  const handleTelegramSave = async () => {
    const chatId = telegramChatId.trim();
    if (!chatId) return;
    setTelegramSaving(true);
    try {
      const updated = await api.upsertTelegramChannel({
        chat_id: chatId,
        enabled: telegramChannel?.enabled ?? true,
      });
      setTelegramChannel(updated);
      toast.success("Telegram channel saved");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save Telegram channel");
    } finally {
      setTelegramSaving(false);
    }
  };

  const handleTelegramToggle = async (enabled: boolean) => {
    if (!telegramChannel) return;
    try {
      const updated = await api.upsertTelegramChannel({
        chat_id: telegramChannel.channel_id,
        enabled,
      });
      setTelegramChannel(updated);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update Telegram channel");
    }
  };

  const handleTelegramDelete = async () => {
    try {
      await api.deleteTelegramChannel();
      setTelegramChannel(null);
      setTelegramChatId("");
      toast.success("Telegram channel removed");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to remove Telegram channel");
    }
  };

  useEffect(() => {
    setProfileName(user?.name ?? "");
  }, [user]);

  const initials = (user?.name ?? "")
    .split(" ")
    .map((w) => w[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);

  const handleAvatarUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    // Reset input so the same file can be re-selected
    e.target.value = "";
    try {
      const result = await upload(file);
      if (!result) return;
      const updated = await api.updateMe({ avatar_url: result.link });
      setUser(updated);
      toast.success("Avatar updated");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to upload avatar");
    }
  };

  const handleProfileSave = async () => {
    setProfileSaving(true);
    try {
      const updated = await api.updateMe({ name: profileName });
      setUser(updated);
      toast.success("Profile updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update profile");
    } finally {
      setProfileSaving(false);
    }
  };

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">Profile</h2>


        <Card>
          <CardContent className="space-y-4">
            {/* Avatar upload */}
            <div className="flex items-center gap-4">
              <button
                type="button"
                className="group relative h-16 w-16 shrink-0 rounded-full bg-muted overflow-hidden focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploading}
              >
                {user?.avatar_url ? (
                  <img
                    src={user.avatar_url}
                    alt={user.name}
                    className="h-full w-full object-cover"
                  />
                ) : (
                  <span className="flex h-full w-full items-center justify-center text-lg font-semibold text-muted-foreground">
                    {initials}
                  </span>
                )}
                <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
                  {uploading ? (
                    <Loader2 className="h-5 w-5 animate-spin text-white" />
                  ) : (
                    <Camera className="h-5 w-5 text-white" />
                  )}
                </div>
              </button>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={handleAvatarUpload}
              />
              <div className="text-xs text-muted-foreground">
                Click to upload avatar
              </div>
            </div>

            <div>
              <Label className="text-xs text-muted-foreground">Name</Label>
              <Input
                type="search"
                value={profileName}
                onChange={(e) => setProfileName(e.target.value)}
                className="mt-1"
              />
            </div>
            <div className="flex items-center justify-end gap-2 pt-1">
              <Button
                size="sm"
                onClick={handleProfileSave}
                disabled={profileSaving || !profileName.trim()}
              >
                <Save className="h-3 w-3" />
                {profileSaving ? "Updating..." : "Update Profile"}
              </Button>
            </div>
          </CardContent>
        </Card>
      </section>
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">Notifications</h2>
        <Card>
          <CardContent className="space-y-4">
            <div className="flex items-center gap-2">
              <Send className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-medium">Telegram</span>
            </div>
            <p className="text-xs text-muted-foreground">
              Connect your Telegram account to receive subscriber notifications outside the app.
              Start a chat with your bot, then enter your chat ID below.
            </p>
            {telegramLoading ? (
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <Loader2 className="h-3 w-3 animate-spin" />
                Loading…
              </div>
            ) : (
              <div className="space-y-3">
                <div>
                  <Label className="text-xs text-muted-foreground">Chat ID or @username</Label>
                  <div className="mt-1 flex items-center gap-2">
                    <Input
                      value={telegramChatId}
                      onChange={(e) => setTelegramChatId(e.target.value)}
                      placeholder="e.g. 123456789 or @username"
                      className="flex-1"
                    />
                    <Button
                      size="sm"
                      onClick={handleTelegramSave}
                      disabled={telegramSaving || !telegramChatId.trim()}
                    >
                      {telegramSaving ? (
                        <Loader2 className="h-3 w-3 animate-spin" />
                      ) : (
                        <Save className="h-3 w-3" />
                      )}
                      {telegramChannel ? "Update" : "Connect"}
                    </Button>
                    {telegramChannel && (
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={handleTelegramDelete}
                        className="text-destructive hover:text-destructive"
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    )}
                  </div>
                </div>
                {telegramChannel && (
                  <div className="flex items-center justify-between">
                    <Label className="text-xs text-muted-foreground">Enable Telegram notifications</Label>
                    <Switch
                      checked={telegramChannel.enabled}
                      onCheckedChange={handleTelegramToggle}
                    />
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
