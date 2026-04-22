package handler

import (
	"context"
	"encoding/json"
	"testing"

	db "github.com/nullne/multica/server/pkg/db/generated"
)

func TestIsRelevantAction(t *testing.T) {
	tests := []struct {
		eventType string
		action    string
		want      bool
	}{
		{"push", "", true},
		{"push", "anything", true},
		{"pull_request", "opened", true},
		{"pull_request", "synchronize", true},
		{"pull_request", "reopened", true},
		{"pull_request", "closed", true},
		{"pull_request", "labeled", false},
		{"pull_request", "assigned", false},
		{"pull_request", "review_requested", false},
		{"issues", "opened", true},
		{"issues", "reopened", true},
		{"issues", "closed", false},
		{"issues", "labeled", false},
		{"issues", "deleted", false},
		{"issue_comment", "created", true},
		{"issue_comment", "edited", false},
		{"issue_comment", "deleted", false},
		{"unknown_event", "opened", false},
	}

	for _, tt := range tests {
		t.Run(tt.eventType+"/"+tt.action, func(t *testing.T) {
			got := isRelevantAction(tt.eventType, tt.action)
			if got != tt.want {
				t.Errorf("isRelevantAction(%q, %q) = %v, want %v", tt.eventType, tt.action, got, tt.want)
			}
		})
	}
}

func TestParsePushEvent(t *testing.T) {
	payload := `{
		"ref": "refs/heads/main",
		"compare": "https://github.com/org/repo/compare/abc...def",
		"pusher": {"name": "alice"},
		"repository": {"full_name": "org/repo", "html_url": "https://github.com/org/repo"},
		"commits": [
			{"message": "fix: typo in readme\nsome details", "url": "https://github.com/org/repo/commit/abc", "author": {"name": "alice"}},
			{"message": "feat: add new endpoint", "url": "https://github.com/org/repo/commit/def", "author": {"name": "bob"}}
		]
	}`

	data := parsePushEvent([]byte(payload))

	assertField(t, data, "branch", "main")
	assertField(t, data, "repo", "org/repo")
	assertField(t, data, "pusher", "alice")
	assertField(t, data, "ref", "refs/heads/main")

	if data["commits"] == "" {
		t.Error("expected non-empty commits field")
	}
	if data["body"] == "" {
		t.Error("expected non-empty body")
	}
}

func TestParsePullRequestEvent(t *testing.T) {
	payload := `{
		"action": "opened",
		"pull_request": {
			"number": 42,
			"title": "Add login feature",
			"body": "This PR adds login.",
			"html_url": "https://github.com/org/repo/pull/42",
			"user": {"login": "alice"},
			"head": {"ref": "feat/login"},
			"base": {"ref": "main"},
			"merged": false
		},
		"repository": {"full_name": "org/repo"}
	}`

	data := parsePullRequestEvent([]byte(payload))

	assertField(t, data, "action", "opened")
	assertField(t, data, "number", "42")
	assertField(t, data, "title", "Add login feature")
	assertField(t, data, "user", "alice")
	assertField(t, data, "repo", "org/repo")
	assertField(t, data, "head_branch", "feat/login")
	assertField(t, data, "base_branch", "main")
}

func TestParsePullRequestEvent_Merged(t *testing.T) {
	payload := `{
		"action": "closed",
		"pull_request": {
			"number": 42,
			"title": "Add login feature",
			"body": "",
			"html_url": "https://github.com/org/repo/pull/42",
			"user": {"login": "alice"},
			"head": {"ref": "feat/login"},
			"base": {"ref": "main"},
			"merged": true
		},
		"repository": {"full_name": "org/repo"}
	}`

	data := parsePullRequestEvent([]byte(payload))
	assertField(t, data, "action", "merged")
}

func TestParseIssuesEvent(t *testing.T) {
	payload := `{
		"action": "opened",
		"issue": {
			"number": 10,
			"title": "Bug: crash on startup",
			"body": "The app crashes when...",
			"html_url": "https://github.com/org/repo/issues/10",
			"user": {"login": "bob"},
			"labels": [{"name": "bug"}, {"name": "priority:high"}]
		},
		"repository": {"full_name": "org/repo"}
	}`

	data := parseIssuesEvent([]byte(payload))

	assertField(t, data, "action", "opened")
	assertField(t, data, "number", "10")
	assertField(t, data, "title", "Bug: crash on startup")
	assertField(t, data, "user", "bob")
	assertField(t, data, "labels", "bug, priority:high")
}

