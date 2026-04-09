// Package repocache manages bare git clone caches for workspace repositories.
// The daemon uses these caches as the source for creating per-task worktrees.
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

// RepoInfo describes a repository to cache.
type RepoInfo struct {
	URL         string
	Description string
}

// CachedRepo describes a cached bare clone ready for worktree creation.
type CachedRepo struct {
	URL         string // remote URL
	Description string // human-readable description
	LocalPath   string // absolute path to the bare clone
}

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

// Sync ensures all repos for a workspace are cloned (or fetched if already cached).
// Repos no longer in the list are left in place (cheap to keep, avoids re-cloning
// if a repo is temporarily removed and re-added).
func (c *Cache) Sync(workspaceID string, repos []RepoInfo) error {
	return c.SyncWithToken(workspaceID, repos, "")
}

// SyncWithToken is like Sync but uses the given GitHub token for authentication.
func (c *Cache) SyncWithToken(workspaceID string, repos []RepoInfo, token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	wsDir := filepath.Join(c.root, workspaceID)
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return fmt.Errorf("create workspace cache dir: %w", err)
	}

	var firstErr error
	for _, repo := range repos {
		if repo.URL == "" {
			continue
		}
		barePath := filepath.Join(wsDir, bareDirName(repo.URL))

		if isBareRepo(barePath) {
			c.logger.Info("repo cache: fetching", "url", repo.URL, "path", barePath)
			if err := gitFetchWithToken(barePath, token); err != nil {
				c.logger.Warn("repo cache: fetch failed", "url", repo.URL, "error", err)
				if firstErr == nil {
					firstErr = err
				}
			}
		} else {
			c.logger.Info("repo cache: cloning", "url", repo.URL, "path", barePath)
			if err := gitCloneBareWithToken(repo.URL, barePath, token); err != nil {
				c.logger.Error("repo cache: clone failed", "url", repo.URL, "error", err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	return firstErr
}

// Lookup returns the local bare clone path for a repo URL within a workspace.
// Returns "" if not cached.
func (c *Cache) Lookup(workspaceID, url string) string {
	barePath := filepath.Join(c.root, workspaceID, bareDirName(url))
	if isBareRepo(barePath) {
		return barePath
	}
	return ""
}

// Fetch runs `git fetch origin` on a cached bare clone to get latest refs.
func (c *Cache) Fetch(barePath string) error {
	return gitFetch(barePath)
}

// FetchWithToken runs `git fetch origin` with a GitHub token for auth.
func (c *Cache) FetchWithToken(barePath, token string) error {
	return gitFetchWithToken(barePath, token)
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

func gitCloneBare(url, dest string) error {
	return gitCloneBareWithToken(url, dest, "")
}

func gitCloneBareWithToken(url, dest, token string) error {
	args := []string{"clone", "--bare"}
	if token != "" {
		args = append(args,
			"-c", fmt.Sprintf("http.extraHeader=Authorization: Basic %s", basicAuth(token)),
		)
	}
	args = append(args, url, dest)
	cmd := exec.Command("git", args...)
	if token != "" {
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dest)
		return fmt.Errorf("git clone --bare: %s: %w", strings.TrimSpace(string(out)), err)
	}
	cmd = exec.Command("git", "-C", dest, "config", "remote.origin.fetch", "+refs/heads/*:refs/heads/*")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("configure fetch refspec: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func gitFetch(barePath string) error {
	return gitFetchWithToken(barePath, "")
}

func gitFetchWithToken(barePath, token string) error {
	args := []string{"-C", barePath}
	if token != "" {
		args = append(args,
			"-c", fmt.Sprintf("http.extraHeader=Authorization: Basic %s", basicAuth(token)),
		)
	}
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
}

// WorktreeResult describes a successfully created worktree.
type WorktreeResult struct {
	Path       string `json:"path"`        // absolute path to the worktree
	BranchName string `json:"branch_name"` // git branch created for this worktree
}

// CreateWorktree looks up the bare cache for a repo, fetches latest, and creates
// a git worktree in the agent's working directory.
func (c *Cache) CreateWorktree(params WorktreeParams) (*WorktreeResult, error) {
	barePath := c.Lookup(params.WorkspaceID, params.RepoURL)
	if barePath == "" {
		return nil, fmt.Errorf("repo not found in cache: %s (workspace: %s)", params.RepoURL, params.WorkspaceID)
	}

	// Fetch latest from origin.
	if err := gitFetch(barePath); err != nil {
		c.logger.Warn("repo checkout: fetch failed (continuing with cached state)", "url", params.RepoURL, "error", err)
	}

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

// getRemoteDefaultBranch returns the default branch ref for a bare repo.
// Tries HEAD, then falls back to "main", then "master".
func getRemoteDefaultBranch(barePath string) string {
	// In a bare repo, HEAD points to the default branch.
	cmd := exec.Command("git", "-C", barePath, "symbolic-ref", "HEAD")
	if out, err := cmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		// ref looks like "refs/heads/main" — return just the branch name.
		if strings.HasPrefix(ref, "refs/heads/") {
			return strings.TrimPrefix(ref, "refs/heads/")
		}
		return ref
	}

	// Fallback: check if main branch exists.
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
