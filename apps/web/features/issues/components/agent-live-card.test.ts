import { describe, it, expect } from "vitest";
import type { AgentTask } from "@/shared/types/agent";
import {
  formatTaskRunModelLabel,
  groupTaskRunsByTrigger,
  isActiveTask,
  partitionTaskRunsByResultComment,
} from "./agent-live-card";

function makeTask(overrides: Partial<AgentTask>): AgentTask {
  return {
    id: "task-x",
    agent_id: "agent-1",
    runtime_id: "rt-1",
    issue_id: "issue-1",
    status: "completed",
    priority: 0,
    dispatched_at: null,
    started_at: null,
    completed_at: null,
    result: null,
    error: null,
    created_at: "2026-05-01T00:00:00Z",
    trigger_comment_id: null,
    provider: null,
    model: null,
    thinking_level: null,
    ...overrides,
  };
}

describe("formatTaskRunModelLabel", () => {
  it("joins model and thinking level when both are present", () => {
    expect(formatTaskRunModelLabel(makeTask({ model: "gpt-5.5", thinking_level: "high" }))).toBe("gpt-5.5 · high");
  });

  it("omits empty task model metadata", () => {
    expect(formatTaskRunModelLabel(makeTask({ model: null, thinking_level: null }))).toBeNull();
  });
});

describe("groupTaskRunsByTrigger", () => {
  it("places null-trigger runs in the issue bucket", () => {
    const tasks = [
      makeTask({ id: "t1", trigger_comment_id: null }),
      makeTask({ id: "t2", trigger_comment_id: undefined }),
    ];
    const { issueTriggered, byCommentId } = groupTaskRunsByTrigger(tasks);
    expect(issueTriggered.map((t) => t.id)).toEqual(["t1", "t2"]);
    expect(byCommentId.size).toBe(0);
  });

  it("buckets comment-triggered runs by trigger_comment_id", () => {
    const tasks = [
      makeTask({ id: "t1", trigger_comment_id: null, created_at: "2026-05-01T00:00:00Z" }),
      makeTask({ id: "t2", trigger_comment_id: "c-1", created_at: "2026-05-02T00:00:00Z" }),
      makeTask({ id: "t3", trigger_comment_id: "c-1", created_at: "2026-05-03T00:00:00Z" }),
      makeTask({ id: "t4", trigger_comment_id: "c-2", created_at: "2026-05-04T00:00:00Z" }),
    ];
    const { issueTriggered, byCommentId } = groupTaskRunsByTrigger(tasks);
    expect(issueTriggered.map((t) => t.id)).toEqual(["t1"]);
    expect(byCommentId.get("c-1")?.map((t) => t.id)).toEqual(["t2", "t3"]);
    expect(byCommentId.get("c-2")?.map((t) => t.id)).toEqual(["t4"]);
  });

  it("sorts each bucket chronologically (oldest first)", () => {
    const tasks = [
      makeTask({ id: "later", created_at: "2026-05-05T00:00:00Z" }),
      makeTask({ id: "earlier", created_at: "2026-05-01T00:00:00Z" }),
    ];
    const { issueTriggered } = groupTaskRunsByTrigger(tasks);
    expect(issueTriggered.map((t) => t.id)).toEqual(["earlier", "later"]);
  });
});

describe("isActiveTask", () => {
  it("returns true for queued/dispatched/running", () => {
    expect(isActiveTask(makeTask({ status: "queued" }))).toBe(true);
    expect(isActiveTask(makeTask({ status: "dispatched" }))).toBe(true);
    expect(isActiveTask(makeTask({ status: "running" }))).toBe(true);
  });

  it("returns false for terminal states", () => {
    expect(isActiveTask(makeTask({ status: "completed" }))).toBe(false);
    expect(isActiveTask(makeTask({ status: "failed" }))).toBe(false);
    expect(isActiveTask(makeTask({ status: "cancelled" }))).toBe(false);
  });
});

describe("partitionTaskRunsByResultComment", () => {
  it("sends tasks without a result_comment_id to the standalone bucket", () => {
    const tasks = [
      makeTask({ id: "running", status: "running", result_comment_id: null }),
      makeTask({ id: "no-reply", status: "completed", result_comment_id: undefined }),
      makeTask({ id: "failed", status: "failed", result_comment_id: null }),
    ];
    const { embedded, standalone } = partitionTaskRunsByResultComment(tasks);
    expect(embedded.size).toBe(0);
    expect(standalone.map((t) => t.id)).toEqual(["running", "no-reply", "failed"]);
  });

  it("embeds tasks with a result_comment_id under that comment id", () => {
    const tasks = [
      makeTask({ id: "t1", status: "completed", result_comment_id: "reply-a" }),
      makeTask({ id: "t2", status: "completed", result_comment_id: "reply-b" }),
    ];
    const { embedded, standalone } = partitionTaskRunsByResultComment(tasks);
    expect(standalone).toHaveLength(0);
    expect(embedded.get("reply-a")?.map((t) => t.id)).toEqual(["t1"]);
    expect(embedded.get("reply-b")?.map((t) => t.id)).toEqual(["t2"]);
  });

  it("groups multiple runs that share one reply chronologically", () => {
    const tasks = [
      makeTask({ id: "third", created_at: "2026-05-03T00:00:00Z", result_comment_id: "reply-x" }),
      makeTask({ id: "first", created_at: "2026-05-01T00:00:00Z", result_comment_id: "reply-x" }),
      makeTask({ id: "second", created_at: "2026-05-02T00:00:00Z", result_comment_id: "reply-x" }),
    ];
    const { embedded } = partitionTaskRunsByResultComment(tasks);
    expect(embedded.get("reply-x")?.map((t) => t.id)).toEqual(["first", "second", "third"]);
  });

  it("mixes the two buckets cleanly when some runs have a reply and others don't", () => {
    const tasks = [
      makeTask({ id: "active", status: "running", result_comment_id: null }),
      makeTask({ id: "done", status: "completed", result_comment_id: "reply-y" }),
      makeTask({ id: "orphan", status: "failed", result_comment_id: null }),
    ];
    const { embedded, standalone } = partitionTaskRunsByResultComment(tasks);
    expect(embedded.get("reply-y")?.map((t) => t.id)).toEqual(["done"]);
    expect(standalone.map((t) => t.id).sort()).toEqual(["active", "orphan"]);
  });
});