func TestParseIssueCommentEvent(t *testing.T) {
	payload := `{
		"action": "created",
		"comment": {
			"body": "Looks good to me!",
			"html_url": "https://github.com/org/repo/issues/10#issuecomment-123",
			"user": {"login": "alice"}
		},
		"issue": {
			"number": 10,
			"title": "Bug: crash on startup",
			"html_url": "https://github.com/org/repo/issues/10"
		},
		"repository": {"full_name": "org/repo"}
	}`

	data := parseIssueCommentEvent([]byte(payload))

	assertField(t, data, "action", "created")
	assertField(t, data, "number", "10")
	assertField(t, data, "issue_title", "Bug: crash on startup")
	assertField(t, data, "comment_body", "Looks good to me!")
	assertField(t, data, "user", "alice")
	assertField(t, data, "repo", "org/repo")
}

func TestDefaultTitleTemplates(t *testing.T) {
	for _, et := range []string{"push", "pull_request", "issues", "issue_comment"} {
		tmpl := defaultTitleTemplate(et)
		if tmpl == "" {
			t.Errorf("defaultTitleTemplate(%q) returned empty string", et)
		}
	}
	if tmpl := defaultTitleTemplate("unknown"); tmpl != "" {
		t.Errorf("defaultTitleTemplate(unknown) = %q, want empty", tmpl)
	}
}

