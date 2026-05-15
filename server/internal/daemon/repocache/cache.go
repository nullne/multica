// Package repocache manages bare git clone caches for workspace repositories.
// The daemon uses these caches as the source for creating per-task worktrees.
//
// All cache population is lazy: the bare clone for a repo is created the first
// time a task asks to check it out (CreateWorktree), using the GitHub token
// scoped to that task. Subsequent tasks reuse the cache and only fetch.
package repocache

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const remoteTrackingFetchRefspec = "+refs/heads/*:refs/remotes/origin/*"

// Cache manages bare git clones for workspace repositories.
type Cache struct {
	root   string // base directory for all caches (e.g. ~/multica_workspaces/.repos)
	logger *slog.Logger
	mu     sync.Mutex
}

// New creates a new repo cache rooted at the given directory.
func New(root string, logger *slog.Logger) *Cache {
	return &Cache{root: root, logger: logger}
}

// Lookup returns the local bare clone path for a repo URL within a workspace.
// Returns "" if not cached.
func (c *Cache) Lookup(workspaceID, url string) string {
	barePath := c.barePathFor(workspaceID, url)
	if isBareRepo(barePath) {
		return barePath
	}
	return ""
}

// barePathFor computes the on-disk path where a repo's bare clone would live.
// It does not check whether the clone actually exists.
func (c *Cache) barePathFor(workspaceID, url string) string {
	return filepath.Join(c.root, workspaceID, bareDirName(url))
}

// bareDirName derives a directory name from a repo URL.
// e.g. "https://github.com/org/my-repo.git" → "my-repo.git"
func bareDirName(url string) string {
	url = strings.TrimRight(url, "/")
	name := url
	if i := strings.LastIndex(url, "/"); i >= 0 {
		name = url[i+1:]
	}
	// Handle SSH-style "host:org/repo".
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[i+1:]
		if j := strings.LastIndex(name, "/"); j >= 0 {
			name = name[j+1:]
		}
	}
	if !strings.HasSuffix(name, ".git") {
		name += ".git"
	}
	if name == ".git" {
		name = "repo.git"
	}
	return name
}

// isBareRepo checks if a path looks like a bare git repository.
func isBareRepo(path string) bool {
	// A bare repo has a HEAD file at the root.
	_, err := os.Stat(filepath.Join(path, "HEAD"))
	return err == nil
}

func gitCloneBareWithToken(url, dest, token string) error {
	args := append([]string{"clone", "--bare"}, gitAuthArgs(token)...)
	args = append(args, url, dest)
	cmd := exec.Command("git", args...)
	if token != "" {
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dest)
		return fmt.Errorf("git clone --bare: %s: %w", strings.TrimSpace(string(out)), err)
	}
	if err := configureRemoteFetchRefspec(dest); err != nil {
		return err
	}
	if err := gitFetchWithToken(dest, token); err != nil {
		return err
	}
	return nil
}

