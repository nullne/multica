import { CheckCircle2, XCircle, KeyRound, Terminal } from "lucide-react";
import type { AgentRuntime } from "@/shared/types";
import { formatLastSeen } from "../utils";
import { RuntimeModeIcon, StatusBadge, AuthStatusBadge, InfoField } from "./shared";
import { PingSection } from "./ping-section";
import { UpdateSection } from "./update-section";
import { UsageSection } from "./usage-section";

function getCliVersion(metadata: Record<string, unknown>): string | null {
  if (
    metadata &&
    typeof metadata.cli_version === "string" &&
    metadata.cli_version
  ) {
    return metadata.cli_version;
  }
  return null;
}

const loginCommands: Record<string, string> = {
  claude: "claude login",
  codex: "codex login",
  opencode: "opencode login",
  cursor: "cursor-agent login",
};

function AuthSection({ runtime }: { runtime: AgentRuntime }) {
  const loginCmd = loginCommands[runtime.provider] ?? `${runtime.provider} login`;

  if (runtime.auth_status === "ready") {
    return (
      <div className="rounded-lg border bg-success/5 p-4">
        <div className="flex items-center gap-2 text-success">
          <CheckCircle2 className="h-4 w-4" />
          <span className="text-sm font-medium">Provider authenticated and ready</span>
        </div>
      </div>
    );
  }

  if (runtime.auth_status === "not_installed") {
    return (
      <div className="rounded-lg border bg-destructive/5 p-4">
        <div className="flex items-center gap-2 text-destructive">
          <XCircle className="h-4 w-4" />
          <span className="text-sm font-medium">CLI not installed</span>
        </div>
        <p className="mt-2 text-xs text-muted-foreground">
          Install the <code className="rounded bg-muted px-1 py-0.5">{runtime.provider}</code> CLI
          on the daemon machine, then restart the daemon.
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-lg border bg-warning/5 p-4 space-y-3">
      <div className="flex items-center gap-2 text-warning">
        <KeyRound className="h-4 w-4" />
        <span className="text-sm font-medium">Authentication required</span>
      </div>
      <div>
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground mb-1.5">
          <Terminal className="h-3 w-3" />
          <span>Login on the machine</span>
        </div>
        <code className="block rounded bg-muted px-3 py-2 text-xs font-mono">
          {loginCmd}
        </code>
      </div>
      <p className="text-xs text-muted-foreground">
        Or configure an API key in{" "}
        <span className="font-medium">Settings &gt; Providers</span>.
      </p>
    </div>
  );
}

export function RuntimeDetail({ runtime }: { runtime: AgentRuntime }) {
  const cliVersion =
    runtime.runtime_mode === "local" ? getCliVersion(runtime.metadata) : null;

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex min-w-0 items-center gap-2">
          <div
            className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md ${
              runtime.status === "online" ? "bg-success/10" : "bg-muted"
            }`}
          >
            <RuntimeModeIcon mode={runtime.runtime_mode} />
          </div>
          <div className="min-w-0">
            <h2 className="text-sm font-semibold truncate">{runtime.name}</h2>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <AuthStatusBadge authStatus={runtime.auth_status} />
          <StatusBadge status={runtime.status} />
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {/* Auth Status */}
        {runtime.auth_status !== "unknown" && (
          <AuthSection runtime={runtime} />
        )}

        {/* Info grid */}
        <div className="grid grid-cols-2 gap-4">
          <InfoField label="Runtime Mode" value={runtime.runtime_mode} />
          <InfoField label="Provider" value={runtime.provider} />
          <InfoField label="Status" value={runtime.status} />
          <InfoField
            label="Last Seen"
            value={formatLastSeen(runtime.last_seen_at)}
          />
          {runtime.device_info && (
            <InfoField label="Device" value={runtime.device_info} />
          )}
          {runtime.daemon_id && (
            <InfoField label="Daemon ID" value={runtime.daemon_id} mono />
          )}
        </div>

        {/* CLI Version & Update */}
        {runtime.runtime_mode === "local" && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              CLI Version
            </h3>
            <UpdateSection
              runtimeId={runtime.id}
              currentVersion={cliVersion}
              isOnline={runtime.status === "online"}
            />
          </div>
        )}

        {/* Connection Test */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Connection Test
          </h3>
          <PingSection runtimeId={runtime.id} />
        </div>

        {/* Usage */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            Token Usage
          </h3>
          <UsageSection runtimeId={runtime.id} />
        </div>

        {/* Metadata */}
        {runtime.metadata && Object.keys(runtime.metadata).length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-2">
              Metadata
            </h3>
            <div className="rounded-lg border bg-muted/30 p-3">
              <pre className="text-xs font-mono whitespace-pre-wrap break-all">
                {JSON.stringify(runtime.metadata, null, 2)}
              </pre>
            </div>
          </div>
        )}

        {/* Timestamps */}
        <div className="grid grid-cols-2 gap-4 border-t pt-4">
          <InfoField
            label="Created"
            value={new Date(runtime.created_at).toLocaleString()}
          />
          <InfoField
            label="Updated"
            value={new Date(runtime.updated_at).toLocaleString()}
          />
        </div>
      </div>
    </div>
  );
}
