package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// githubAdapter handles incoming GitHub App webhook payloads. The HMAC
// signature is verified by the handler layer before Parse is called; this
// adapter focuses on translating each supported event type into the unified
// Event format consumed by webhook actions.
//
// Event.Type uses the convention "github.<event_type>[.<action>]" so action
// filters can match by full event type. Event.Data always includes:
//   - source_url:   PR/issue HTML URL used by issue_link reverse lookup
//   - source_kind:  "pr" | "issue"  (matches issue_link.kind)
//   - repo:         "owner/repo"
//   - title / body: pre-rendered defaults so the simplest action config works
type githubAdapter struct{}

// Header used by the handler to forward the GitHub event type into Parse.
const headerGitHubEvent = "X-GitHub-Event"

func (a *githubAdapter) Parse(payload json.RawMessage, headers http.Header) ([]Event, error) {
	eventType := headers.Get(headerGitHubEvent)
	if eventType == "" {
		return nil, fmt.Errorf("missing %s header", headerGitHubEvent)
	}

	// envelope holds fields common to every event: action and repo.
	var envelope struct {
		Action     string `json:"action"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	_ = json.Unmarshal(payload, &envelope)

	if !isRelevantGitHubAction(eventType, envelope.Action) {
		return nil, nil
	}

	var data map[string]string
	switch eventType {
	case "push":
		data = parseGitHubPush(payload)
	case "pull_request":
		data = parseGitHubPullRequest(payload)
	case "issues":
		data = parseGitHubIssues(payload)
	case "issue_comment":
		data = parseGitHubIssueComment(payload)
	case "check_run":
		data = parseGitHubCheckRun(payload)
	case "check_suite":
		data = parseGitHubCheckSuite(payload)
	case "status":
		data = parseGitHubStatus(payload)
	case "workflow_run":
		data = parseGitHubWorkflowRun(payload)
	default:
		return nil, nil
	}

	if data == nil {
		return nil, nil
	}

	if data["repo"] == "" && envelope.Repository.FullName != "" {
		data["repo"] = envelope.Repository.FullName
	}
	if data["action"] == "" && envelope.Action != "" {
		data["action"] = envelope.Action
	}

	dedupKey := data["dedup_key"]
	if dedupKey == "" {
		dedupKey = fmt.Sprintf("github:%s:%s:%s", eventType, envelope.Action, data["source_url"])
	}

	typeWithAction := "github." + eventType
	if act := data["action"]; act != "" {
		typeWithAction = typeWithAction + "." + act
	}

	return []Event{{
		Type:       typeWithAction,
		DedupKey:   dedupKey,
		Data:       data,
		RawPayload: payload,
	}}, nil
}

// isRelevantGitHubAction filters event actions so noisy webhooks like
// assignment churn never reach the action pipeline. We accept "labeled"
// for PRs and issues so label-filter actions can fire when a configured
// label is added to an existing PR/issue after creation.
func isRelevantGitHubAction(eventType, action string) bool {
	switch eventType {
	case "push":
		return true
	case "pull_request":
		switch action {
		case "opened", "synchronize", "reopened", "closed", "labeled":
			return true
		}
		return false
	case "issues":
		switch action {
		case "opened", "reopened", "closed", "labeled":
			return true
		}
		return false
	case "issue_comment":
		return action == "created"
	case "check_run", "check_suite", "workflow_run":
		return action == "completed"
	case "status":
		return true
	default:
		return false
	}
}

func parseGitHubPush(body []byte) map[string]string {
	var ev struct {
		Ref     string `json:"ref"`
		Compare string `json:"compare"`
		Pusher  struct {
			Name string `json:"name"`
		} `json:"pusher"`
		Repository struct {
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
		} `json:"repository"`
		Commits []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			URL     string `json:"url"`
			Author  struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commits"`
	}
	_ = json.Unmarshal(body, &ev)

	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	repo := ev.Repository.FullName

	var commitLines strings.Builder
	for i, c := range ev.Commits {
		if i >= 10 {
			commitLines.WriteString(fmt.Sprintf("\n... and %d more commits", len(ev.Commits)-10))
			break
		}
		firstLine := strings.SplitN(c.Message, "\n", 2)[0]
		commitLines.WriteString(fmt.Sprintf("- [`%s`](%s) %s\n", firstLine, c.URL, c.Author.Name))
	}

	return map[string]string{
		"ref":         ev.Ref,
		"branch":      branch,
		"repo":        repo,
		"pusher":      ev.Pusher.Name,
		"compare_url": ev.Compare,
		"repo_url":    ev.Repository.HTMLURL,
		"commits":     commitLines.String(),
		"source_url":  ev.Compare,
		"source_kind": "commit",
		"title":       fmt.Sprintf("Push to %s/%s by %s", repo, branch, ev.Pusher.Name),
		"body":        fmt.Sprintf("**Branch:** `%s`\n**Pusher:** %s\n**Compare:** [view diff](%s)\n\n**Commits:**\n%s", branch, ev.Pusher.Name, ev.Compare, commitLines.String()),
	}
}

