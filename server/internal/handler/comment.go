package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nullne/multica/server/internal/logger"
	"github.com/nullne/multica/server/internal/mention"
	"github.com/nullne/multica/server/internal/util"
	db "github.com/nullne/multica/server/pkg/db/generated"
	"github.com/nullne/multica/server/pkg/protocol"
)

type CommentResponse struct {
	ID          string               `json:"id"`
	IssueID     string               `json:"issue_id"`
	AuthorType  string               `json:"author_type"`
	AuthorID    string               `json:"author_id"`
	Content     string               `json:"content"`
	Type        string               `json:"type"`
	ParentID    *string              `json:"parent_id"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
	Reactions   []ReactionResponse   `json:"reactions"`
	Attachments []AttachmentResponse `json:"attachments"`
}

func commentToResponse(c db.Comment, reactions []ReactionResponse, attachments []AttachmentResponse) CommentResponse {
	if reactions == nil {
		reactions = []ReactionResponse{}
	}
	if attachments == nil {
		attachments = []AttachmentResponse{}
	}
	return CommentResponse{
		ID:          uuidToString(c.ID),
		IssueID:     uuidToString(c.IssueID),
		AuthorType:  c.AuthorType,
		AuthorID:    uuidToString(c.AuthorID),
		Content:     c.Content,
		Type:        c.Type,
		ParentID:    uuidToPtr(c.ParentID),
		CreatedAt:   timestampToString(c.CreatedAt),
		UpdatedAt:   timestampToString(c.UpdatedAt),
		Reactions:   reactions,
		Attachments: attachments,
	}
}

func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, issueID)
	if !ok {
		return
	}

	comments, err := h.Queries.ListComments(r.Context(), db.ListCommentsParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list comments")
		return
	}

	commentIDs := make([]pgtype.UUID, len(comments))
	for i, c := range comments {
		commentIDs[i] = c.ID
	}
	grouped := h.groupReactions(r, commentIDs)
	groupedAtt := h.groupAttachments(r, commentIDs)

	resp := make([]CommentResponse, len(comments))
	for i, c := range comments {
		cid := uuidToString(c.ID)
		resp[i] = commentToResponse(c, grouped[cid], groupedAtt[cid])
	}

	writeJSON(w, http.StatusOK, resp)
}

type CreateCommentRequest struct {
	Content       string   `json:"content"`
	Type          string   `json:"type"`
	ParentID      *string  `json:"parent_id"`
	AttachmentIDs []string `json:"attachment_ids"`
}

func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, issueID)
	if !ok {
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	if req.Type == "" {
		req.Type = "comment"
	}

	var parentID pgtype.UUID
	var parentComment *db.Comment
	if req.ParentID != nil {
		parentID = parseUUID(*req.ParentID)
		parent, err := h.Queries.GetComment(r.Context(), parentID)
		if err != nil || uuidToString(parent.IssueID) != issueID {
			writeError(w, http.StatusBadRequest, "invalid parent comment")
			return
		}
		parentComment = &parent
	}

	// Determine author identity: agent (via X-Agent-ID header) or member.
	authorType, authorID := h.resolveActor(r, userID, uuidToString(issue.WorkspaceID))

	// Expand bare issue identifiers (e.g. MUL-117) into mention links.
	req.Content = mention.ExpandIssueIdentifiers(r.Context(), h.Queries, issue.WorkspaceID, req.Content)

	comment, err := h.Queries.CreateComment(r.Context(), db.CreateCommentParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  authorType,
		AuthorID:    parseUUID(authorID),
		Content:     req.Content,
		Type:        req.Type,
		ParentID:    parentID,
	})
	if err != nil {
		slog.Warn("create comment failed", append(logger.RequestAttrs(r), "error", err, "issue_id", issueID)...)
		writeError(w, http.StatusInternalServerError, "failed to create comment: "+err.Error())
		return
	}

	// Link uploaded attachments to this comment.
	if len(req.AttachmentIDs) > 0 {
		h.linkAttachmentsByIDs(r.Context(), comment.ID, issue.ID, req.AttachmentIDs)
	}

	// Fetch linked attachments so the response includes them.
	groupedAtt := h.groupAttachments(r, []pgtype.UUID{comment.ID})
	resp := commentToResponse(comment, nil, groupedAtt[uuidToString(comment.ID)])
	slog.Info("comment created", append(logger.RequestAttrs(r), "comment_id", uuidToString(comment.ID), "issue_id", issueID)...)
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), authorType, authorID, map[string]any{
		"comment":             resp,
		"issue_title":         issue.Title,
		"issue_assignee_type": textToPtr(issue.AssigneeType),
		"issue_assignee_id":   uuidToPtr(issue.AssigneeID),
		"issue_status":        issue.Status,
	})

	// A human member comment counts as explicit human intervention and resets
	// any in-progress agent-to-agent mention chain on this issue.
	if authorType == "member" {
		if err := h.Queries.ResetIssueAgentMentionChain(r.Context(), issue.ID); err != nil {
			slog.Warn("reset agent mention chain failed", "issue_id", issueID, "error", err)
		}
	}

	// If the issue is assigned to an agent with on_comment trigger, enqueue a new task.
	// Skip when the comment comes from the assigned agent itself to avoid loops.
	// Also skip when the comment @mentions others but not the assignee agent —
	// the user is talking to someone else, not requesting work from the assignee.
	// Also skip when replying in a member-started thread without mentioning the
	// assignee — the user is continuing a member-to-member conversation.
	onCommentEnqueued := false
	if authorType == "member" && h.shouldEnqueueOnComment(r.Context(), issue) &&
		!h.commentMentionsOthersButNotAssignee(comment.Content, issue) &&
		!h.isReplyToMemberThread(parentComment, comment.Content, issue) {
		// Resolve thread root: if the comment is a reply, agent should reply
		// to the thread root (matching frontend behavior where all replies
		// in a thread share the same top-level parent).
		replyTo := comment.ID
		if comment.ParentID.Valid {
			replyTo = comment.ParentID
		}
		if _, err := h.TaskService.EnqueueTaskForIssue(r.Context(), issue, replyTo); err != nil {
			slog.Warn("enqueue agent task on comment failed", "issue_id", issueID, "error", err)
		} else {
			onCommentEnqueued = true
		}
	}

	// Trigger @mentioned agents: parse agent mentions and enqueue tasks for each.
	h.enqueueMentionedAgentTasks(r.Context(), issue, comment, authorType, authorID, onCommentEnqueued)

	writeJSON(w, http.StatusCreated, resp)
}

// commentMentionsOthersButNotAssignee returns true if the comment @mentions
// anyone but does NOT @mention the issue's assignee agent. This is used to
// suppress the on_comment trigger when the user is directing their comment at
// someone else (e.g. sharing results with a colleague, asking another agent).
// @all is treated as a broadcast — it suppresses the trigger because the user
// is announcing to everyone, not specifically requesting work from the agent.
func (h *Handler) commentMentionsOthersButNotAssignee(content string, issue db.Issue) bool {
	mentions := util.ParseMentions(content)
	if len(mentions) == 0 {
		return false // No mentions — normal on_comment behavior
	}
	// @all is a broadcast to all members — suppress agent trigger.
	if util.HasMentionAll(mentions) {
		return true
	}
	if !issue.AssigneeID.Valid {
		return true // No assignee — mentions target others
	}
	assigneeID := uuidToString(issue.AssigneeID)
	for _, m := range mentions {
		if m.ID == assigneeID {
			return false // Assignee is mentioned — allow trigger
		}
	}
	return true // Others mentioned but not assignee — suppress trigger
}

// isReplyToMemberThread returns true if the comment is a reply in a thread
// started by a member and does NOT @mention the issue's assignee agent.
// When a member replies in a member-started thread, they are most likely
// continuing a human conversation — not requesting work from the assigned agent.
// Replying to an agent-started thread, or explicitly @mentioning the assignee
// in the reply, still triggers on_comment as expected.
func (h *Handler) isReplyToMemberThread(parent *db.Comment, content string, issue db.Issue) bool {
	if parent == nil {
		return false // Not a reply — normal top-level comment
	}
	if parent.AuthorType != "member" {
		return false // Thread started by an agent — allow trigger
	}
	// Thread was started by a member. Suppress on_comment unless the reply
	// explicitly @mentions the assignee agent.
	if !issue.AssigneeID.Valid {
		return true // No assignee to mention
	}
	assigneeID := uuidToString(issue.AssigneeID)
	for _, m := range util.ParseMentions(content) {
		if m.ID == assigneeID {
			return false // Assignee explicitly mentioned — allow trigger
		}
	}
	return true // Reply to member thread without mentioning agent — suppress
}

// postSystemNotice creates a system-authored notice comment on an issue to
// inform users that a mentioned agent could not be dispatched. The comment is
// posted as a reply to the triggering comment thread (or its root if nested).
func (h *Handler) postSystemNotice(ctx context.Context, issue db.Issue, content string, parentCommentID pgtype.UUID) {
	comment, err := h.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "system",
		AuthorID:    issue.WorkspaceID,
		Content:     content,
		Type:        "system",
		ParentID:    parentCommentID,
	})
	if err != nil {
		slog.Warn("post system notice failed", "issue_id", uuidToString(issue.ID), "error", err)
		return
	}
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "system", uuidToString(issue.WorkspaceID), map[string]any{
		"comment": map[string]any{
			"id":          uuidToString(comment.ID),
			"issue_id":    uuidToString(comment.IssueID),
			"author_type": comment.AuthorType,
			"author_id":   uuidToString(comment.AuthorID),
			"content":     comment.Content,
			"type":        comment.Type,
			"parent_id":   uuidToPtr(comment.ParentID),
			"created_at":  timestampToString(comment.CreatedAt),
			"reactions":   []any{},
			"attachments": []any{},
		},
		"issue_title":  issue.Title,
		"issue_status": issue.Status,
	})
}

// AgentMentionChainLimit caps the number of consecutive agent-to-agent mention
// handoffs allowed on a single issue without human intervention. Once reached,
// further agent-triggered mention dispatches are blocked and a system notice is
// posted. A human member comment resets the chain count.
const AgentMentionChainLimit = 5

// enqueueMentionedAgentTasks parses @agent mentions from comment content and
// enqueues a task for each mentioned agent. Skips self-mentions, agents whose
// task was already enqueued via the on_comment trigger above, agents with
// on_mention trigger disabled, and private agents mentioned by non-owner
// members (only the agent owner or workspace admin/owner can mention a
// private agent). When the comment author is an agent, this function also
// enforces the agent-to-agent mention chain limit per issue.
// Note: no status gate here — @mention is an explicit action and should work
// even on done/cancelled issues (the agent can reopen the issue if needed).
func (h *Handler) enqueueMentionedAgentTasks(ctx context.Context, issue db.Issue, comment db.Comment, authorType, authorID string, onCommentEnqueued bool) {
	wsID := uuidToString(issue.WorkspaceID)
	mentions := util.ParseMentions(comment.Content)
	chainLimitNoticePosted := false
	for _, m := range mentions {
		if m.Type != "agent" {
			continue
		}
		// Prevent self-trigger: skip if the comment author is this agent.
		if authorType == "agent" && authorID == m.ID {
			continue
		}
		agentUUID := parseUUID(m.ID)
		// Prevent duplicate: skip if the on_comment trigger already enqueued a
		// task for this assignee in response to the same comment. We only
		// suppress here when on_comment actually fired — agent-authored
		// comments and member comments where on_comment was suppressed (e.g.
		// @all broadcast) still flow through mention dispatch normally.
		if onCommentEnqueued && issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" &&
			issue.AssigneeID.Valid && uuidToString(issue.AssigneeID) == m.ID {
			continue
		}

		// Thread root for any notice replies: use the comment itself as parent,
		// or the root comment if the mention is in a nested reply.
		noticeParent := comment.ID
		if comment.ParentID.Valid {
			noticeParent = comment.ParentID
		}

		// Load the agent to check visibility, archive status, and trigger config.
		agent, err := h.Queries.GetAgent(ctx, agentUUID)
		if err != nil {
			h.postSystemNotice(ctx, issue, "A mentioned agent could not be found.", noticeParent)
			continue
		}
		if agent.ArchivedAt.Valid {
			agentLink := fmt.Sprintf("[@%s](mention://agent/%s)", agent.Name, m.ID)
			h.postSystemNotice(ctx, issue, agentLink+" is archived and cannot accept tasks.", noticeParent)
			continue
		}
		if len(agent.Providers) == 0 {
			agentLink := fmt.Sprintf("[@%s](mention://agent/%s)", agent.Name, m.ID)
			h.postSystemNotice(ctx, issue, agentLink+" has no AI providers configured and cannot accept tasks.", noticeParent)
			continue
		}

		// Private agents can only be mentioned by the agent owner or workspace admin/owner.
		if agent.Visibility == "private" && authorType == "member" {
			isOwner := uuidToString(agent.OwnerID) == authorID
			if !isOwner {
				member, err := h.getWorkspaceMember(ctx, authorID, wsID)
				if err != nil || !roleAllowed(member.Role, "owner", "admin") {
					h.postSystemNotice(ctx, issue, "A mentioned agent could not be dispatched due to access restrictions.", noticeParent)
					continue
				}
			}
		}
		// Check if the agent has on_mention trigger enabled.
		if !agentHasTriggerEnabled(agent.Triggers, "on_mention") {
			agentLink := fmt.Sprintf("[@%s](mention://agent/%s)", agent.Name, m.ID)
			h.postSystemNotice(ctx, issue, agentLink+" has the mention trigger disabled.", noticeParent)
			continue
		}
		// Dedup: skip if this agent already has a pending task for this issue.
		hasPending, err := h.Queries.HasPendingTaskForIssueAndAgent(ctx, db.HasPendingTaskForIssueAndAgentParams{
			IssueID: issue.ID,
			AgentID: agentUUID,
		})
		if err != nil || hasPending {
			if hasPending {
				agentLink := fmt.Sprintf("[@%s](mention://agent/%s)", agent.Name, m.ID)
				h.postSystemNotice(ctx, issue, agentLink+" already has a pending task for this issue.", noticeParent)
			}
			continue
		}
		// Resolve thread root for reply threading.
		replyTo := comment.ID
		if comment.ParentID.Valid {
			replyTo = comment.ParentID
		}

		// Agent-to-agent chain guard. When the comment author is an agent we
		// atomically reserve one slot before enqueueing — a single conditional
		// UPDATE is the serialization point so concurrent agent comments can
		// never collectively exceed AgentMentionChainLimit. If the enqueue
		// itself fails, we release the slot so a subsequent retry has the same
		// budget. Member-authored mentions never consume a slot.
		chainSlotReserved := false
		if authorType == "agent" {
			if _, err := h.Queries.TryReserveIssueAgentMentionChain(ctx, db.TryReserveIssueAgentMentionChainParams{
				ID:       issue.ID,
				MaxCount: AgentMentionChainLimit,
			}); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					if !chainLimitNoticePosted {
						h.postSystemNotice(ctx, issue, fmt.Sprintf(
							"Agent-to-agent mention chain limit reached (%d consecutive handoffs). "+
								"Awaiting human input before further agent dispatches on this issue.",
							AgentMentionChainLimit,
						), noticeParent)
						chainLimitNoticePosted = true
					}
					continue
				}
				slog.Warn("reserve agent mention chain slot failed", "issue_id", uuidToString(issue.ID), "error", err)
				continue
			}
			chainSlotReserved = true
		}

		if _, err := h.TaskService.EnqueueTaskForMention(ctx, issue, agentUUID, replyTo); err != nil {
			if chainSlotReserved {
				if _, derr := h.Queries.ReleaseIssueAgentMentionChainSlot(ctx, issue.ID); derr != nil {
					slog.Warn("release agent mention chain slot failed", "issue_id", uuidToString(issue.ID), "error", derr)
				}
			}
			agentLink := fmt.Sprintf("[@%s](mention://agent/%s)", agent.Name, m.ID)
			h.postSystemNotice(ctx, issue, agentLink+" has no available runtime to execute the task.", noticeParent)
			slog.Warn("enqueue mention agent task failed", "issue_id", uuidToString(issue.ID), "agent_id", m.ID, "error", err)
			continue
		}
	}
}

func (h *Handler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	commentId := chi.URLParam(r, "commentId")

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	// Load comment scoped to current workspace.
	workspaceID := resolveWorkspaceID(r)
	existing, err := h.Queries.GetCommentInWorkspace(r.Context(), db.GetCommentInWorkspaceParams{
		ID:          parseUUID(commentId),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}

	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	isAuthor := existing.AuthorType == actorType && uuidToString(existing.AuthorID) == actorID
	isAdmin := roleAllowed(member.Role, "owner", "admin")
	if !isAuthor && !isAdmin {
		writeError(w, http.StatusForbidden, "only comment author or admin can edit")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	comment, err := h.Queries.UpdateComment(r.Context(), db.UpdateCommentParams{
		ID:      parseUUID(commentId),
		Content: req.Content,
	})
	if err != nil {
		slog.Warn("update comment failed", append(logger.RequestAttrs(r), "error", err, "comment_id", commentId)...)
		writeError(w, http.StatusInternalServerError, "failed to update comment")
		return
	}

	// Fetch reactions and attachments for the updated comment.
	grouped := h.groupReactions(r, []pgtype.UUID{comment.ID})
	groupedAtt := h.groupAttachments(r, []pgtype.UUID{comment.ID})
	cid := uuidToString(comment.ID)
	resp := commentToResponse(comment, grouped[cid], groupedAtt[cid])
	slog.Info("comment updated", append(logger.RequestAttrs(r), "comment_id", commentId)...)
	h.publish(protocol.EventCommentUpdated, workspaceID, actorType, actorID, map[string]any{"comment": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	commentId := chi.URLParam(r, "commentId")

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	// Load comment scoped to current workspace.
	workspaceID := resolveWorkspaceID(r)
	comment, err := h.Queries.GetCommentInWorkspace(r.Context(), db.GetCommentInWorkspaceParams{
		ID:          parseUUID(commentId),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}

	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	isAuthor := comment.AuthorType == actorType && uuidToString(comment.AuthorID) == actorID
	isAdmin := roleAllowed(member.Role, "owner", "admin")
	if !isAuthor && !isAdmin {
		writeError(w, http.StatusForbidden, "only comment author or admin can delete")
		return
	}

	// Collect attachment URLs before CASCADE delete removes them.
	attachmentURLs, _ := h.Queries.ListAttachmentURLsByCommentID(r.Context(), parseUUID(commentId))

	if err := h.Queries.DeleteComment(r.Context(), parseUUID(commentId)); err != nil {
		slog.Warn("delete comment failed", append(logger.RequestAttrs(r), "error", err, "comment_id", commentId)...)
		writeError(w, http.StatusInternalServerError, "failed to delete comment")
		return
	}

	h.deleteS3Objects(r.Context(), attachmentURLs)
	slog.Info("comment deleted", append(logger.RequestAttrs(r), "comment_id", commentId, "issue_id", uuidToString(comment.IssueID))...)
	h.publish(protocol.EventCommentDeleted, workspaceID, actorType, actorID, map[string]any{
		"comment_id": commentId,
		"issue_id":   uuidToString(comment.IssueID),
	})
	w.WriteHeader(http.StatusNoContent)
}
