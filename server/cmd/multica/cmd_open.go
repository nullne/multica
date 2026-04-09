package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/nullne/multica/server/internal/cli"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Interactive project picker — select a project and open it in your editor",
	Long:  "Fetch workspaces and repos from the server, present an interactive picker,\nclone if needed, and open in Cursor (or another configured editor).",
	RunE:  runOpen,
}

func init() {
	openCmd.Flags().String("editor", "", "Editor to open projects in (default: cursor, fallback: code)")
	openCmd.Flags().Bool("list", false, "Print project list as newline-separated strings and exit (for scripting)")
	openCmd.Flags().String("project", "", "Skip picker and open a specific project (format: \"workspace / repo\")")
}

type project struct {
	WorkspaceName string
	WorkspaceID   string
	RepoName      string
	RepoURL       string
	Description   string
}

func repoNameFromURL(repoURL string) string {
	name := path.Base(repoURL)
	if idx := strings.LastIndex(name, ":"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, ".git")
	if name == "" || name == "." {
		return "repo"
	}
	return name
}

func fetchProjects(cmd *cobra.Command) ([]project, error) {
	serverURL := resolveServerURL(cmd)
	token := resolveToken(cmd)
	if token == "" {
		return nil, fmt.Errorf("not authenticated: run 'multica login' first")
	}

	client := cli.NewAPIClient(serverURL, "", token)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var workspaces []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Repos []struct {
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"repos"`
	}
	if err := client.GetJSON(ctx, "/api/workspaces", &workspaces); err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	var projects []project
	for _, ws := range workspaces {
		for _, repo := range ws.Repos {
			if repo.URL == "" {
				continue
			}
			projects = append(projects, project{
				WorkspaceName: ws.Name,
				WorkspaceID:   ws.ID,
				RepoName:      repoNameFromURL(repo.URL),
				RepoURL:       repo.URL,
				Description:   repo.Description,
			})
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		if projects[i].WorkspaceName != projects[j].WorkspaceName {
			return projects[i].WorkspaceName < projects[j].WorkspaceName
		}
		return projects[i].RepoName < projects[j].RepoName
	})

	return projects, nil
}

func workspacesRoot(profile string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if root := os.Getenv("MULTICA_WORKSPACES_ROOT"); root != "" {
		return root, nil
	}
	suffix := ""
	if profile != "" {
		suffix = "_" + profile
	}
	return filepath.Join(home, "multica_workspaces"+suffix), nil
}

// barePath returns the path for the bare clone cache.
// Layout: ~/multica_workspaces/.repos/{workspaceID}/{repo-name}.git
func barePath(wsRoot string, proj project) string {
	name := proj.RepoName + ".git"
	return filepath.Join(wsRoot, ".repos", proj.WorkspaceID, name)
}

func sanitizeDirName(name string) string {
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	return strings.Trim(name, "-")
}

// ensureBareClone creates or updates a bare clone for caching.
// Uses refs/remotes/origin/* so fetches never conflict with checked-out worktrees.
func ensureBareClone(repoURL, bare string) error {
	if _, err := os.Stat(filepath.Join(bare, "HEAD")); err == nil {
		fmt.Fprintf(os.Stderr, "Updating %s...\n", filepath.Base(bare))
		cmd := exec.Command("git", "-C", bare, "fetch", "origin")
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		cmd.Run() // best-effort
		return nil
	}

	fmt.Fprintf(os.Stderr, "Cloning %s (bare)...\n", repoURL)
	if err := os.MkdirAll(filepath.Dir(bare), 0o755); err != nil {
		return fmt.Errorf("create bare repo dir: %w", err)
	}
	cmd := exec.Command("git", "clone", "--bare", repoURL, bare)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone --bare failed: %w", err)
	}
	// Use remotes refspec so fetch never conflicts with checked-out worktree branches.
	exec.Command("git", "-C", bare, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*").Run()
	// Fetch once with the new refspec to populate refs/remotes/origin/*.
	exec.Command("git", "-C", bare, "fetch", "origin").Run()
	return nil
}

// defaultBranchRef returns the remote ref for the default branch (e.g. "origin/main").
func defaultBranchRef(bare string) string {
	// Try symbolic-ref HEAD → refs/heads/main → check origin/main
	out, err := exec.Command("git", "-C", bare, "symbolic-ref", "HEAD").Output()
	if err == nil {
		branch := strings.TrimPrefix(strings.TrimSpace(string(out)), "refs/heads/")
		remoteRef := "refs/remotes/origin/" + branch
		if exec.Command("git", "-C", bare, "rev-parse", "--verify", remoteRef).Run() == nil {
			return remoteRef
		}
		// Fall back to the local branch ref if remote doesn't exist yet
		if exec.Command("git", "-C", bare, "rev-parse", "--verify", branch).Run() == nil {
			return branch
		}
	}
	for _, name := range []string{"main", "master"} {
		ref := "refs/remotes/origin/" + name
		if exec.Command("git", "-C", bare, "rev-parse", "--verify", ref).Run() == nil {
			return ref
		}
		if exec.Command("git", "-C", bare, "rev-parse", "--verify", name).Run() == nil {
			return name
		}
	}
	return "HEAD"
}

// createWorktree creates a new worktree from the bare clone with a unique branch.
// Layout: ~/multica_workspaces/{workspace-name}/{repo-name}-{timestamp}/
func createWorktree(bare, wsRoot string, proj project) (string, error) {
	ts := time.Now().Format("20060102-150405")
	dirName := fmt.Sprintf("%s-%s", proj.RepoName, ts)
	wtPath := filepath.Join(wsRoot, sanitizeDirName(proj.WorkspaceName), dirName)

	baseRef := defaultBranchRef(bare)
	branchName := fmt.Sprintf("open/%s-%s", proj.RepoName, ts)

	fmt.Fprintf(os.Stderr, "Creating worktree at %s...\n", dirName)
	cmd := exec.Command("git", "-C", bare, "worktree", "add", "-b", branchName, wtPath, baseRef)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git worktree add failed: %w", err)
	}
	return wtPath, nil
}

func openInEditor(editor, editorPath string) error {
	if editor == "" {
		if p, err := exec.LookPath("cursor"); err == nil {
			editor = p
		} else if p, err := exec.LookPath("code"); err == nil {
			editor = p
		}
	}

	if editor != "" {
		cmd := exec.Command(editor, editorPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	}

	if runtime.GOOS == "darwin" {
		for _, app := range []string{"Cursor", "Visual Studio Code"} {
			cmd := exec.Command("open", "-a", app, editorPath)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
	}

	return fmt.Errorf("no editor found: install Cursor or VS Code, or use --editor flag")
}

func runOpen(cmd *cobra.Command, _ []string) error {
	profile := resolveProfile(cmd)
	editor, _ := cmd.Flags().GetString("editor")
	listOnly, _ := cmd.Flags().GetBool("list")
	projectArg, _ := cmd.Flags().GetString("project")

	projects, err := fetchProjects(cmd)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		return fmt.Errorf("no repos found in any workspace.\nAdd repos to your workspace settings in the web app")
	}

	// --list: print project names and exit (used by the native macOS launcher)
	if listOnly {
		for _, p := range projects {
			fmt.Printf("%s / %s\n", p.WorkspaceName, p.RepoName)
		}
		return nil
	}

	var proj *project

	// --project: skip TUI, match by "workspace / repo" label
	if projectArg != "" {
		for i, p := range projects {
			label := fmt.Sprintf("%s / %s", p.WorkspaceName, p.RepoName)
			if label == projectArg {
				proj = &projects[i]
				break
			}
		}
		if proj == nil {
			return fmt.Errorf("project %q not found", projectArg)
		}
	} else {
		// Interactive TUI picker
		fmt.Fprintf(os.Stderr, "Fetching workspaces...\n")
		m := newOpenModel(projects, editor)
		p := tea.NewProgram(m, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		result := finalModel.(openModel)
		if result.selected == nil {
			return nil
		}
		proj = result.selected
	}

	return cloneAndOpen(profile, editor, *proj)
}

func cloneAndOpen(profile, editor string, proj project) error {
	wsRoot, err := workspacesRoot(profile)
	if err != nil {
		return fmt.Errorf("resolve workspaces root: %w", err)
	}

	bare := barePath(wsRoot, proj)
	if err := ensureBareClone(proj.RepoURL, bare); err != nil {
		return err
	}

	wtPath, err := createWorktree(bare, wsRoot, proj)
	if err != nil {
		return err
	}

	editorName := "editor"
	if editor != "" {
		editorName = filepath.Base(editor)
	} else if _, lookErr := exec.LookPath("cursor"); lookErr == nil {
		editorName = "Cursor"
	}
	fmt.Fprintf(os.Stderr, "Opening %s/%s in %s...\n", proj.WorkspaceName, proj.RepoName, editorName)
	return openInEditor(editor, wtPath)
}

// --- Bubbletea TUI Model ---

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(true).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Padding(0, 1)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			PaddingLeft(4)
)

type openModel struct {
	projects []project
	filtered []int
	cursor   int
	filter   textinput.Model
	selected *project
	editor   string
	quitting bool
}

func newOpenModel(projects []project, editor string) openModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()
	ti.CharLimit = 80
	ti.Width = 40

	indices := make([]int, len(projects))
	for i := range projects {
		indices[i] = i
	}

	return openModel{
		projects: projects,
		filtered: indices,
		filter:   ti,
		editor:   editor,
	}
}

func (m openModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m openModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case "enter":
			if len(m.filtered) == 0 {
				break
			}
			idx := m.filtered[m.cursor]
			proj := m.projects[idx]
			m.selected = &proj
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	oldVal := m.filter.Value()
	m.filter, cmd = m.filter.Update(msg)
	if m.filter.Value() != oldVal {
		m.applyFilter()
	}
	return m, cmd
}

func (m *openModel) applyFilter() {
	query := strings.ToLower(m.filter.Value())
	if query == "" {
		m.filtered = make([]int, len(m.projects))
		for i := range m.projects {
			m.filtered[i] = i
		}
	} else {
		m.filtered = m.filtered[:0]
		for i, p := range m.projects {
			haystack := strings.ToLower(p.WorkspaceName + " " + p.RepoName + " " + p.Description)
			if strings.Contains(haystack, query) {
				m.filtered = append(m.filtered, i)
			}
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m openModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Multica — Open Project"))
	b.WriteString("\n")
	b.WriteString(m.filter.View())
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("  No matching projects."))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  esc quit"))
	} else {
		maxVisible := 15
		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.filtered))

		for i := start; i < end; i++ {
			p := m.projects[m.filtered[i]]
			label := fmt.Sprintf("%s / %s", p.WorkspaceName, p.RepoName)
			if i == m.cursor {
				b.WriteString(selectedStyle.Render("▸ " + label))
				if p.Description != "" {
					b.WriteString("\n")
					b.WriteString(descStyle.Render(p.Description))
				}
			} else {
				b.WriteString(normalStyle.Render("  " + label))
			}
			b.WriteString("\n")
		}

		if len(m.filtered) > maxVisible {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  ... %d projects total", len(m.filtered))))
			b.WriteString("\n")
		}

		b.WriteString(helpStyle.Render("  ↑/↓ navigate • enter open • esc quit"))
	}

	return b.String()
}
