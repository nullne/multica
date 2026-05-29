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

func TestGitHubAdapter_PullRequestIssueCandidates(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "opened",
		"pull_request": {
			"number": 42,
			"title": "Add login feature",
			"body": "Fixes #7 and closes other/repo#8. See https://github.com/org/repo/issues/9",
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

	want := strings.Join([]string{
		"https://github.com/org/repo/issues/7",
		"https://github.com/other/repo/issues/8",
		"https://github.com/org/repo/issues/9",
		"https://github.com/org/repo/pull/42",
	}, "\n")
	if got := events[0].Data["source_urls"]; got != want {
		t.Errorf("source_urls = %q, want %q", got, want)
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
	wantDedup := "github:issue_comment:created:https://github.com/org/repo/issues/42#issuecomment-1"
	if ev.DedupKey != wantDedup {
		t.Errorf("dedup_key = %q, want %q", ev.DedupKey, wantDedup)
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

func TestGitHubAdapter_PullRequestReviewComment(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "created",
		"comment": {
			"body": "Please simplify this branch",
			"html_url": "https://github.com/org/repo/pull/42#discussion_r1",
			"path": "server/main.go",
			"user": {"login": "reviewer"}
		},
		"pull_request": {
			"number": 42,
			"title": "Add login",
			"html_url": "https://github.com/org/repo/pull/42"
		},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "pull_request_review_comment")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	ev := events[0]
	if ev.Type != "github.pull_request_review_comment.created" {
		t.Fatalf("type = %q", ev.Type)
	}
	if ev.Data["source_url"] != "https://github.com/org/repo/pull/42" {
		t.Fatalf("source_url = %q", ev.Data["source_url"])
	}
	if ev.Data["source_kind"] != "pr" {
		t.Fatalf("source_kind = %q", ev.Data["source_kind"])
	}
	wantDedup := "github:pull_request_review_comment:created:https://github.com/org/repo/pull/42#discussion_r1"
	if ev.DedupKey != wantDedup {
		t.Fatalf("dedup_key = %q, want %q", ev.DedupKey, wantDedup)
	}
	if !strings.Contains(ev.Data["body"], "Please simplify this branch") {
		t.Fatalf("body = %q", ev.Data["body"])
	}
}

func TestGitHubAdapter_PullRequestReviewSubmitted(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "submitted",
		"review": {
			"body": "Requesting changes before merge",
			"state": "changes_requested",
			"html_url": "https://github.com/org/repo/pull/42#pullrequestreview-1",
			"user": {"login": "reviewer"}
		},
		"pull_request": {
			"number": 42,
			"title": "Add login",
			"html_url": "https://github.com/org/repo/pull/42"
		},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "pull_request_review")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	ev := events[0]
	if ev.Type != "github.pull_request_review.submitted" {
		t.Fatalf("type = %q", ev.Type)
	}
	if ev.Data["source_url"] != "https://github.com/org/repo/pull/42" {
		t.Fatalf("source_url = %q", ev.Data["source_url"])
	}
	if ev.Data["review_state"] != "changes_requested" {
		t.Fatalf("review_state = %q", ev.Data["review_state"])
	}
	wantDedup := "github:pull_request_review:submitted:https://github.com/org/repo/pull/42#pullrequestreview-1"
	if ev.DedupKey != wantDedup {
		t.Fatalf("dedup_key = %q, want %q", ev.DedupKey, wantDedup)
	}
}

func TestGitHubAdapter_CheckRunCompletedOnPR(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "completed",
		"check_run": {
			"name": "CI",
			"status": "completed",
			"conclusion": "failure",
			"html_url": "https://github.com/org/repo/runs/123",
			"head_sha": "abc123",
			"pull_requests": [{"html_url": "https://github.com/org/repo/pull/42"}]
		},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "check_run")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	ev := events[0]
	if ev.Type != "github.check_run.completed" {
		t.Errorf("type = %q", ev.Type)
	}
	if ev.Data["source_url"] != "https://github.com/org/repo/pull/42" {
		t.Errorf("source_url = %q", ev.Data["source_url"])
	}
	if ev.Data["source_kind"] != "pr" {
		t.Errorf("source_kind = %q", ev.Data["source_kind"])
	}
	if ev.Data["check_name"] != "CI" || ev.Data["conclusion"] != "failure" {
		t.Errorf("check data = name %q conclusion %q", ev.Data["check_name"], ev.Data["conclusion"])
	}
}

func TestGitHubAdapter_FiltersIrrelevantActions(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{"action": "unknown_action", "pull_request": {"number": 1}, "repository": {"full_name": "org/repo"}}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "pull_request")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected no events for unknown action, got %d", len(events))
	}
}

