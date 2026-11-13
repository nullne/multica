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

func TestIsTerminalEditor(t *testing.T) {
	cases := []struct {
		editor string
		want   bool
	}{
		{"claude", true},
		{"codex", true},
		{"/usr/local/bin/claude", true},
		{"/opt/homebrew/bin/codex", true},
		{"cursor", false},
		{"code", false},
		{"/Applications/Cursor.app/Contents/MacOS/Cursor", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isTerminalEditor(c.editor); got != c.want {
			t.Errorf("isTerminalEditor(%q) = %v, want %v", c.editor, got, c.want)
		}
	}
}

func TestClaudeOpenFlags(t *testing.T) {
	cases := []struct {
		editor string
		want   string
	}{
		{"claude", " --dangerously-skip-permissions"},
		{"/usr/local/bin/claude", " --dangerously-skip-permissions"},
		{"codex", ""},
		{"cursor", ""},
	}
	for _, c := range cases {
		if got := claudeOpenFlags(c.editor); got != c.want {
			t.Errorf("claudeOpenFlags(%q) = %q, want %q", c.editor, got, c.want)
		}
	}
}

func TestOpenITermCommand(t *testing.T) {
	got := openITermCommand("/tmp/some dir", "claude")
	want := `cd '/tmp/some dir' && 'claude' --dangerously-skip-permissions; osascript -e 'tell application "iTerm" to close current session'`
	if got != want {
		t.Errorf("openITermCommand() = %q, want %q", got, want)
	}

	gotCodex := openITermCommand("/tmp/repo", "codex")
	if strings.Contains(gotCodex, "dangerously-skip-permissions") {
		t.Errorf("openITermCommand(codex) = %q, should not contain claude-only flag", gotCodex)
	}
	if !strings.Contains(gotCodex, "close current session") {
		t.Errorf("openITermCommand(codex) = %q, should close the session on exit", gotCodex)
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"simple", "'simple'"},
		{"/path/with space", "'/path/with space'"},
		{"a'b", `'a'\''b'`},
	}
	for _, c := range cases {
		if got := shellQuote(c.in); got != c.want {
			t.Errorf("shellQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAppleScriptString(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", `"hello"`},
		{`say "hi"`, `"say \"hi\""`},
		{`back\slash`, `"back\\slash"`},
	}
	for _, c := range cases {
		if got := appleScriptString(c.in); got != c.want {
			t.Errorf("appleScriptString(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
