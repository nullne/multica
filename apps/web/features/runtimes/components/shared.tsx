import { Monitor, Cloud, Wifi, WifiOff, RefreshCw, ShieldCheck, ShieldAlert, ShieldX } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { ReactNode } from "react";
import type { ProviderAuthStatus } from "@/shared/types";

export function RuntimeModeIcon({ mode }: { mode: string }) {
  return mode === "cloud" ? (
    <Cloud className="h-3.5 w-3.5" />
  ) : (
    <Monitor className="h-3.5 w-3.5" />
  );
}

export function StatusBadge({ status }: { status: string }) {
  if (status === "updating") {
    return (
      <Badge variant="secondary" className="bg-info/10 text-info">
        <RefreshCw className="h-3 w-3 animate-spin" />
        Updating
      </Badge>
    );
  }
  const isOnline = status === "online";
  return (
    <Badge
      variant="secondary"
      className={isOnline ? "bg-success/10 text-success" : ""}
    >
      {isOnline ? (
        <Wifi className="h-3 w-3" />
      ) : (
        <WifiOff className="h-3 w-3" />
      )}
      {isOnline ? "Online" : "Offline"}
    </Badge>
  );
}

export function InfoField({
  label,
  value,
  mono,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
}) {
  return (
    <div>
      <div className="text-xs text-muted-foreground">{label}</div>
      <div
        className={`mt-0.5 text-sm truncate ${mono ? "font-mono text-xs" : ""}`}
      >
        {value}
      </div>
    </div>
  );
}

const authStatusConfig: Record<ProviderAuthStatus, { label: string; className: string; Icon: typeof ShieldCheck }> = {
  ready: { label: "Ready", className: "bg-success/10 text-success", Icon: ShieldCheck },
  unauthenticated: { label: "Needs Auth", className: "bg-warning/10 text-warning", Icon: ShieldAlert },
  not_installed: { label: "Not Installed", className: "bg-destructive/10 text-destructive", Icon: ShieldX },
  unknown: { label: "Unknown", className: "", Icon: ShieldAlert },
};

export function AuthStatusBadge({ authStatus }: { authStatus: ProviderAuthStatus }) {
  const config = authStatusConfig[authStatus] ?? authStatusConfig.unknown;
  const { Icon } = config;
  return (
    <Badge variant="secondary" className={config.className}>
      <Icon className="h-3 w-3" />
      {config.label}
    </Badge>
  );
}

export function TokenCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border px-3 py-2">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-sm font-semibold tabular-nums">{value}</div>
    </div>
  );
}