func TestGitHubAdapter_OfficialPRAndIssueActions(t *testing.T) {
	a := &githubAdapter{}
	cases := []struct {
		eventType string
		action    string
		body      string
		wantType  string
	}{
		{
			eventType: "pull_request",
			action:    "ready_for_review",
			body:      `"pull_request":{"number":1,"title":"T","html_url":"https://github.com/org/repo/pull/1","user":{"login":"u"},"head":{"ref":"h"},"base":{"ref":"main"}}`,
			wantType:  "github.pull_request.ready_for_review",
		},
		{
			eventType: "issues",
			action:    "edited",
			body:      `"issue":{"number":2,"title":"I","html_url":"https://github.com/org/repo/issues/2","user":{"login":"u"},"labels":[]}`,
			wantType:  "github.issues.edited",
		},
	}
	for _, tc := range cases {
		t.Run(tc.wantType, func(t *testing.T) {
			body := []byte(`{"action":"` + tc.action + `",` + tc.body + `,"repository":{"full_name":"org/repo"}}`)
			headers := http.Header{}
			headers.Set("X-GitHub-Event", tc.eventType)
			events, err := a.Parse(body, headers)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}
			if events[0].Type != tc.wantType {
				t.Fatalf("type = %q, want %q", events[0].Type, tc.wantType)
			}
		})
	}
}

func TestGitHubAdapter_ReleasePublished(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "published",
		"release": {
			"name": "v1.0.0",
			"tag_name": "v1.0.0",
			"body": "Release notes",
			"html_url": "https://github.com/org/repo/releases/tag/v1.0.0",
			"author": {"login": "alice"}
		},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "release")
	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != "github.release.published" {
		t.Fatalf("type = %q", ev.Type)
	}
	if ev.Data["source_url"] != "https://github.com/org/repo/releases/tag/v1.0.0" {
		t.Fatalf("source_url = %q", ev.Data["source_url"])
	}
	if ev.Data["source_kind"] != "release" {
		t.Fatalf("source_kind = %q", ev.Data["source_kind"])
	}
}

func TestGitHubAdapter_PullRequestLabeled(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "labeled",
		"pull_request": {
			"number": 42,
			"title": "Add login feature",
			"body": "x",
			"html_url": "https://github.com/org/repo/pull/42",
			"user": {"login": "alice"},
			"head": {"ref": "feat/login"},
			"base": {"ref": "main"},
			"labels": [{"name": "bug"}, {"name": "agent-task"}]
		},
		"label": {"name": "agent-task"},
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
	if ev.Type != "github.pull_request.labeled" {
		t.Errorf("type = %q, want github.pull_request.labeled", ev.Type)
	}
	if ev.Data["labels"] != "bug, agent-task" {
		t.Errorf("labels = %q, want 'bug, agent-task'", ev.Data["labels"])
	}
	if ev.Data["label_name"] != "agent-task" {
		t.Errorf("label_name = %q, want agent-task", ev.Data["label_name"])
	}
	wantDedup := "github:pull_request:labeled:https://github.com/org/repo/pull/42:agent-task"
	if ev.DedupKey != wantDedup {
		t.Errorf("dedup_key = %q, want %q", ev.DedupKey, wantDedup)
	}
}

