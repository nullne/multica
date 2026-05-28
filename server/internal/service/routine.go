package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/util"
	db "github.com/nullne/multica/server/pkg/db/generated"
)

type routineTxStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type RoutineService struct {
	Queries     *db.Queries
	TxStarter   routineTxStarter
	TaskService *TaskService
}

func NewRoutineService(queries *db.Queries, txStarter routineTxStarter, taskService *TaskService) *RoutineService {
	return &RoutineService{
		Queries:     queries,
		TxStarter:   txStarter,
		TaskService: taskService,
	}
}

type RoutineEvent struct {
	Type     string
	DedupKey string
	Data     map[string]string
	Payload  []byte
}

type RoutineCreateIssueConfig struct {
	TitleTemplate       string   `json:"title_template"`
	DescriptionTemplate string   `json:"description_template"`
	Labels              []string `json:"labels"`
	SubscriberIDs       []string `json:"subscriber_ids"`
}

type RoutineCommentIssueConfig struct {
	ContentTemplate string `json:"content_template"`
	BotUserID       string `json:"bot_user_id"`
	MentionAgentID  string `json:"mention_agent_id,omitempty"`
}

type RoutineActionResult struct {
	Ran       bool
	IssueID   pgtype.UUID
	CommentID pgtype.UUID
}

func (s *RoutineService) ExecuteAction(ctx context.Context, routine db.Routine, trigger db.RoutineTrigger, action db.RoutineAction, evt RoutineEvent) (RoutineActionResult, error) {
	switch action.ActionType {
	case "create_issue":
		return s.executeCreateIssue(ctx, routine, trigger, action, evt)
	case "comment_issue":
		return s.executeCommentIssue(ctx, routine, trigger, action, evt)
	default:
		return RoutineActionResult{}, fmt.Errorf("unknown routine action type: %s", action.ActionType)
	}
}

func (s *RoutineService) executeCreateIssue(ctx context.Context, routine db.Routine, trigger db.RoutineTrigger, action db.RoutineAction, evt RoutineEvent) (RoutineActionResult, error) {
	var cfg RoutineCreateIssueConfig
	if len(action.Config) > 0 {
		if err := json.Unmarshal(action.Config, &cfg); err != nil {
			_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, "invalid create_issue config: "+err.Error())
			return RoutineActionResult{}, fmt.Errorf("invalid create_issue config: %w", err)
		}
	}

	if link, ok, err := s.findIssueLinkForEvent(ctx, routine.WorkspaceID, evt); err != nil {
		_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, err.Error())
		return RoutineActionResult{}, err
	} else if ok {
		if err := s.persistEventSourceLinks(ctx, s.Queries, routine.WorkspaceID, trigger.TriggerType, link.IssueID, evt); err != nil {
			_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, err.Error())
			return RoutineActionResult{}, err
		}
		_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "processed", link.IssueID, pgtype.UUID{}, "")
		return RoutineActionResult{Ran: true, IssueID: link.IssueID}, nil
	}

	title := renderRoutineTemplate(cfg.TitleTemplate, evt.Data)
	if title == "" {
		title = evt.Data["title"]
	}

	description := renderRoutineTemplate(cfg.DescriptionTemplate, evt.Data)
	if description == "" {
		description = routine.Instructions.String
	}
	if description == "" {
		description = evt.Data["body"]
	}

	priority := routine.Priority
	if priority == "" {
		priority = "medium"
	}
	if evtPriority := evt.Data["priority"]; evtPriority != "" {
		priority = evtPriority
	}

	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return RoutineActionResult{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.Queries.WithTx(tx)
	number, err := qtx.IncrementIssueCounter(ctx, routine.WorkspaceID)
	if err != nil {
		return RoutineActionResult{}, err
	}

	dueDate := pgtype.Timestamptz{}
	if routine.DueDateOffsetHours.Valid {
		offset := time.Duration(routine.DueDateOffsetHours.Int32) * time.Hour
		dueDate = pgtype.Timestamptz{Time: time.Now().Add(offset), Valid: true}
	}

	issue, err := qtx.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID:           routine.WorkspaceID,
		Title:                 title,
		Description:           strToText(description),
		Status:                "todo",
		Priority:              priority,
		AssigneeType:          routine.AssigneeType,
		AssigneeID:            routine.AssigneeID,
		CreatorType:           routine.CreatedByType,
		CreatorID:             routine.CreatedByID,
		VerifierAgentID:       pgtype.UUID{},
		ParentIssueID:         pgtype.UUID{},
		Position:              0,
		DueDate:               dueDate,
		Number:                number,
		MaxVerificationRounds: pgtype.Int4{},
		DispatchProvider:      routine.DispatchProvider,
		DispatchDaemonID:      routine.DispatchDaemonID,
		DispatchDaemonLabel:   routine.DispatchDaemonLabel,
		GithubAutoFixEnabled:  routine.GithubAutoFixEnabled,
	})
	if err != nil {
		return RoutineActionResult{}, err
	}

	labels, err := qtx.ListRoutineLabels(ctx, routine.ID)
	if err == nil {
		for _, label := range labels {
			_ = qtx.AddLabelToIssue(ctx, db.AddLabelToIssueParams{IssueID: issue.ID, LabelID: label.LabelID})
		}
	}
	for _, labelID := range cfg.Labels {
		lid := util.ParseUUID(labelID)
		if lid.Valid {
			_ = qtx.AddLabelToIssue(ctx, db.AddLabelToIssueParams{IssueID: issue.ID, LabelID: lid})
		}
	}

	subs, err := qtx.ListRoutineSubscribers(ctx, routine.ID)
	if err == nil {
		for _, sub := range subs {
			_ = qtx.AddIssueSubscriber(ctx, db.AddIssueSubscriberParams{IssueID: issue.ID, UserType: "member", UserID: sub.UserID, Reason: "manual"})
		}
	}
	for _, subID := range cfg.SubscriberIDs {
		uid := util.ParseUUID(subID)
		if uid.Valid {
			_ = qtx.AddIssueSubscriber(ctx, db.AddIssueSubscriberParams{IssueID: issue.ID, UserType: "member", UserID: uid, Reason: "manual"})
		}
	}

	if err := s.persistEventSourceLinks(ctx, qtx, routine.WorkspaceID, trigger.TriggerType, issue.ID, evt); err != nil {
		return RoutineActionResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return RoutineActionResult{}, err
	}

	if s.TaskService != nil && issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" && issue.AssigneeID.Valid {
		_, _ = s.TaskService.EnqueueTaskForIssueWithContext(ctx, issue, routineTaskContext(evt))
	}

	_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "processed", issue.ID, pgtype.UUID{}, "")
	return RoutineActionResult{Ran: true, IssueID: issue.ID}, nil
}

