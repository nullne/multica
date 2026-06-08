package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	wh "github.com/nullne/multica/server/internal/webhook"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

// --- Action config schemas ---

// CreateIssueActionConfig is the config schema for action_type = "create_issue".
type CreateIssueActionConfig struct {
	AgentID             string   `json:"agent_id"`
	TitleTemplate       string   `json:"title_template"`
	DescriptionTemplate string   `json:"description_template"`
	Labels              []string `json:"labels"`
	DispatchProvider    string   `json:"dispatch_provider,omitempty"`
	DispatchDaemonID    string   `json:"dispatch_daemon_id,omitempty"`
	DispatchDaemonLabel string   `json:"dispatch_daemon_label,omitempty"`
	// Optional event filters; matched against Event.Type and Event.Data["repo"].
	EventTypes []string `json:"event_types,omitempty"`
	Repos      []string `json:"repos,omitempty"`
	// Optional GitHub label filter. When non-empty, the action only matches
	// PR/issue events whose label set includes at least one of these names.
	GitHubLabels []string `json:"github_labels,omitempty"`
	// Optional member user IDs to subscribe to issues created by this action.
	SubscriberIDs []string `json:"subscriber_ids,omitempty"`
}

// CommentIssueActionConfig is the config schema for action_type = "comment_issue".
// The action looks up the target Multica issue(s) by Event.Data["source_url"]
// against issue_link, then posts a templated comment authored by the bot user
// identified by BotUserID. When MentionAgentID is set, the rendered content is
// appended with an @mention link, which lets the existing on_mention trigger
// pick it up.
type CommentIssueActionConfig struct {
	ContentTemplate string `json:"content_template"`
	BotUserID       string `json:"bot_user_id,omitempty"`
	MentionAgentID  string `json:"mention_agent_id,omitempty"`
	// OnlyIfIssueAutoFixEnabled gates the comment on the per-issue
	// issue.github_auto_fix_enabled opt-in. Used by the managed GitHub
	// auto-fix routine so feedback is only relayed to opted-in issues.
	OnlyIfIssueAutoFixEnabled bool     `json:"only_if_issue_auto_fix_enabled,omitempty"`
	EventTypes                []string `json:"event_types,omitempty"`
	Repos                     []string `json:"repos,omitempty"`
	GitHubLabels              []string `json:"github_labels,omitempty"`
}

// actionMatchesFilters returns true when the action should run for this event,
// based on the EventTypes/Repos/GitHubLabels filters. Empty filter lists
// mean "match all" — except for `*.labeled` events: those only fire when
// the action has opted in via a non-empty GitHubLabels filter, so existing
// actions configured before label filtering existed do not start receiving
// extra triggers when a label is added to a PR/issue.
func actionMatchesFilters(eventTypes, repos, githubLabels []string, evt wh.Event) bool {
	explicitEventTypeMatch := false
	for _, t := range eventTypes {
		if t == evt.Type {
			explicitEventTypeMatch = true
			break
		}
	}
	if isGitHubLabeledEvent(evt.Type) && len(githubLabels) == 0 && !explicitEventTypeMatch {
		return false
	}
	if len(eventTypes) > 0 {
		matched := false
		for _, t := range eventTypes {
			if t == "" {
				continue
			}
			if t == evt.Type || strings.HasPrefix(evt.Type, t+".") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(repos) > 0 {
		repo := evt.Data["repo"]
		matched := false
		for _, r := range repos {
			if r == "" {
				continue
			}
			if r == repo {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(githubLabels) > 0 {
		eventLabels := parseGitHubLabels(evt.Data["labels"])
		matched := false
		for _, want := range githubLabels {
			want = strings.TrimSpace(want)
			if want == "" {
				continue
			}
			if _, ok := eventLabels[want]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// isGitHubLabeledEvent reports whether evt.Type is one of the GitHub
// `labeled` action events emitted by the github adapter.
func isGitHubLabeledEvent(eventType string) bool {
	return eventType == "github.pull_request.labeled" || eventType == "github.issues.labeled"
}

// parseGitHubLabels turns the comma-separated label string the github
// adapter writes into Event.Data["labels"] back into a set for membership
// checks.
func parseGitHubLabels(raw string) map[string]struct{} {
	out := make(map[string]struct{})
	if raw == "" {
		return out
	}
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

// findIssueLinksForEvent resolves every Multica issue linked to the event's
// source URL(s) via issue_link. Duplicate issues are returned once.
func (h *Handler) findIssueLinksForEvent(ctx context.Context, workspaceID pgtype.UUID, evt wh.Event) ([]db.IssueLink, error) {
	seenIssues := map[string]bool{}
	links := []db.IssueLink{}
	for _, sourceURL := range eventSourceURLs(evt) {
		matches, err := h.Queries.ListIssueLinksByURL(ctx, db.ListIssueLinksByURLParams{
			WorkspaceID: workspaceID,
			Url:         sourceURL,
		})
		if err != nil {
			return nil, fmt.Errorf("lookup issue_link: %w", err)
		}
		for _, link := range matches {
			issueID := uuidToString(link.IssueID)
			if seenIssues[issueID] {
				continue
			}
			seenIssues[issueID] = true
			links = append(links, link)
		}
	}
	return links, nil
}

func eventSourceURLs(evt wh.Event) []string {
	seen := make(map[string]bool)
	var urls []string
	add := func(url string) {
		url = strings.TrimSpace(url)
		if url == "" || seen[url] {
			return
		}
		seen[url] = true
		urls = append(urls, url)
	}

	for _, sourceURL := range strings.Split(evt.Data["source_urls"], "\n") {
		add(sourceURL)
	}
	add(evt.Data["source_url"])
	return urls
}

// commentForBroadcast builds the lightweight payload published over WS for
// new comments. We avoid pulling reactions/attachments here since
// system-generated comments never have either at creation time.
func commentForBroadcast(c db.Comment) map[string]any {
	return map[string]any{
		"id":           uuidToString(c.ID),
		"issue_id":     uuidToString(c.IssueID),
		"workspace_id": uuidToString(c.WorkspaceID),
		"author_type":  c.AuthorType,
		"author_id":    uuidToString(c.AuthorID),
		"content":      c.Content,
		"type":         c.Type,
		"parent_id":    uuidToPtr(c.ParentID),
		"created_at":   timestampToString(c.CreatedAt),
		"updated_at":   timestampToString(c.UpdatedAt),
	}
}

// renderTemplate replaces {{.key}} placeholders with values from data.
func renderTemplate(tmpl string, data map[string]string) string {
	if tmpl == "" {
		return ""
	}
	result := tmpl
	for k, v := range data {
		result = strings.ReplaceAll(result, "{{."+k+"}}", v)
	}
	return result
}