func TestGitHubAdapter_IssueLabeled(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "labeled",
		"issue": {
			"number": 7,
			"title": "Bug",
			"body": "boom",
			"html_url": "https://github.com/org/repo/issues/7",
			"user": {"login": "bob"},
			"labels": [{"name": "bug"}, {"name": "p1"}]
		},
		"label": {"name": "p1"},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "issues")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != "github.issues.labeled" {
		t.Errorf("type = %q, want github.issues.labeled", ev.Type)
	}
	if ev.Data["label_name"] != "p1" {
		t.Errorf("label_name = %q, want p1", ev.Data["label_name"])
	}
	wantDedup := "github:issues:labeled:https://github.com/org/repo/issues/7:p1"
	if ev.DedupKey != wantDedup {
		t.Errorf("dedup_key = %q, want %q", ev.DedupKey, wantDedup)
	}
}

func TestGitHubAdapter_PullRequestLabelsExtracted(t *testing.T) {
	a := &githubAdapter{}
	body := []byte(`{
		"action": "opened",
		"pull_request": {
			"number": 10,
			"title": "T",
			"body": "x",
			"html_url": "https://github.com/org/repo/pull/10",
			"user": {"login": "a"},
			"head": {"ref": "h"},
			"base": {"ref": "main"},
			"labels": [{"name": "bug"}, {"name": "needs-review"}]
		},
		"repository": {"full_name": "org/repo"}
	}`)
	headers := http.Header{}
	headers.Set("X-GitHub-Event", "pull_request")

	events, err := a.Parse(body, headers)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if events[0].Data["labels"] != "bug, needs-review" {
		t.Errorf("labels = %q", events[0].Data["labels"])
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

func TestGitHubAdapter_LongBodyPreserved(t *testing.T) {
	// 8 KiB body — well above the old 2000-char truncation limit.
	longBody := strings.Repeat("x", 8*1024)
	a := &githubAdapter{}

	t.Run("pull_request", func(t *testing.T) {
		payload := []byte(`{"action":"opened","pull_request":{"number":1,"title":"T","body":"` + longBody + `","html_url":"https://github.com/o/r/pull/1","user":{"login":"u"},"head":{"ref":"h"},"base":{"ref":"main"}},"repository":{"full_name":"o/r"}}`)
		headers := http.Header{}
		headers.Set("X-GitHub-Event", "pull_request")
		events, err := a.Parse(payload, headers)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		body := events[0].Data["body"]
		if !strings.Contains(body, longBody) {
			t.Errorf("PR body truncated: len=%d, want >=%d", len(body), len(longBody))
		}
		if strings.Contains(body, "truncated") {
			t.Error("PR body contains truncation marker")
		}
	})

	t.Run("issues", func(t *testing.T) {
		payload := []byte(`{"action":"opened","issue":{"number":1,"title":"T","body":"` + longBody + `","html_url":"https://github.com/o/r/issues/1","user":{"login":"u"},"labels":[]},"repository":{"full_name":"o/r"}}`)
		headers := http.Header{}
		headers.Set("X-GitHub-Event", "issues")
		events, err := a.Parse(payload, headers)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		body := events[0].Data["body"]
		if !strings.Contains(body, longBody) {
			t.Errorf("issue body truncated: len=%d, want >=%d", len(body), len(longBody))
		}
		if strings.Contains(body, "truncated") {
			t.Error("issue body contains truncation marker")
		}
	})

	t.Run("issue_comment", func(t *testing.T) {
		payload := []byte(`{"action":"created","comment":{"body":"` + longBody + `","html_url":"https://github.com/o/r/issues/1#c1","user":{"login":"u"}},"issue":{"number":1,"title":"T","html_url":"https://github.com/o/r/issues/1"},"repository":{"full_name":"o/r"}}`)
		headers := http.Header{}
		headers.Set("X-GitHub-Event", "issue_comment")
		events, err := a.Parse(payload, headers)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		commentBody := events[0].Data["comment_body"]
		if commentBody != longBody {
			t.Errorf("comment_body truncated: len=%d, want %d", len(commentBody), len(longBody))
		}
		if strings.Contains(events[0].Data["body"], "truncated") {
			t.Error("comment body field contains truncation marker")
		}
	})
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