func parseGitHubPullRequest(body []byte) map[string]string {
	var ev struct {
		Action      string `json:"action"`
		PullRequest struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
			Head struct {
				Ref string `json:"ref"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
			Merged bool `json:"merged"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		} `json:"pull_request"`
		Label struct {
			Name string `json:"name"`
		} `json:"label"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	_ = json.Unmarshal(body, &ev)

	pr := ev.PullRequest
	action := ev.Action
	if action == "closed" && pr.Merged {
		action = "merged"
	}
	sourceURLs := githubPRSourceURLs(ev.Repository.FullName, pr.Body, pr.HTMLURL)

	var labelNames []string
	for _, l := range pr.Labels {
		labelNames = append(labelNames, l.Name)
	}

	data := map[string]string{
		"action":      action,
		"number":      fmt.Sprintf("%d", pr.Number),
		"title":       pr.Title,
		"user":        pr.User.Login,
		"repo":        ev.Repository.FullName,
		"html_url":    pr.HTMLURL,
		"head_branch": pr.Head.Ref,
		"base_branch": pr.Base.Ref,
		"labels":      strings.Join(labelNames, ", "),
		"source_url":  pr.HTMLURL,
		"source_urls": strings.Join(sourceURLs, "\n"),
		"source_kind": "pr",
		"external_id": fmt.Sprintf("%s#%d", ev.Repository.FullName, pr.Number),
		"body":        fmt.Sprintf("**PR [#%d](%s): %s**\n**Author:** %s\n**Branch:** `%s` → `%s`\n**Action:** %s\n\n%s", pr.Number, pr.HTMLURL, pr.Title, pr.User.Login, pr.Head.Ref, pr.Base.Ref, action, pr.Body),
	}
	if action == "labeled" && ev.Label.Name != "" {
		data["label_name"] = ev.Label.Name
		data["dedup_key"] = fmt.Sprintf("github:pull_request:labeled:%s:%s", pr.HTMLURL, ev.Label.Name)
	}
	return data
}

var (
	githubIssueURLRe   = regexp.MustCompile(`https://github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)/issues/([0-9]+)`)
	githubClosingRefRe = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+((?:[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)?#[0-9]+)`)
)

func githubPRSourceURLs(repo, body, prURL string) []string {
	var urls []string
	seen := make(map[string]bool)
	type candidate struct {
		index int
		url   string
	}
	var candidates []candidate
	add := func(url string) {
		if url == "" || seen[url] {
			return
		}
		seen[url] = true
		urls = append(urls, url)
	}

	for _, match := range githubIssueURLRe.FindAllStringSubmatchIndex(body, -1) {
		candidates = append(candidates, candidate{
			index: match[0],
			url:   fmt.Sprintf("https://github.com/%s/issues/%s", body[match[2]:match[3]], body[match[4]:match[5]]),
		})
	}
	for _, match := range githubClosingRefRe.FindAllStringSubmatchIndex(body, -1) {
		ref := body[match[2]:match[3]]
		hash := strings.LastIndex(ref, "#")
		if hash < 0 {
			continue
		}
		refRepo := ref[:hash]
		if refRepo == "" {
			refRepo = repo
		}
		candidates = append(candidates, candidate{
			index: match[0],
			url:   fmt.Sprintf("https://github.com/%s/issues/%s", refRepo, ref[hash+1:]),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].index < candidates[j].index
	})
	for _, candidate := range candidates {
		add(candidate.url)
	}
	add(prURL)
	return urls
}

func parseGitHubIssues(body []byte) map[string]string {
	var ev struct {
		Action string `json:"action"`
		Issue  struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		} `json:"issue"`
		Label struct {
			Name string `json:"name"`
		} `json:"label"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	_ = json.Unmarshal(body, &ev)

	issue := ev.Issue
	var labelNames []string
	for _, l := range issue.Labels {
		labelNames = append(labelNames, l.Name)
	}

	data := map[string]string{
		"action":      ev.Action,
		"number":      fmt.Sprintf("%d", issue.Number),
		"title":       issue.Title,
		"user":        issue.User.Login,
		"repo":        ev.Repository.FullName,
		"html_url":    issue.HTMLURL,
		"labels":      strings.Join(labelNames, ", "),
		"source_url":  issue.HTMLURL,
		"source_kind": "issue",
		"external_id": fmt.Sprintf("%s#%d", ev.Repository.FullName, issue.Number),
		"body":        fmt.Sprintf("**GitHub Issue [#%d](%s): %s**\n**Author:** %s\n**Action:** %s\n\n%s", issue.Number, issue.HTMLURL, issue.Title, issue.User.Login, ev.Action, issue.Body),
	}
	if ev.Action == "labeled" && ev.Label.Name != "" {
		data["label_name"] = ev.Label.Name
		data["dedup_key"] = fmt.Sprintf("github:issues:labeled:%s:%s", issue.HTMLURL, ev.Label.Name)
	}
	return data
}

func parseGitHubIssueComment(body []byte) map[string]string {
	var ev struct {
		Action  string `json:"action"`
		Comment struct {
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"comment"`
		Issue struct {
			Number      int    `json:"number"`
			Title       string `json:"title"`
			HTMLURL     string `json:"html_url"`
			PullRequest *struct {
				HTMLURL string `json:"html_url"`
			} `json:"pull_request"`
		} `json:"issue"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	_ = json.Unmarshal(body, &ev)

	// GitHub's "issue_comment" event is fired for both issues AND PR comments
	// (PRs are issues under the hood). We point source_url at the parent
	// resource (PR if present, else the issue) so reverse-lookup matches the
	// original create_issue event.
	parentURL := ev.Issue.HTMLURL
	parentKind := "issue"
	if ev.Issue.PullRequest != nil && ev.Issue.PullRequest.HTMLURL != "" {
		parentURL = ev.Issue.PullRequest.HTMLURL
		parentKind = "pr"
	}

	return map[string]string{
		"action":       ev.Action,
		"number":       fmt.Sprintf("%d", ev.Issue.Number),
		"issue_title":  ev.Issue.Title,
		"comment_body": ev.Comment.Body,
		"user":         ev.Comment.User.Login,
		"repo":         ev.Repository.FullName,
		"html_url":     ev.Comment.HTMLURL,
		"issue_url":    ev.Issue.HTMLURL,
		"source_url":   parentURL,
		"source_kind":  parentKind,
		"external_id":  fmt.Sprintf("%s#%d", ev.Repository.FullName, ev.Issue.Number),
		"title":        fmt.Sprintf("Comment on %s#%d", ev.Repository.FullName, ev.Issue.Number),
		"body":         fmt.Sprintf("**Comment on [%s#%d](%s): %s**\n**By:** %s\n\n%s", ev.Repository.FullName, ev.Issue.Number, ev.Issue.HTMLURL, ev.Issue.Title, ev.Comment.User.Login, ev.Comment.Body),
	}
}

func parseGitHubCheckRun(body []byte) map[string]string {
	var ev struct {
		Action   string `json:"action"`
		CheckRun struct {
			Name         string `json:"name"`
			Status       string `json:"status"`
			Conclusion   string `json:"conclusion"`
			HTMLURL      string `json:"html_url"`
			HeadSha      string `json:"head_sha"`
			PullRequests []struct {
				HTMLURL string `json:"html_url"`
			} `json:"pull_requests"`
		} `json:"check_run"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	_ = json.Unmarshal(body, &ev)
	sourceURL := ev.CheckRun.HTMLURL
	sourceKind := "commit"
	if len(ev.CheckRun.PullRequests) > 0 && ev.CheckRun.PullRequests[0].HTMLURL != "" {
		sourceURL = ev.CheckRun.PullRequests[0].HTMLURL
		sourceKind = "pr"
	}
	return map[string]string{
		"action":      ev.Action,
		"repo":        ev.Repository.FullName,
		"check_name":  ev.CheckRun.Name,
		"status":      ev.CheckRun.Status,
		"conclusion":  ev.CheckRun.Conclusion,
		"html_url":    ev.CheckRun.HTMLURL,
		"sha":         ev.CheckRun.HeadSha,
		"source_url":  sourceURL,
		"source_kind": sourceKind,
		"external_id": ev.Repository.FullName + "@" + ev.CheckRun.HeadSha,
		"title":       fmt.Sprintf("Check run %s: %s", ev.CheckRun.Name, ev.CheckRun.Conclusion),
		"body":        fmt.Sprintf("**Check run:** %s\n**Status:** %s\n**Conclusion:** %s\n**Commit:** `%s`\n**Details:** %s", ev.CheckRun.Name, ev.CheckRun.Status, ev.CheckRun.Conclusion, ev.CheckRun.HeadSha, ev.CheckRun.HTMLURL),
	}
}

func parseGitHubCheckSuite(body []byte) map[string]string {
	var ev struct {
		Action     string `json:"action"`
		CheckSuite struct {
			Status       string `json:"status"`
			Conclusion   string `json:"conclusion"`
			HeadSha      string `json:"head_sha"`
			PullRequests []struct {
				HTMLURL string `json:"html_url"`
			} `json:"pull_requests"`
		} `json:"check_suite"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	_ = json.Unmarshal(body, &ev)
	sourceURL := ""
	sourceKind := "commit"
	if len(ev.CheckSuite.PullRequests) > 0 && ev.CheckSuite.PullRequests[0].HTMLURL != "" {
		sourceURL = ev.CheckSuite.PullRequests[0].HTMLURL
		sourceKind = "pr"
	}
	return map[string]string{
		"action":      ev.Action,
		"repo":        ev.Repository.FullName,
		"check_name":  "check suite",
		"status":      ev.CheckSuite.Status,
		"conclusion":  ev.CheckSuite.Conclusion,
		"sha":         ev.CheckSuite.HeadSha,
		"source_url":  sourceURL,
		"source_kind": sourceKind,
		"external_id": ev.Repository.FullName + "@" + ev.CheckSuite.HeadSha,
		"title":       fmt.Sprintf("Check suite: %s", ev.CheckSuite.Conclusion),
		"body":        fmt.Sprintf("**Check suite status:** %s\n**Conclusion:** %s\n**Commit:** `%s`", ev.CheckSuite.Status, ev.CheckSuite.Conclusion, ev.CheckSuite.HeadSha),
	}
}

func parseGitHubStatus(body []byte) map[string]string {
	var ev struct {
		State       string `json:"state"`
		Context     string `json:"context"`
		Description string `json:"description"`
		TargetURL   string `json:"target_url"`
		Sha         string `json:"sha"`
		Repository  struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	_ = json.Unmarshal(body, &ev)
	return map[string]string{
		"action":      ev.State,
		"repo":        ev.Repository.FullName,
		"check_name":  ev.Context,
		"status":      ev.State,
		"conclusion":  ev.State,
		"html_url":    ev.TargetURL,
		"sha":         ev.Sha,
		"source_url":  ev.TargetURL,
		"source_kind": "commit",
		"external_id": ev.Repository.FullName + "@" + ev.Sha,
		"title":       fmt.Sprintf("Status %s: %s", ev.Context, ev.State),
		"body":        fmt.Sprintf("**Status:** %s\n**Context:** %s\n**Description:** %s\n**Commit:** `%s`\n**Details:** %s", ev.State, ev.Context, ev.Description, ev.Sha, ev.TargetURL),
	}
}

func parseGitHubWorkflowRun(body []byte) map[string]string {
	var ev struct {
		Action      string `json:"action"`
		WorkflowRun struct {
			Name         string `json:"name"`
			Status       string `json:"status"`
			Conclusion   string `json:"conclusion"`
			HTMLURL      string `json:"html_url"`
			HeadSha      string `json:"head_sha"`
			HeadBranch   string `json:"head_branch"`
			PullRequests []struct {
				URL    string `json:"url"`
				Number int    `json:"number"`
			} `json:"pull_requests"`
		} `json:"workflow_run"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	_ = json.Unmarshal(body, &ev)
	sourceURL := ev.WorkflowRun.HTMLURL
	sourceKind := "commit"
	if len(ev.WorkflowRun.PullRequests) > 0 && ev.WorkflowRun.PullRequests[0].Number > 0 {
		sourceURL = fmt.Sprintf("https://github.com/%s/pull/%d", ev.Repository.FullName, ev.WorkflowRun.PullRequests[0].Number)
		sourceKind = "pr"
	}
	return map[string]string{
		"action":      ev.Action,
		"repo":        ev.Repository.FullName,
		"check_name":  ev.WorkflowRun.Name,
		"status":      ev.WorkflowRun.Status,
		"conclusion":  ev.WorkflowRun.Conclusion,
		"html_url":    ev.WorkflowRun.HTMLURL,
		"sha":         ev.WorkflowRun.HeadSha,
		"branch":      ev.WorkflowRun.HeadBranch,
		"source_url":  sourceURL,
		"source_kind": sourceKind,
		"external_id": ev.Repository.FullName + "@" + ev.WorkflowRun.HeadSha,
		"title":       fmt.Sprintf("Workflow %s: %s", ev.WorkflowRun.Name, ev.WorkflowRun.Conclusion),
		"body":        fmt.Sprintf("**Workflow:** %s\n**Status:** %s\n**Conclusion:** %s\n**Branch:** `%s`\n**Commit:** `%s`\n**Details:** %s", ev.WorkflowRun.Name, ev.WorkflowRun.Status, ev.WorkflowRun.Conclusion, ev.WorkflowRun.HeadBranch, ev.WorkflowRun.HeadSha, ev.WorkflowRun.HTMLURL),
	}
}

func (a *githubAdapter) Keys() []AdapterKey {
	return []AdapterKey{
		{Key: "title", Description: "Default event title (push/PR/issue/comment summary)", Required: true},
		{Key: "body", Description: "Default formatted markdown body", Required: true},
		{Key: "action", Description: "GitHub action (e.g. opened, closed, merged)", Required: false},
		{Key: "repo", Description: "Repository full name (owner/repo)", Required: true},
		{Key: "number", Description: "PR or issue number", Required: false},
		{Key: "user", Description: "GitHub login of the actor", Required: false},
		{Key: "branch", Description: "Branch ref (push only)", Required: false},
		{Key: "head_branch", Description: "PR head branch", Required: false},
		{Key: "base_branch", Description: "PR base branch", Required: false},
		{Key: "html_url", Description: "Direct URL to the PR / issue / comment", Required: false},
		{Key: "labels", Description: "Comma-separated label names attached to the PR or issue", Required: false},
		{Key: "source_url", Description: "Stable URL of the parent resource — used by issue_link reverse lookup", Required: true},
		{Key: "source_urls", Description: "Ordered newline-separated candidate source URLs for reverse lookup", Required: false},
		{Key: "source_kind", Description: "'pr' | 'issue' | 'commit'", Required: true},
		{Key: "external_id", Description: "Compact id like 'owner/repo#123'", Required: false},
		{Key: "comment_body", Description: "Full comment body (issue_comment only)", Required: false},
		{Key: "commits", Description: "Markdown list of commits (push only)", Required: false},
		{Key: "check_name", Description: "Check/status/workflow name (CI events only)", Required: false},
		{Key: "status", Description: "Check/status/workflow status (CI events only)", Required: false},
		{Key: "conclusion", Description: "Check/status/workflow conclusion (CI events only)", Required: false},
		{Key: "sha", Description: "Commit SHA (CI events only)", Required: false},
	}
}

func (a *githubAdapter) Example() string {
	return `{
  "action": "opened",
  "pull_request": {
    "number": 42,
    "title": "Add feature X",
    "body": "Implements feature X.",
    "html_url": "https://github.com/owner/repo/pull/42",
    "user": { "login": "octocat" },
    "head": { "ref": "feature-x" },
    "base": { "ref": "main" }
  },
  "repository": { "full_name": "owner/repo" }
}`
}
