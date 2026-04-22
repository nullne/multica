package repocache

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.Default()
}

func TestBareDirName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, want string
	}{
		{"https://github.com/org/my-repo.git", "my-repo.git"},
		{"https://github.com/org/my-repo", "my-repo.git"},
		{"git@github.com:org/my-repo.git", "my-repo.git"},
		{"git@github.com:org/my-repo", "my-repo.git"},
		{"https://github.com/org/repo/", "repo.git"},
		{"my-repo", "my-repo.git"},
		{"", "repo.git"},
	}
	for _, tt := range tests {
		if got := bareDirName(tt.input); got != tt.want {
			t.Errorf("bareDirName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsBareRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
	if !isBareRepo(dir) {
		t.Error("expected bare repo to be detected")
	}

	emptyDir := t.TempDir()
	if isBareRepo(emptyDir) {
		t.Error("expected empty dir to not be detected as bare repo")
	}
}

// createTestRepo creates a local git repo with an initial commit and returns its path.
func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git setup failed: %s: %v", out, err)
		}
	}
	return dir
}

func gitHead(t *testing.T, repoPath string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD failed in %s: %v", repoPath, err)
	}
	return strings.TrimSpace(string(out))
}

// TestCreateWorktree exercises the happy path: bare cache does not exist, so
// CreateWorktree lazily clones, then materializes a worktree on the agent
// branch.
func TestCreateWorktree(t *testing.T) {
	t.Parallel()
	sourceRepo := createTestRepo(t)
	cache := New(t.TempDir(), testLogger())

	workDir := t.TempDir()
	result, err := cache.CreateWorktree(WorktreeParams{
		WorkspaceID: "ws-1",
		RepoURL:     sourceRepo,
		WorkDir:     workDir,
		AgentName:   "Code Reviewer",
		TaskID:      "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if _, err := os.Stat(result.Path); os.IsNotExist(err) {
		t.Fatalf("worktree path does not exist: %s", result.Path)
	}

	if !strings.HasPrefix(result.BranchName, "agent/code-reviewer/") {
		t.Errorf("expected branch to start with 'agent/code-reviewer/', got %q", result.BranchName)
	}

	cmd := exec.Command("git", "-C", result.Path, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("show branch failed: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != result.BranchName {
		t.Errorf("expected branch %q, got %q", result.BranchName, got)
	}
}

// TestCreateWorktreeLazyClonePopulatesCache verifies the bare cache is
// populated as a side effect of the first CreateWorktree call (no separate
// sync step needed).
func TestCreateWorktreeLazyClonePopulatesCache(t *testing.T) {
	t.Parallel()
	sourceRepo := createTestRepo(t)
	cache := New(t.TempDir(), testLogger())

	if got := cache.Lookup("ws-1", sourceRepo); got != "" {
		t.Fatalf("expected empty cache before first checkout, got %q", got)
	}

	if _, err := cache.CreateWorktree(WorktreeParams{
		WorkspaceID: "ws-1",
		RepoURL:     sourceRepo,
		WorkDir:     t.TempDir(),
		AgentName:   "agent",
		TaskID:      "task-1",
	}); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	barePath := cache.Lookup("ws-1", sourceRepo)
	if barePath == "" {
		t.Fatal("expected cache to be populated after lazy clone")
	}
	if !isBareRepo(barePath) {
		t.Fatalf("expected bare repo at %s", barePath)
	}
}

// TestCreateWorktreeFetchesLatest verifies that a second CreateWorktree call
// picks up new commits from origin (i.e. CreateWorktree fetches before
// branching, not just on first clone).
func TestCreateWorktreeFetchesLatest(t *testing.T) {
	t.Parallel()
	sourceRepo := createTestRepo(t)
	cache := New(t.TempDir(), testLogger())

	if _, err := cache.CreateWorktree(WorktreeParams{
		WorkspaceID: "ws-1",
		RepoURL:     sourceRepo,
		WorkDir:     t.TempDir(),
		AgentName:   "agent",
		TaskID:      "task-1",
	}); err != nil {
		t.Fatalf("first CreateWorktree failed: %v", err)
	}

	barePath := cache.Lookup("ws-1", sourceRepo)
	oldHead := gitHead(t, barePath)

	// Add a new commit to the source.
	cmd := exec.Command("git", "-C", sourceRepo, "commit", "--allow-empty", "-m", "second")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add commit failed: %s: %v", out, err)
	}
	sourceHead := gitHead(t, sourceRepo)
	if sourceHead == oldHead {
		t.Fatal("source HEAD should differ after new commit")
	}

	// Second CreateWorktree should fetch the new commit.
	if _, err := cache.CreateWorktree(WorktreeParams{
		WorkspaceID: "ws-1",
		RepoURL:     sourceRepo,
		WorkDir:     t.TempDir(),
		AgentName:   "agent",
		TaskID:      "task-2",
	}); err != nil {
		t.Fatalf("second CreateWorktree failed: %v", err)
	}

	newHead := gitHead(t, barePath)
	if newHead != sourceHead {
		t.Fatalf("expected cache HEAD %s to match source HEAD %s after fetch", newHead, sourceHead)
	}
}

// TestCreateWorktreeFetchFailureHardErrors is the key regression guard for
// this fix: when fetch fails (origin gone, network error, bad token, ...),
// CreateWorktree must NOT silently fall back to the stale cached state and
// produce a worktree based on outdated code. It must return an error and not
// leave a worktree behind.
func TestCreateWorktreeFetchFailureHardErrors(t *testing.T) {
	t.Parallel()
	sourceRepo := createTestRepo(t)
	cache := New(t.TempDir(), testLogger())

	if _, err := cache.CreateWorktree(WorktreeParams{
		WorkspaceID: "ws-1",
		RepoURL:     sourceRepo,
		WorkDir:     t.TempDir(),
		AgentName:   "agent",
		TaskID:      "task-1",
	}); err != nil {
		t.Fatalf("initial CreateWorktree failed: %v", err)
	}

	// Make origin unreachable.
	if err := os.RemoveAll(sourceRepo); err != nil {
		t.Fatalf("remove source repo failed: %v", err)
	}

	workDir := t.TempDir()
	result, err := cache.CreateWorktree(WorktreeParams{
		WorkspaceID: "ws-1",
		RepoURL:     sourceRepo,
		WorkDir:     workDir,
		AgentName:   "agent",
		TaskID:      "task-2",
	})
	if err == nil {
		t.Fatalf("expected fetch failure to produce error, got worktree at %q", result.Path)
	}
	if !strings.Contains(err.Error(), "fetch") {
		t.Errorf("expected fetch error, got: %v", err)
	}

	// No worktree should have been created.
	entries, _ := os.ReadDir(workDir)
	if len(entries) != 0 {
		t.Errorf("expected empty workDir after fetch failure, got entries: %v", entries)
	}
}

func TestCreateWorktreeRejectsMissingTaskContext(t *testing.T) {
	t.Parallel()

	cache := New(t.TempDir(), testLogger())
	_, err := cache.CreateWorktree(WorktreeParams{
		WorkspaceID: "ws-1",
		RepoURL:     createTestRepo(t),
		WorkDir:     t.TempDir(),
		AgentName:   "",
		TaskID:      "",
	})
	if err == nil {
		t.Fatal("expected error for missing task context")
	}
	if got, want := err.Error(), "agent name is required"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

// TestCreateWorktreeLazyCloneFailure verifies that a non-existent source URL
// fails the lazy clone with a clear error (and does not leave a half-baked
// cache directory).
func TestCreateWorktreeLazyCloneFailure(t *testing.T) {
	t.Parallel()
	cacheRoot := t.TempDir()
	cache := New(cacheRoot, testLogger())

	bogusURL := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := cache.CreateWorktree(WorktreeParams{
		WorkspaceID: "ws-1",
		RepoURL:     bogusURL,
		WorkDir:     t.TempDir(),
		AgentName:   "agent",
		TaskID:      "task-1",
	})
	if err == nil {
		t.Fatal("expected error cloning a nonexistent repo")
	}
	if !strings.Contains(err.Error(), "clone") {
		t.Errorf("expected clone error, got: %v", err)
	}

	// The bare cache should not be left as a half-baked directory.
	if got := cache.Lookup("ws-1", bogusURL); got != "" {
		t.Errorf("expected no cache entry after failed clone, got %q", got)
	}
}
