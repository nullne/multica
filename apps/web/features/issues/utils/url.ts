export function issueUrl(issueId: string, workspaceSlug: string): string {
  return `/w/${workspaceSlug}/issues/${issueId}`;
}
