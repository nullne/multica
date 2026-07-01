"use client";

import { useEffect, useState } from "react";
import { Loader2, Save, Send, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";
import { api } from "@/shared/api";
import {
  normalizeTelegramPreferences,
  personalTelegramPreferenceCategories,
  type NotificationChannel,
  type TelegramPreferenceCategory,
} from "@/shared/types";

export function NotificationsTab() {
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
        preferences: normalizeTelegramPreferences(telegramChannel?.preferences),
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
        preferences: normalizeTelegramPreferences(telegramChannel.preferences),
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

  const handleTelegramPreferenceToggle = async (
    category: TelegramPreferenceCategory,
    enabled: boolean,
  ) => {
    if (!telegramChannel) return;
    const preferences = {
      ...normalizeTelegramPreferences(telegramChannel.preferences),
      [category]: enabled,
    };
    setTelegramSaving(true);
    try {
      const updated = await api.upsertTelegramChannel({
        chat_id: telegramChannel.channel_id,
        enabled: telegramChannel.enabled,
        preferences,
      });
      setTelegramChannel(updated);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update Telegram preferences");
    } finally {
      setTelegramSaving(false);
    }
  };

  return (
    <div className="space-y-8">
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
                  <div className="space-y-4 border-t pt-4">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs text-muted-foreground">Enable Telegram notifications</Label>
                      <Switch
                        checked={telegramChannel.enabled}
                        onCheckedChange={handleTelegramToggle}
                        disabled={telegramSaving}
                      />
                    </div>

                    <div className="space-y-3">
                      <div>
                        <p className="text-xs font-medium">Notification types</p>
                        <p className="text-xs text-muted-foreground">
                          Choose which private Telegram notifications you receive.
                        </p>
                      </div>
                      {personalTelegramPreferenceCategories.map((category) => {
                        const preferences = normalizeTelegramPreferences(telegramChannel.preferences);
                        return (
                          <div key={category.id} className="flex items-start justify-between gap-4">
                            <div className="space-y-0.5">
                              <Label className="text-xs">{category.label}</Label>
                              <p className="text-xs text-muted-foreground">{category.description}</p>
                            </div>
                            <Switch
                              checked={preferences[category.id]}
                              onCheckedChange={(enabled) => handleTelegramPreferenceToggle(category.id, enabled)}
                              disabled={telegramSaving}
                              aria-label={`Toggle ${category.label}`}
                            />
                          </div>
                        );
                      })}
                    </div>
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
