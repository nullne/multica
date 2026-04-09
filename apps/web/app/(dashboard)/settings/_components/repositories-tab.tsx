"use client";

import { useEffect, useState } from "react";
import { Save, Plus, Trash2, Github, Unplug, ExternalLink } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import type { WorkspaceRepo } from "@/shared/types";

export function RepositoriesTab() {
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);
  const updateWorkspace = useWorkspaceStore((s) => s.updateWorkspace);

  const [repos, setRepos] = useState<WorkspaceRepo[]>(workspace?.repos ?? []);
  const [saving, setSaving] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);

  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManageWorkspace = currentMember?.role === "owner" || currentMember?.role === "admin";

  useEffect(() => {
    setRepos(workspace?.repos ?? []);
  }, [workspace]);

  const handleSave = async () => {
    if (!workspace) return;
    setSaving(true);
    try {
      const updated = await api.updateWorkspace(workspace.id, { repos });
      updateWorkspace(updated);
      toast.success("Repositories saved");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save repositories");
    } finally {
      setSaving(false);
    }
  };

  const handleAddRepo = () => {
    setRepos([...repos, { url: "", description: "" }]);
  };

  const handleRemoveRepo = (index: number) => {
    setRepos(repos.filter((_, i) => i !== index));
  };

  const handleRepoChange = (index: number, field: keyof WorkspaceRepo, value: string) => {
    setRepos(repos.map((r, i) => (i === index ? { ...r, [field]: value } : r)));
  };

  const handleConnectGitHub = async () => {
    if (!workspace) return;
    setConnecting(true);
    try {
      const { url } = await api.getGitHubInstallURL(workspace.id);
      window.open(url, "_blank");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to get install URL");
    } finally {
      setConnecting(false);
    }
  };

  const handleDisconnectGitHub = async () => {
    if (!workspace) return;
    setDisconnecting(true);
    try {
      const updated = await api.disconnectGitHub(workspace.id);
      updateWorkspace(updated);
      toast.success("GitHub disconnected");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to disconnect");
    } finally {
      setDisconnecting(false);
    }
  };

  if (!workspace) return null;

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">GitHub Integration</h2>

        <Card>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Github className="h-4 w-4" />
                <span className="text-sm font-medium">GitHub App</span>
                {workspace.github_connected ? (
                  <Badge variant="secondary" className="text-xs">Connected</Badge>
                ) : (
                  <Badge variant="outline" className="text-xs">Not connected</Badge>
                )}
              </div>
              {canManageWorkspace && (
                <div>
                  {workspace.github_connected ? (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleDisconnectGitHub}
                      disabled={disconnecting}
                    >
                      <Unplug className="h-3 w-3" />
                      {disconnecting ? "Disconnecting..." : "Disconnect"}
                    </Button>
                  ) : (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleConnectGitHub}
                      disabled={connecting}
                    >
                      <ExternalLink className="h-3 w-3" />
                      {connecting ? "Opening..." : "Connect GitHub"}
                    </Button>
                  )}
                </div>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              {workspace.github_connected
                ? "Agents receive scoped GitHub tokens automatically. Each agent's code access level controls what it can do."
                : "Connect a GitHub App to give agents scoped access tokens for repositories. Without this, agents rely on host-level git credentials."}
            </p>
          </CardContent>
        </Card>
      </section>

      <section className="space-y-4">
        <h2 className="text-sm font-semibold">Repositories</h2>

        <Card>
          <CardContent className="space-y-3">
            <p className="text-xs text-muted-foreground">
              GitHub repositories associated with this workspace. Agents use these to clone and work on code.
            </p>

            {repos.map((repo, index) => (
              <div key={index} className="flex gap-2">
                <div className="flex-1 space-y-1.5">
                  <Input
                    type="url"
                    value={repo.url}
                    onChange={(e) => handleRepoChange(index, "url", e.target.value)}
                    disabled={!canManageWorkspace}
                    placeholder="https://github.com/org/repo"
                    className="text-sm"
                  />
                  <Input
                    type="text"
                    value={repo.description}
                    onChange={(e) => handleRepoChange(index, "description", e.target.value)}
                    disabled={!canManageWorkspace}
                    placeholder="Description (e.g. Go backend + Next.js frontend)"
                    className="text-sm"
                  />
                </div>
                {canManageWorkspace && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="mt-0.5 shrink-0 text-muted-foreground hover:text-destructive"
                    onClick={() => handleRemoveRepo(index)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                )}
              </div>
            ))}

            {canManageWorkspace && (
              <div className="flex items-center justify-between pt-1">
                <Button variant="outline" size="sm" onClick={handleAddRepo}>
                  <Plus className="h-3 w-3" />
                  Add repository
                </Button>
                <Button
                  size="sm"
                  onClick={handleSave}
                  disabled={saving}
                >
                  <Save className="h-3 w-3" />
                  {saving ? "Saving..." : "Save"}
                </Button>
              </div>
            )}

            {!canManageWorkspace && (
              <p className="text-xs text-muted-foreground">
                Only admins and owners can manage repositories.
              </p>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