func (s *RoutineService) executeCommentIssue(ctx context.Context, routine db.Routine, trigger db.RoutineTrigger, action db.RoutineAction, evt RoutineEvent) (RoutineActionResult, error) {
	var cfg RoutineCommentIssueConfig
	if err := json.Unmarshal(action.Config, &cfg); err != nil {
		_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, "invalid comment_issue config: "+err.Error())
		return RoutineActionResult{}, fmt.Errorf("invalid comment_issue config: %w", err)
	}
	botUserID := util.ParseUUID(cfg.BotUserID)
	if !botUserID.Valid {
		err := fmt.Errorf("comment_issue requires bot_user_id")
		_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, err.Error())
		return RoutineActionResult{}, err
	}

	link, ok, err := s.findIssueLinkForEvent(ctx, routine.WorkspaceID, evt)
	if err != nil {
		_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", pgtype.UUID{}, pgtype.UUID{}, err.Error())
		return RoutineActionResult{}, err
	}
	if !ok {
		_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "filtered", pgtype.UUID{}, pgtype.UUID{}, "no matching issue link")
		return RoutineActionResult{Ran: false}, nil
	}

	content := renderRoutineTemplate(cfg.ContentTemplate, evt.Data)
	if content == "" {
		content = evt.Data["body"]
	}
	if content == "" {
		content = "(empty routine event)"
	}
	if cfg.MentionAgentID != "" {
		agent, agentErr := s.Queries.GetAgent(ctx, util.ParseUUID(cfg.MentionAgentID))
		if agentErr == nil && util.UUIDToString(agent.WorkspaceID) == util.UUIDToString(routine.WorkspaceID) {
			content = strings.TrimRight(content, "\n") + fmt.Sprintf("\n\n[@%s](mention://agent/%s)", agent.Name, cfg.MentionAgentID)
		}
	}

	issue, err := s.Queries.GetIssue(ctx, link.IssueID)
	if err != nil {
		return RoutineActionResult{}, err
	}
	comment, err := s.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "member",
		AuthorID:    botUserID,
		Content:     content,
		Type:        "comment",
	})
	if err != nil {
		_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "error", link.IssueID, pgtype.UUID{}, err.Error())
		return RoutineActionResult{}, err
	}
	_ = s.logRun(ctx, routine.ID, trigger.ID, action.ID, evt, "processed", link.IssueID, comment.ID, "")
	return RoutineActionResult{Ran: true, IssueID: link.IssueID, CommentID: comment.ID}, nil
}