func configureRemoteFetchRefspec(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "config", "--replace-all", "remote.origin.fetch", remoteTrackingFetchRefspec)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("configure fetch refspec: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func gitFetchWithToken(barePath, token string) error {
	args := append([]string{"-C", barePath}, gitAuthArgs(token)...)
	args = append(args, "fetch", "origin")
	cmd := exec.Command("git", args...)
	if token != "" {
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// gitAuthArgs returns the `-c` config args required to authenticate against
// HTTPS git remotes using a GitHub App installation token.
//
// The Authorization header is injected via http.extraHeader, and any
// credential helper inherited from the user's gitconfig (e.g. configured
// by `gh auth setup-git`) is disabled. Without this, the helper would
// supply its own Authorization header in addition to ours, and GitHub
// rejects the request with HTTP 400: Duplicate header: "Authorization".
func gitAuthArgs(token string) []string {
	if token == "" {
		return nil
	}
	return []string{
		"-c", fmt.Sprintf("http.extraHeader=Authorization: Basic %s", basicAuth(token)),
		"-c", "credential.helper=",
	}
}

// basicAuth encodes "x-access-token:{token}" for GitHub App installation tokens.
func basicAuth(token string) string {
	return base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
}

// WorktreeParams holds inputs for creating a worktree from a cached bare clone.
type WorktreeParams struct {
	WorkspaceID string // workspace that owns the repo
	RepoURL     string // remote URL to look up in the cache
	WorkDir     string // parent directory for the worktree (e.g. task workdir)
	AgentName   string // for branch naming
	TaskID      string // for branch naming uniqueness
	Token       string // GitHub installation token (optional; required for private repos)
}

// WorktreeResult describes a successfully created worktree.
type WorktreeResult struct {
	Path       string `json:"path"`        // absolute path to the worktree
	BranchName string `json:"branch_name"` // git branch created for this worktree
}

// CreateWorktree ensures the bare cache for a repo is present and up-to-date,
// then creates a git worktree in the agent's working directory.
//
// If the bare cache does not exist yet, it is cloned on the spot using
// params.Token. If the cache already exists, a fetch is performed first to
// pick up new commits. A fetch failure is treated as a hard error — we never
// fall back to stale cache state, because the resulting worktree (and any PR
// derived from it) would silently be based on outdated code.
func (c *Cache) CreateWorktree(params WorktreeParams) (*WorktreeResult, error) {
	if err := c.ensureBareCache(params.WorkspaceID, params.RepoURL, params.Token); err != nil {
		return nil, err
	}
	barePath := c.barePathFor(params.WorkspaceID, params.RepoURL)

	// Determine the default branch to base the worktree on.
	baseRef := getRemoteDefaultBranch(barePath)

	// Build branch name: agent/{sanitized-name}/{short-task-id}
	branchName := fmt.Sprintf("agent/%s/%s", sanitizeName(params.AgentName), shortID(params.TaskID))

	// Derive directory name from repo URL.
	dirName := repoNameFromURL(params.RepoURL)
	worktreePath := filepath.Join(params.WorkDir, dirName)

	// Create the worktree.
	if err := createWorktree(barePath, worktreePath, branchName, baseRef); err != nil {
		return nil, fmt.Errorf("create worktree: %w", err)
	}

	// Exclude agent context files from git tracking.
	for _, pattern := range []string{".agent_context", "CLAUDE.md", "AGENTS.md", ".claude", ".config/opencode", ".cursor"} {
		_ = excludeFromGit(worktreePath, pattern)
	}

	c.logger.Info("repo checkout: worktree created",
		"url", params.RepoURL,
		"path", worktreePath,
		"branch", branchName,
		"base", baseRef,
	)

	return &WorktreeResult{
		Path:       worktreePath,
		BranchName: branchName,
	}, nil
}

// ensureBareCache makes sure a bare clone for (workspaceID, url) exists and is
// up to date. The mutex serializes lazy clones / fetches across concurrent
// CreateWorktree calls so two tasks racing on the same repo do not corrupt
// each other's git state.
func (c *Cache) ensureBareCache(workspaceID, url, token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	wsDir := filepath.Join(c.root, workspaceID)
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return fmt.Errorf("create workspace cache dir: %w", err)
	}

	barePath := filepath.Join(wsDir, bareDirName(url))

	if !isBareRepo(barePath) {
		c.logger.Info("repo cache: cloning", "url", url, "path", barePath, "with_token", token != "")
		if err := gitCloneBareWithToken(url, barePath, token); err != nil {
			return fmt.Errorf("clone bare cache: %w", err)
		}
		return nil
	}

	c.logger.Info("repo cache: fetching", "url", url, "path", barePath, "with_token", token != "")
	if err := configureRemoteFetchRefspec(barePath); err != nil {
		return err
	}
	if err := gitFetchWithToken(barePath, token); err != nil {
		return fmt.Errorf("fetch latest: %w", err)
	}
	return nil
}

// createWorktree creates a git worktree at the given path with a new branch.
func createWorktree(gitRoot, worktreePath, branchName, baseRef string) error {
	err := runWorktreeAdd(gitRoot, worktreePath, branchName, baseRef)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		// Branch name collision: append timestamp and retry once.
		branchName = fmt.Sprintf("%s-%d", branchName, time.Now().Unix())
		err = runWorktreeAdd(gitRoot, worktreePath, branchName, baseRef)
	}
	return err
}

