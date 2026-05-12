"use client";

import { useSearchParams } from "next/navigation";
import { Bell, Key, Monitor, Palette, User } from "lucide-react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ProfileTab } from "./_components/profile-tab";
import { AppearanceTab } from "./_components/appearance-tab";
import { TokensTab } from "./_components/tokens-tab";
import { NotificationsTab } from "./_components/notifications-tab";
import { MyDaemonsTab } from "./_components/my-daemons-tab";

const accountTabs = [
  { value: "profile", label: "Profile", icon: User },
  { value: "daemons", label: "My Daemons", icon: Monitor },
  { value: "appearance", label: "Appearance", icon: Palette },
  { value: "notifications", label: "Notifications", icon: Bell },
  { value: "tokens", label: "API Tokens", icon: Key },
];

export default function AccountPage() {
  const searchParams = useSearchParams();
  const tab = searchParams.get("tab");
  const defaultTab = accountTabs.some((item) => item.value === tab) ? tab! : "profile";

  return (
    <Tabs defaultValue={defaultTab} orientation="vertical" className="flex-1 min-h-0 gap-0">
      {/* Left nav */}
      <div className="w-52 shrink-0 border-r overflow-y-auto p-4">
        <h1 className="text-sm font-semibold mb-4 px-2">Account</h1>
        <TabsList variant="line" className="flex-col items-stretch">
          {accountTabs.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value}>
              <tab.icon className="h-4 w-4" />
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </div>

      {/* Right content */}
      <div className="flex-1 min-w-0 overflow-y-auto">
        <div className="w-full max-w-3xl mx-auto p-6">
          <TabsContent value="profile"><ProfileTab /></TabsContent>
          <TabsContent value="daemons"><MyDaemonsTab /></TabsContent>
          <TabsContent value="appearance"><AppearanceTab /></TabsContent>
          <TabsContent value="notifications"><NotificationsTab /></TabsContent>
          <TabsContent value="tokens"><TokensTab /></TabsContent>
        </div>
      </div>
    </Tabs>
  );
}
