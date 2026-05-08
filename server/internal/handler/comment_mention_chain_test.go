package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	dbq "github.com/nullne/multica/server/pkg/db/generated"
)

// agentMentionTriggerJSON is the JSON snippet enabling on_mention.
const agentMentionTriggerJSON = `[{"type":"on_mention","enabled":true}]`

// postAgentMention posts a comment authored by speakerAgentID that mentions
// targetAgentID. Returns the recorder for assertions.
func postAgentMention(t *testing.T, issueID, speakerAgentID, targetAgentID string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@target](mention://agent/%s) please continue", targetAgentID),
	})
	req.Header.Set("X-Agent-ID", speakerAgentID)
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment (agent author): expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return w
}

// readAgentMentionChain returns the persisted chain count for an issue.
func readAgentMentionChain(t *testing.T, issueID string) int {
	t.Helper()
	var count int
	err := testPool.QueryRow(context.Background(),
		`SELECT agent_mention_chain_count FROM issue WHERE id = $1`, issueID).Scan(&count)
	if err != nil {
		t.Fatalf("readAgentMentionChain: %v", err)
	}
	return count
}

// pendingTaskCountForAgent returns the number of queued/dispatched/running
// tasks for an agent on an issue.
func pendingTaskCountForAgent(t *testing.T, issueID, agentID string) int {
	t.Helper()
	var count int
	err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM agent_task_queue
		 WHERE issue_id = $1 AND agent_id = $2 AND status IN ('queued','dispatched','running')`,
		issueID, agentID).Scan(&count)
	if err != nil {
		t.Fatalf("pendingTaskCountForAgent: %v", err)
	}
	return count
}

// clearPendingTasksForIssue marks any pending tasks on the issue as completed.
// This simulates each handoff finishing before the next one starts, which the
// idx_one_pending_task_per_issue partial unique index requires.
func clearPendingTasksForIssue(t *testing.T, issueID string) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(),
		`UPDATE agent_task_queue SET status = 'completed', completed_at = now()
		 WHERE issue_id = $1 AND status IN ('queued','dispatched','running')`,
		issueID); err != nil {
		t.Fatalf("clearPendingTasksForIssue: %v", err)
	}
}

// setIssueAssignee assigns an agent to an issue directly via SQL.
func setIssueAssignee(t *testing.T, issueID, agentID string) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(),
		`UPDATE issue SET assignee_type = 'agent', assignee_id = $2 WHERE id = $1`,
		issueID, agentID); err != nil {
		t.Fatalf("setIssueAssignee: %v", err)
	}
}

// TestMentionDispatch_AgentMentionsAssignee_Enqueued verifies that an
// agent-authored comment can mention the issue assignee and trigger an
// on_mention task — the previous behavior silently skipped this case.
func TestMentionDispatch_AgentMentionsAssignee_Enqueued(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: agent->assignee handoff")

	assigneeID := createTestAgent(t, "chain-assignee", []string{"codex"}, false, agentMentionTriggerJSON)
	speakerID := createTestAgent(t, "chain-speaker", []string{"codex"}, false, agentMentionTriggerJSON)
	setIssueAssignee(t, issueID, assigneeID)

	postAgentMention(t, issueID, speakerID, assigneeID)

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) != 0 {
		t.Fatalf("expected no system notices for agent->assignee handoff, got %d: %v", len(notices), notices)
	}
	if got := pendingTaskCountForAgent(t, issueID, assigneeID); got != 1 {
		t.Fatalf("expected 1 pending task for assignee after agent mention, got %d", got)
	}
	if got := readAgentMentionChain(t, issueID); got != 1 {
		t.Fatalf("expected chain count 1 after one agent->agent handoff, got %d", got)
	}
}

// TestMentionDispatch_MemberMentionsAssignee_NoDuplicateNotice verifies that
// when a member mentions the assignee agent, the on_comment trigger handles
// the dispatch and the mention loop does not post a spurious "already pending"
// notice for the same comment.
func TestMentionDispatch_MemberMentionsAssignee_NoDuplicateNotice(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: member->assignee no duplicate")

	assigneeID := createTestAgent(t, "chain-member-assignee",
		[]string{"codex"}, false,
		`[{"type":"on_comment","enabled":true},{"type":"on_mention","enabled":true}]`)
	setIssueAssignee(t, issueID, assigneeID)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@assignee](mention://agent/%s) please help", assigneeID),
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) != 0 {
		t.Fatalf("expected no system notices for member mentioning assignee, got %d: %v", len(notices), notices)
	}
	if got := pendingTaskCountForAgent(t, issueID, assigneeID); got != 1 {
		t.Fatalf("expected 1 pending task for assignee, got %d", got)
	}
}

// TestMentionDispatch_MemberMention_DoesNotIncrementChain verifies that a
// member-authored comment with an agent mention does not increment the
// agent-to-agent chain counter.
func TestMentionDispatch_MemberMention_DoesNotIncrementChain(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: member mention chain")

	agentID := createTestAgent(t, "chain-no-increment", []string{"codex"}, false, agentMentionTriggerJSON)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@agent](mention://agent/%s) please help", agentID),
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if got := readAgentMentionChain(t, issueID); got != 0 {
		t.Fatalf("expected chain count 0 after member-authored mention, got %d", got)
	}
}

// TestMentionDispatch_AgentChainLimit_BlocksAfterThreshold drives the chain
// counter up to AgentMentionChainLimit and verifies the next agent-authored
// mention is blocked with a "limit reached" system notice.
func TestMentionDispatch_AgentChainLimit_BlocksAfterThreshold(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: agent chain limit")

	agents := make([]string, AgentMentionChainLimit+2)
	for i := range agents {
		agents[i] = createTestAgent(t,
			fmt.Sprintf("chain-link-%d", i),
			[]string{"codex"}, false, agentMentionTriggerJSON,
		)
	}

	for i := 0; i < AgentMentionChainLimit; i++ {
		postAgentMention(t, issueID, agents[i], agents[i+1])
		// Each enqueued task must complete before the next handoff can be
		// enqueued — the database enforces one pending task per issue.
		clearPendingTasksForIssue(t, issueID)
	}
	if got := readAgentMentionChain(t, issueID); got != AgentMentionChainLimit {
		t.Fatalf("expected chain count %d after %d handoffs, got %d",
			AgentMentionChainLimit, AgentMentionChainLimit, got)
	}
	noticesBefore := listSystemNoticesForIssue(t, issueID)
	if len(noticesBefore) != 0 {
		t.Fatalf("expected no system notices during chain build-up, got %d: %v",
			len(noticesBefore), noticesBefore)
	}

	// One more agent-authored mention should be blocked.
	postAgentMention(t, issueID, agents[0], agents[AgentMentionChainLimit+1])

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) == 0 {
		t.Fatal("expected a system notice when chain limit is reached, got none")
	}
	found := false
	for _, n := range notices {
		if containsAll(n, "chain limit reached") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected notice about chain limit, got: %v", notices)
	}
	if got := pendingTaskCountForAgent(t, issueID, agents[AgentMentionChainLimit+1]); got != 0 {
		t.Fatalf("expected no pending task for the blocked target, got %d", got)
	}
}

// TestMentionDispatch_MemberCommentResetsChain verifies that a member comment
// resets the agent-to-agent chain counter, allowing dispatch to resume.
func TestMentionDispatch_MemberCommentResetsChain(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: member resets chain")

	agents := make([]string, AgentMentionChainLimit+2)
	for i := range agents {
		agents[i] = createTestAgent(t,
			fmt.Sprintf("chain-reset-%d", i),
			[]string{"codex"}, false, agentMentionTriggerJSON,
		)
	}
	for i := 0; i < AgentMentionChainLimit; i++ {
		postAgentMention(t, issueID, agents[i], agents[i+1])
		clearPendingTasksForIssue(t, issueID)
	}
	if got := readAgentMentionChain(t, issueID); got != AgentMentionChainLimit {
		t.Fatalf("expected chain count %d, got %d", AgentMentionChainLimit, got)
	}

	// Plain human member comment — should reset chain.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "checking in, please continue when ready",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment (member reset): expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if got := readAgentMentionChain(t, issueID); got != 0 {
		t.Fatalf("expected chain count to reset to 0 after member comment, got %d", got)
	}

	// Fresh agent->agent handoff after reset must succeed without limit notice.
	startNotices := len(listSystemNoticesForIssue(t, issueID))
	postAgentMention(t, issueID, agents[0], agents[AgentMentionChainLimit+1])

	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) > startNotices {
		t.Fatalf("expected no new system notices after reset, got: %v", notices[startNotices:])
	}
	if got := readAgentMentionChain(t, issueID); got != 1 {
		t.Fatalf("expected chain count 1 after one handoff post-reset, got %d", got)
	}
}

// TestMentionDispatch_AgentSelfMention_StillSkipped verifies the existing
// self-mention safeguard remains in place after the chain-limit changes.
func TestMentionDispatch_AgentSelfMention_StillSkipped(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: agent self mention")

	agentID := createTestAgent(t, "chain-self", []string{"codex"}, false, agentMentionTriggerJSON)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": fmt.Sprintf("[@self](mention://agent/%s) i'm just thinking out loud", agentID),
	})
	req.Header.Set("X-Agent-ID", agentID)
	req = withURLParam(req, "id", issueID)
	testHandler.CreateComment(w, req)
	if w.Code != 201 {
		t.Fatalf("CreateComment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if got := pendingTaskCountForAgent(t, issueID, agentID); got != 0 {
		t.Fatalf("expected no pending task from self-mention, got %d", got)
	}
	if got := readAgentMentionChain(t, issueID); got != 0 {
		t.Fatalf("expected chain count 0 after self-mention, got %d", got)
	}
	if notices := listSystemNoticesForIssue(t, issueID); len(notices) != 0 {
		t.Fatalf("expected no system notices for self-mention, got %d: %v", len(notices), notices)
	}
}

// TestMentionDispatch_ChainSlotReleasedOnEnqueueFailure verifies that when an
// agent-authored mention reserves a chain slot but the underlying task enqueue
// fails (e.g. no runtime is available for the target's providers), the slot is
// released so a subsequent retry has the same budget.
func TestMentionDispatch_ChainSlotReleasedOnEnqueueFailure(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: slot release on failure")

	speakerID := createTestAgent(t, "chain-release-speaker",
		[]string{"codex"}, false, agentMentionTriggerJSON)
	// Provider has no online runtime in the test fixture, so EnqueueTaskForMention
	// will fail with "no online runtime for providers".
	targetID := createTestAgent(t, "chain-release-target",
		[]string{"claude"}, false, agentMentionTriggerJSON)

	postAgentMention(t, issueID, speakerID, targetID)

	if got := readAgentMentionChain(t, issueID); got != 0 {
		t.Fatalf("expected chain count 0 after failed enqueue (slot released), got %d", got)
	}
	notices := listSystemNoticesForIssue(t, issueID)
	if len(notices) == 0 {
		t.Fatal("expected runtime-unavailable system notice, got none")
	}
}

// TestTryReserveIssueAgentMentionChain_Concurrent drives the atomic reservation
// query from many goroutines at once and verifies that the chain count never
// exceeds the limit. This guards against the TOCTOU race that an earlier
// non-atomic implementation suffered from.
func TestTryReserveIssueAgentMentionChain_Concurrent(t *testing.T) {
	issueID := createTestIssue(t, "Mention dispatch: concurrent reservation")

	const limit = 5
	const attempts = 32

	var wg sync.WaitGroup
	successCh := make(chan struct{}, attempts)
	failureCh := make(chan struct{}, attempts)

	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func() {
			defer wg.Done()
			_, err := testHandler.Queries.TryReserveIssueAgentMentionChain(
				context.Background(),
				dbq.TryReserveIssueAgentMentionChainParams{
					ID:       parseUUID(issueID),
					MaxCount: limit,
				},
			)
			if err == nil {
				successCh <- struct{}{}
				return
			}
			if errors.Is(err, pgx.ErrNoRows) {
				failureCh <- struct{}{}
				return
			}
			t.Errorf("unexpected reservation error: %v", err)
		}()
	}
	wg.Wait()
	close(successCh)
	close(failureCh)

	successes := len(successCh)
	failures := len(failureCh)
	if successes != limit {
		t.Fatalf("expected exactly %d successful reservations, got %d (failures=%d)",
			limit, successes, failures)
	}
	if failures != attempts-limit {
		t.Fatalf("expected %d limit-rejections, got %d", attempts-limit, failures)
	}
	if got := readAgentMentionChain(t, issueID); got != limit {
		t.Fatalf("expected final chain count %d, got %d", limit, got)
	}
}