func TestParsePushEvent_ManyCommits(t *testing.T) {
	type commit struct {
		Message string `json:"message"`
		URL     string `json:"url"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	}
	commits := make([]commit, 15)
	for i := range commits {
		commits[i] = commit{Message: "commit " + string(rune('A'+i)), URL: "https://example.com/" + string(rune('A'+i))}
		commits[i].Author.Name = "dev"
	}
	payload := map[string]any{
		"ref":        "refs/heads/main",
		"compare":    "https://example.com/compare",
		"pusher":     map[string]string{"name": "dev"},
		"repository": map[string]string{"full_name": "org/repo", "html_url": "https://example.com"},
		"commits":    commits,
	}
	b, _ := json.Marshal(payload)

	data := parsePushEvent(b)
	if data["commits"] == "" {
		t.Error("expected non-empty commits")
	}
}

func TestParseGitHubEventData_UnknownType(t *testing.T) {
	data := parseGitHubEventData("unknown", []byte(`{}`))
	if len(data) != 0 {
		t.Errorf("expected empty data for unknown event type, got %v", data)
	}
}

func TestValidGitHubEventTypes(t *testing.T) {
	valid := []string{"push", "pull_request", "issues", "issue_comment"}
	invalid := []string{"", "star", "fork", "deployment"}

	for _, v := range valid {
		if !validGitHubEventTypes[v] {
			t.Errorf("expected %q to be a valid event type", v)
		}
	}
	for _, v := range invalid {
		if validGitHubEventTypes[v] {
			t.Errorf("expected %q to NOT be a valid event type", v)
		}
	}
}

func assertField(t *testing.T, data map[string]string, key, want string) {
	t.Helper()
	got, ok := data[key]
	if !ok {
		t.Errorf("missing key %q in data", key)
		return
	}
	if got != want {
		t.Errorf("data[%q] = %q, want %q", key, got, want)
	}
}

func TestIsValidRepoFullName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"owner/repo", true},
		{"my-org/my-repo", true},
		{"foo.bar/baz_qux-1", true},
		{"a/b", true},
		{"", false},
		{"owner", false},
		{"owner/", false},
		{"/repo", false},
		{"owner/repo/extra", false},
		{"owner/re po", false},
		{"owner repo", false},
		{"owner/re$po", false},
	}
	for _, tc := range cases {
		if got := isValidRepoFullName(tc.in); got != tc.want {
			t.Errorf("isValidRepoFullName(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestGitHubEventRulePerRepoLookup verifies that GetGitHubEventRule prefers
// an exact repo match and falls back to the workspace-default rule
// (repo_full_name = '') when no per-repo rule exists. Requires the test
// database fixture from TestMain.
func TestGitHubEventRulePerRepoLookup(t *testing.T) {
	if testHandler == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	q := testHandler.Queries

	var agentIDStr string
	err := testPool.QueryRow(ctx,
		`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "Handler Test Agent",
	).Scan(&agentIDStr)
	if err != nil {
		t.Fatalf("failed to find test agent: %v", err)
	}
	agentID := parseUUID(agentIDStr)
	wsID := parseUUID(testWorkspaceID)

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM github_event_rule WHERE workspace_id = $1`, testWorkspaceID)
	})

	defaultRule, err := q.UpsertGitHubEventRule(ctx, db.UpsertGitHubEventRuleParams{
		WorkspaceID:   wsID,
		EventType:     "pull_request",
		RepoFullName:  "",
		AgentID:       agentID,
		Enabled:       true,
		TitleTemplate: "default-title",
	})
	if err != nil {
		t.Fatalf("upsert default rule: %v", err)
	}

	overrideRule, err := q.UpsertGitHubEventRule(ctx, db.UpsertGitHubEventRuleParams{
		WorkspaceID:   wsID,
		EventType:     "pull_request",
		RepoFullName:  "acme/widgets",
		AgentID:       agentID,
		Enabled:       true,
		TitleTemplate: "override-title",
	})
	if err != nil {
		t.Fatalf("upsert override rule: %v", err)
	}

	matched, err := q.GetGitHubEventRule(ctx, db.GetGitHubEventRuleParams{
		WorkspaceID:  wsID,
		EventType:    "pull_request",
		RepoFullName: "acme/widgets",
	})
	if err != nil {
		t.Fatalf("lookup matching repo: %v", err)
	}
	if matched.ID != overrideRule.ID {
		t.Fatalf("expected override rule for matching repo")
	}
	if matched.TitleTemplate != "override-title" {
		t.Fatalf("expected override title, got %q", matched.TitleTemplate)
	}

	fallback, err := q.GetGitHubEventRule(ctx, db.GetGitHubEventRuleParams{
		WorkspaceID:  wsID,
		EventType:    "pull_request",
		RepoFullName: "other/repo",
	})
	if err != nil {
		t.Fatalf("lookup unmatched repo: %v", err)
	}
	if fallback.ID != defaultRule.ID {
		t.Fatalf("expected default rule for unmatched repo")
	}
	if fallback.TitleTemplate != "default-title" {
		t.Fatalf("expected default title, got %q", fallback.TitleTemplate)
	}

	if err := q.DeleteGitHubEventRule(ctx, defaultRule.ID); err != nil {
		t.Fatalf("delete default rule: %v", err)
	}

	matched2, err := q.GetGitHubEventRule(ctx, db.GetGitHubEventRuleParams{
		WorkspaceID:  wsID,
		EventType:    "pull_request",
		RepoFullName: "acme/widgets",
	})
	if err != nil {
		t.Fatalf("lookup matching repo after default removed: %v", err)
	}
	if matched2.ID != overrideRule.ID {
		t.Fatalf("expected override rule still matches its repo")
	}

	if _, err := q.GetGitHubEventRule(ctx, db.GetGitHubEventRuleParams{
		WorkspaceID:  wsID,
		EventType:    "pull_request",
		RepoFullName: "other/repo",
	}); err == nil {
		t.Fatalf("expected no-rows error for unmatched repo with no default")
	}

	rules, err := q.ListGitHubEventRules(ctx, wsID)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != overrideRule.ID {
		t.Fatalf("unexpected list result after delete: %+v", rules)
	}

	if _, err := q.UpsertGitHubEventRule(ctx, db.UpsertGitHubEventRuleParams{
		WorkspaceID:   wsID,
		EventType:     "pull_request",
		RepoFullName:  "",
		AgentID:       agentID,
		Enabled:       true,
		TitleTemplate: "default-title",
	}); err != nil {
		t.Fatalf("re-insert default: %v", err)
	}
	rules, err = q.ListGitHubEventRules(ctx, wsID)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].RepoFullName != "" || rules[1].RepoFullName != "acme/widgets" {
		t.Fatalf("unexpected list ordering: %q then %q", rules[0].RepoFullName, rules[1].RepoFullName)
	}
}