func (s *RoutineService) findIssueLinkForEvent(ctx context.Context, workspaceID pgtype.UUID, evt RoutineEvent) (db.IssueLink, bool, error) {
	for _, sourceURL := range routineEventSourceURLs(evt) {
		link, err := s.Queries.GetIssueLinkByURL(ctx, db.GetIssueLinkByURLParams{WorkspaceID: workspaceID, Url: sourceURL})
		if err == nil {
			return link, true, nil
		}
		if err == pgx.ErrNoRows {
			continue
		}
		return db.IssueLink{}, false, fmt.Errorf("lookup issue_link: %w", err)
	}
	return db.IssueLink{}, false, nil
}

func (s *RoutineService) persistEventSourceLinks(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID, sourceType string, issueID pgtype.UUID, evt RoutineEvent) error {
	for _, sourceURL := range routineEventSourceURLs(evt) {
		_, err := queries.CreateIssueLink(ctx, db.CreateIssueLinkParams{
			IssueID:     issueID,
			WorkspaceID: workspaceID,
			SourceType:  sourceType,
			Kind:        routineEventSourceKind(evt, sourceURL),
			Direction:   "source",
			Url:         sourceURL,
			ExternalID:  evt.Data["external_id"],
		})
		if err != nil {
			return fmt.Errorf("create issue_link: %w", err)
		}
	}
	return nil
}

func routineEventSourceURLs(evt RoutineEvent) []string {
	seen := map[string]bool{}
	var urls []string
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			return
		}
		seen[raw] = true
		urls = append(urls, raw)
	}
	for _, sourceURL := range strings.Split(evt.Data["source_urls"], "\n") {
		add(sourceURL)
	}
	add(evt.Data["source_url"])
	return urls
}

func routineEventSourceKind(evt RoutineEvent, sourceURL string) string {
	if sourceURL == evt.Data["source_url"] && evt.Data["source_kind"] != "" {
		return evt.Data["source_kind"]
	}
	if strings.Contains(sourceURL, "/pull/") {
		return "pr"
	}
	if strings.Contains(sourceURL, "/issues/") {
		return "issue"
	}
	if evt.Data["source_kind"] != "" {
		return evt.Data["source_kind"]
	}
	return "issue"
}

func (s *RoutineService) logRun(ctx context.Context, routineID, triggerID, actionID pgtype.UUID, evt RoutineEvent, status string, issueID, commentID pgtype.UUID, errorMessage string) error {
	payload := evt.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	_, err := s.Queries.CreateRoutineRun(ctx, db.CreateRoutineRunParams{
		RoutineID:    routineID,
		TriggerID:    triggerID,
		ActionID:     actionID,
		EventType:    evt.Type,
		DedupKey:     evt.DedupKey,
		Payload:      payload,
		Status:       status,
		IssueID:      issueID,
		CommentID:    commentID,
		ErrorMessage: util.StrToText(errorMessage),
	})
	return err
}

func renderRoutineTemplate(tmpl string, data map[string]string) string {
	if tmpl == "" {
		return ""
	}
	result := tmpl
	for k, v := range data {
		result = strings.ReplaceAll(result, "{{."+k+"}}", v)
	}
	return result
}

func strToText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func routineTaskContext(evt RoutineEvent) []byte {
	rawPayload := evt.Data["raw_payload"]
	if rawPayload == "" && len(evt.Payload) > 0 {
		rawPayload = string(evt.Payload)
	}
	routineEvent := map[string]any{
		"type":        evt.Type,
		"dedup_key":   evt.DedupKey,
		"raw_payload": rawPayload,
	}
	var parsedPayload any
	if rawPayload != "" && json.Unmarshal([]byte(rawPayload), &parsedPayload) == nil {
		routineEvent["payload"] = parsedPayload
	}
	context := map[string]any{
		"routine_event": routineEvent,
	}
	b, err := json.Marshal(context)
	if err != nil {
		return nil
	}
	return b
}
