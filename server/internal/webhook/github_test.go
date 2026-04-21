package webhook

import (
	"net/http"
	"strings"
	"testing"
)

func TestGitHubAdapter_PullRequestOpened(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "opened",
		"pull_request": {
			"number": 42,
			"title": "Add login feature",
			"body": "This PR adds login.",
			"html_url": "https://github.com/org/repo/pull/42",
			"user": {"login": "alice"},
			"head": {"ref": "feat/login"},
			"base": {"ref": "main"}
		},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "pull_request")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != "github.pull_request.opened" {
		t.Errorf("type = %q, want github.pull_request.opened", ev.Type)
	}
	if ev.Data["source_url"] != "https://github.com/org/repo/pull/42" {
		t.Errorf("source_url = %q", ev.Data["source_url"])
	}
	if ev.Data["source_kind"] != "pr" {
		t.Errorf("source_kind = %q, want pr", ev.Data["source_kind"])
	}
	if ev.Data["repo"] != "org/repo" {
		t.Errorf("repo = %q", ev.Data["repo"])
	}
	if ev.Data["external_id"] != "org/repo#42" {
		t.Errorf("external_id = %q", ev.Data["external_id"])
	}
}

func TestGitHubAdapter_PullRequestMerged(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "closed",
		"pull_request": {
			"number": 42,
			"title": "x",
			"html_url": "https://github.com/org/repo/pull/42",
			"user": {"login": "a"},
			"head": {"ref": "h"},
			"base": {"ref": "main"},
			"merged": true
		},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "pull_request")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	if events[0].Data["action"] != "merged" {
		t.Errorf("expected action=merged, got %q", events[0].Data["action"])
	}
	if events[0].Type != "github.pull_request.merged" {
		t.Errorf("type = %q", events[0].Type)
	}
}

func TestGitHubAdapter_IssuesEvent(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "opened",
		"issue": {
			"number": 7,
			"title": "Bug",
			"body": "boom",
			"html_url": "https://github.com/org/repo/issues/7",
			"user": {"login": "bob"},
			"labels": [{"name": "bug"}, {"name": "p1"}]
		},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "issues")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	ev := events[0]
	if ev.Data["source_url"] != "https://github.com/org/repo/issues/7" {
		t.Errorf("source_url = %q", ev.Data["source_url"])
	}
	if ev.Data["source_kind"] != "issue" {
		t.Errorf("source_kind = %q", ev.Data["source_kind"])
	}
	if ev.Data["labels"] != "bug, p1" {
		t.Errorf("labels = %q", ev.Data["labels"])
	}
}

func TestGitHubAdapter_IssueCommentOnPR(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "created",
		"comment": {
			"body": "LGTM",
			"html_url": "https://github.com/org/repo/issues/42#issuecomment-1",
			"user": {"login": "carol"}
		},
		"issue": {
			"number": 42,
			"title": "Add login",
			"html_url": "https://github.com/org/repo/issues/42",
			"pull_request": {"html_url": "https://github.com/org/repo/pull/42"}
		},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "issue_comment")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	ev := events[0]
	// Comments on PRs must point source_url at the PR (not the issue URL),
	// so the same dedup/lookup as the original PR event lands on one issue.
	if ev.Data["source_url"] != "https://github.com/org/repo/pull/42" {
		t.Errorf("source_url for PR comment = %q, want PR URL", ev.Data["source_url"])
	}
	if ev.Data["source_kind"] != "pr" {
		t.Errorf("source_kind for PR comment = %q", ev.Data["source_kind"])
	}
	if !strings.Contains(ev.Data["comment_body"], "LGTM") {
		t.Errorf("comment_body missing content: %q", ev.Data["comment_body"])
	}
}

func TestGitHubAdapter_IssueCommentOnIssue(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "created",
		"comment": {"body": "+1", "html_url": "x", "user": {"login": "u"}},
		"issue": {"number": 7, "title": "t", "html_url": "https://github.com/org/repo/issues/7"},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "issue_comment")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if events[0].Data["source_url"] != "https://github.com/org/repo/issues/7" {
		t.Errorf("source_url = %q", events[0].Data["source_url"])
	}
	if events[0].Data["source_kind"] != "issue" {
		t.Errorf("source_kind = %q, want issue", events[0].Data["source_kind"])
	}
}

func TestGitHubAdapter_FiltersIrrelevantActions(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{"action": "labeled", "pull_request": {"number": 1}, "repository": {"full_name": "org/repo"}}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "pull_request")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected no events for labeled action, got %d", len(events))
	}
}

func TestGitHubAdapter_PushEvent(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"ref": "refs/heads/main",
		"compare": "https://github.com/org/repo/compare/abc...def",
		"pusher": {"name": "alice"},
		"repository": {"full_name": "org/repo", "html_url": "https://github.com/org/repo"},
		"commits": [{"message": "fix", "url": "u", "author": {"name": "a"}}]
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "push")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	ev := events[0]
	if ev.Data["branch"] != "main" {
		t.Errorf("branch = %q", ev.Data["branch"])
	}
	if ev.Data["source_kind"] != "commit" {
		t.Errorf("source_kind = %q", ev.Data["source_kind"])
	}
	if ev.Type != "github.push" {
		t.Errorf("type = %q", ev.Type)
	}
}

func TestGitHubAdapter_MissingHeader(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{"action": "opened"}`)
	headers := http.Header{}

	if _, err := a.Parse(body, headers); err == nil {
		t.Error("expected error when X-GitHub-Event missing")
	}
}

func TestGitHubAdapter_RegisteredInList(t *testing.T) {
	infos := ListAdapters()
	for _, info := range infos {
		if info.SourceType == "github" {
			if len(info.Keys) == 0 {
				t.Error("github adapter info has no keys")
			}
			return
		}
	}
	t.Error("github adapter not present in ListAdapters()")
}
