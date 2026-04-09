package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/nullne/multica/server/internal/cli"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Interactive project picker — select a project and open it in your editor",
	Long:  "Scan local workspace repositories and present an interactive picker.\nSelect a project to open it in Cursor (or another configured editor).",
	RunE:  runOpen,
}

func init() {
	openCmd.Flags().String("editor", "", "Editor to open projects in (default: cursor, fallback: code)")
}

// project represents a locally available repository within a workspace.
type project struct {
	WorkspaceName string
	WorkspaceID   string
	RepoName      string
	Path          string // bare repo path under .repos/
}

func scanProjects(profile string) ([]project, error) {
	cfg, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	wsNames := make(map[string]string)
	for _, w := range cfg.WatchedWorkspaces {
		wsNames[w.ID] = w.Name
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	suffix := ""
	if profile != "" {
		suffix = "_" + profile
	}
	wsRoot := os.Getenv("MULTICA_WORKSPACES_ROOT")
	if wsRoot == "" {
		wsRoot = filepath.Join(home, "multica_workspaces"+suffix)
	}

	reposRoot := filepath.Join(wsRoot, ".repos")
	entries, err := os.ReadDir(reposRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read repos dir: %w", err)
	}

	var projects []project
	for _, wsDir := range entries {
		if !wsDir.IsDir() {
			continue
		}
		wsID := wsDir.Name()
		wsName := wsNames[wsID]
		if wsName == "" {
			wsName = wsID[:min(8, len(wsID))]
		}

		repos, err := os.ReadDir(filepath.Join(reposRoot, wsID))
		if err != nil {
			continue
		}
		for _, repo := range repos {
			if !repo.IsDir() {
				continue
			}
			name := repo.Name()
			displayName := strings.TrimSuffix(name, ".git")
			projects = append(projects, project{
				WorkspaceName: wsName,
				WorkspaceID:   wsID,
				RepoName:      displayName,
				Path:          filepath.Join(reposRoot, wsID, name),
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

// findWorktrees locates existing git worktrees for a bare repo.
func findWorktrees(barePath string) []string {
	out, err := exec.Command("git", "-C", barePath, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil
	}

	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			wt := strings.TrimPrefix(line, "worktree ")
			if wt != barePath {
				paths = append(paths, wt)
			}
		}
	}
	return paths
}

func openInEditor(editor, path string) error {
	if editor == "" {
		if p, err := exec.LookPath("cursor"); err == nil {
			editor = p
		} else if p, err := exec.LookPath("code"); err == nil {
			editor = p
		}
	}

	if editor != "" {
		cmd := exec.Command(editor, path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	}

	if runtime.GOOS == "darwin" {
		for _, app := range []string{"Cursor", "Visual Studio Code"} {
			cmd := exec.Command("open", "-a", app, path)
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

	projects, err := scanProjects(profile)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		return fmt.Errorf("no local projects found.\nRun 'multica daemon start' to sync repos, or 'multica workspace watch <id>' to add workspaces")
	}

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

	proj := result.selected
	targetPath := result.openPath

	if targetPath == "" {
		return fmt.Errorf("no working directory available for %s/%s", proj.WorkspaceName, proj.RepoName)
	}

	editorName := "editor"
	if editor != "" {
		editorName = filepath.Base(editor)
	} else if p, _ := exec.LookPath("cursor"); p != "" {
		editorName = "Cursor"
	}
	fmt.Fprintf(os.Stderr, "Opening %s/%s in %s...\n", proj.WorkspaceName, proj.RepoName, editorName)
	return openInEditor(editor, targetPath)
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

	worktreeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	activeWorktreeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))
)

type openModel struct {
	projects  []project
	filtered  []int // indices into projects
	cursor    int
	filter    textinput.Model
	selected  *project
	openPath  string
	editor    string
	quitting  bool
	wtExpand  int // index in filtered of expanded worktree list, -1 = none
	wtList    []string
	wtCursor  int
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
		wtExpand: -1,
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
			if m.wtExpand >= 0 {
				if m.wtCursor > 0 {
					m.wtCursor--
				}
			} else if m.cursor > 0 {
				m.cursor--
			}

		case "down", "ctrl+n":
			if m.wtExpand >= 0 {
				if m.wtCursor < len(m.wtList)-1 {
					m.wtCursor++
				}
			} else if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case "enter":
			if len(m.filtered) == 0 {
				break
			}

			if m.wtExpand >= 0 {
				proj := m.projects[m.filtered[m.wtExpand]]
				m.selected = &proj
				if m.wtCursor < len(m.wtList) {
					m.openPath = m.wtList[m.wtCursor]
				}
				return m, tea.Quit
			}

			idx := m.filtered[m.cursor]
			proj := m.projects[idx]
			worktrees := findWorktrees(proj.Path)

			if len(worktrees) == 0 {
				m.selected = &proj
				m.openPath = proj.Path
				return m, tea.Quit
			}

			if len(worktrees) == 1 {
				m.selected = &proj
				m.openPath = worktrees[0]
				return m, tea.Quit
			}

			m.wtExpand = m.cursor
			m.wtList = worktrees
			m.wtCursor = 0

		case "backspace":
			if m.wtExpand >= 0 {
				m.wtExpand = -1
				m.wtList = nil
				m.wtCursor = 0
				return m, nil
			}
		}
	}

	if m.wtExpand < 0 {
		var cmd tea.Cmd
		oldVal := m.filter.Value()
		m.filter, cmd = m.filter.Update(msg)
		if m.filter.Value() != oldVal {
			m.applyFilter()
		}
		return m, cmd
	}

	return m, nil
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
			haystack := strings.ToLower(p.WorkspaceName + " " + p.RepoName)
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

	if m.wtExpand >= 0 {
		proj := m.projects[m.filtered[m.wtExpand]]
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %s / %s — select worktree:", proj.WorkspaceName, proj.RepoName)))
		b.WriteString("\n")
		for i, wt := range m.wtList {
			display := filepath.Base(filepath.Dir(wt)) + "/" + filepath.Base(wt)
			if i == m.wtCursor {
				b.WriteString(activeWorktreeStyle.Render("▸ " + display))
			} else {
				b.WriteString(worktreeStyle.Render("  " + display))
			}
			b.WriteString("\n")
		}
		b.WriteString(helpStyle.Render("  ↑/↓ navigate • enter open • backspace back • esc quit"))
	} else if len(m.filtered) == 0 {
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