func runWorktreeAdd(gitRoot, worktreePath, branchName, baseRef string) error {
	cmd := exec.Command("git", "-C", gitRoot, "worktree", "add", "-b", branchName, worktreePath, baseRef)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// getRemoteDefaultBranch returns the best base ref for a bare repo.
// Prefers remote-tracking refs so worktrees are based on the latest fetched
// origin state instead of possibly stale local branches.
func getRemoteDefaultBranch(barePath string) string {
	cmd := exec.Command("git", "-C", barePath, "symbolic-ref", "refs/remotes/origin/HEAD")
	if out, err := cmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		if strings.HasPrefix(ref, "refs/remotes/") {
			return strings.TrimPrefix(ref, "refs/remotes/")
		}
		return ref
	}

	cmd = exec.Command("git", "-C", barePath, "rev-parse", "--verify", "origin/main")
	if err := cmd.Run(); err == nil {
		return "origin/main"
	}

	cmd = exec.Command("git", "-C", barePath, "rev-parse", "--verify", "origin/master")
	if err := cmd.Run(); err == nil {
		return "origin/master"
	}

	// Backward-compatibility fallback for caches that have not been migrated yet.
	cmd = exec.Command("git", "-C", barePath, "symbolic-ref", "HEAD")
	if out, err := cmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		if strings.HasPrefix(ref, "refs/heads/") {
			return strings.TrimPrefix(ref, "refs/heads/")
		}
		return ref
	}

	cmd = exec.Command("git", "-C", barePath, "rev-parse", "--verify", "main")
	if err := cmd.Run(); err == nil {
		return "main"
	}

	cmd = exec.Command("git", "-C", barePath, "rev-parse", "--verify", "master")
	if err := cmd.Run(); err == nil {
		return "master"
	}

	return "HEAD"
}

// excludeFromGit adds a pattern to the worktree's .git/info/exclude file.
func excludeFromGit(worktreePath, pattern string) error {
	cmd := exec.Command("git", "-C", worktreePath, "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("resolve git dir: %w", err)
	}

	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(worktreePath, gitDir)
	}

	excludePath := filepath.Join(gitDir, "info", "exclude")

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("create info dir: %w", err)
	}

	existing, _ := os.ReadFile(excludePath)
	if strings.Contains(string(existing), pattern) {
		return nil
	}

	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open exclude file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n%s\n", pattern); err != nil {
		return fmt.Errorf("write exclude pattern: %w", err)
	}
	return nil
}

// repoNameFromURL extracts a short directory name from a git remote URL.
// e.g. "https://github.com/org/my-repo.git" → "my-repo"
func repoNameFromURL(url string) string {
	url = strings.TrimRight(url, "/")
	url = strings.TrimSuffix(url, ".git")

	if i := strings.LastIndex(url, "/"); i >= 0 {
		url = url[i+1:]
	}
	if i := strings.LastIndex(url, ":"); i >= 0 {
		url = url[i+1:]
		if j := strings.LastIndex(url, "/"); j >= 0 {
			url = url[j+1:]
		}
	}

	name := strings.TrimSpace(url)
	if name == "" {
		return "repo"
	}
	return name
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeName produces a git-branch-safe name from a human-readable string.
func sanitizeName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 30 {
		s = s[:30]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = "agent"
	}
	return s
}

// shortID returns the first 8 characters of a UUID string (dashes stripped).
func shortID(uuid string) string {
	s := strings.ReplaceAll(uuid, "-", "")
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
