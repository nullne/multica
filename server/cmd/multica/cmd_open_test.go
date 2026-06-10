package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func createOpenTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in test environment")
	}
	dir := t.TempDir()
	openGitRun(t, "", "init", dir)
	openGitRun(t, dir, "commit", "--allow-empty", "-m", "initial")
	return dir
}

func openGitRun(t *testing.T, repo string, args ...string) string {
	t.Helper()
	if repo != "" {
		args = append([]string{"-C", repo}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %s: %v", strings.Join(args, " "), out, err)
	}
	return strings.TrimSpace(string(out))
}

func openGitHead(t *testing.T, repo string) string {
	t.Helper()
	return openGitRun(t, repo, "rev-parse", "HEAD")
}

func openTestProject(repoURL string) project {
	return project{
		WorkspaceName: "Workspace",
		WorkspaceID:   "ws-1",
		RepoName:      "repo",
		RepoURL:       repoURL,
	}
}

func TestBarePathDistinguishesSameNameRepos(t *testing.T) {
	wsRoot := t.TempDir()
	first := openTestProject("/org-one/repo.git")
	second := openTestProject("/org-two/repo.git")

	if got, want := barePath(wsRoot, first), barePath(wsRoot, second); got == want {
		t.Fatalf("barePath() = %q for distinct repo URLs", got)
	}
}

func TestEnsureBareCloneRefreshesRemoteTrackingDefaultBranch(t *testing.T) {
	source := createOpenTestRepo(t)
	wsRoot := t.TempDir()
	proj := openTestProject(source)
	bare := barePath(wsRoot, proj)

	if err := ensureBareClone(source, bare); err != nil {
		t.Fatalf("initial ensureBareClone() error = %v", err)
	}
	openGitRun(t, source, "commit", "--allow-empty", "-m", "second")
	want := openGitHead(t, source)

	if err := ensureBareClone(source, bare); err != nil {
		t.Fatalf("refresh ensureBareClone() error = %v", err)
	}

	wtPath, err := createWorktree(bare, wsRoot, proj)
	if err != nil {
		t.Fatalf("createWorktree() error = %v", err)
	}
	if got := openGitHead(t, wtPath); got != want {
		t.Fatalf("worktree HEAD = %s, want refreshed source HEAD %s", got, want)
	}
}

func TestEnsureBareCloneFetchFailurePreventsStaleWorktree(t *testing.T) {
	source := createOpenTestRepo(t)
	wsRoot := t.TempDir()
	proj := openTestProject(source)
	bare := barePath(wsRoot, proj)

	if err := ensureBareClone(source, bare); err != nil {
		t.Fatalf("initial ensureBareClone() error = %v", err)
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatalf("RemoveAll(source) error = %v", err)
	}

	err := ensureBareClone(source, bare)
	if err == nil {
		t.Fatal("ensureBareClone() error = nil, want fetch failure")
	}
	if !strings.Contains(err.Error(), "git fetch origin") {
		t.Fatalf("ensureBareClone() error = %v, want git fetch origin error", err)
	}

	workspaceDir := filepath.Join(wsRoot, sanitizeDirName(proj.WorkspaceName))
	if _, err := os.Stat(workspaceDir); !os.IsNotExist(err) {
		t.Fatalf("workspace dir exists after fetch failure: %v", err)
	}
}
