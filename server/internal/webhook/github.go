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

// isRelevantGitHubAction filters event actions so noisy webhooks like label
// changes or assignment churn never reach the action pipeline.
func isRelevantGitHubAction(eventType, action string) bool {
	switch eventType {
	case "push":
		return true
	case "pull_request":
		switch action {
		case "opened", "synchronize", "reopened", "closed":
			return true
		}
		return false
	case "issues":
		switch action {
		case "opened", "reopened", "closed":
			return true
		}
		return false
	case "issue_comment":
		return action == "created"
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
		} `json:"pull_request"`
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

	return map[string]string{
		"action":      action,
		"number":      fmt.Sprintf("%d", pr.Number),
		"title":       pr.Title,
		"user":        pr.User.Login,
		"repo":        ev.Repository.FullName,
		"html_url":    pr.HTMLURL,
		"head_branch": pr.Head.Ref,
		"base_branch": pr.Base.Ref,
		"source_url":  pr.HTMLURL,
		"source_urls": strings.Join(sourceURLs, "\n"),
		"source_kind": "pr",
		"external_id": fmt.Sprintf("%s#%d", ev.Repository.FullName, pr.Number),
		"body":        fmt.Sprintf("**PR [#%d](%s): %s**\n**Author:** %s\n**Branch:** `%s` → `%s`\n**Action:** %s\n\n%s", pr.Number, pr.HTMLURL, pr.Title, pr.User.Login, pr.Head.Ref, pr.Base.Ref, action, pr.Body),
	}
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

	return map[string]string{
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
		{Key: "source_url", Description: "Stable URL of the parent resource — used by issue_link reverse lookup", Required: true},
		{Key: "source_urls", Description: "Ordered newline-separated candidate source URLs for reverse lookup", Required: false},
		{Key: "source_kind", Description: "'pr' | 'issue' | 'commit'", Required: true},
		{Key: "external_id", Description: "Compact id like 'owner/repo#123'", Required: false},
		{Key: "comment_body", Description: "Full comment body (issue_comment only)", Required: false},
		{Key: "commits", Description: "Markdown list of commits (push only)", Required: false},
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
