"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { useActiveTaskStore } from "@/features/issues/stores/active-task-store";

interface RunningIndicatorRingProps {
  /** Issue ID — when this issue has an active agent task, the ring is shown. */
  issueId: string;
  /** The original icon/avatar to wrap. Rendered untouched when no task is active. */
  children: ReactNode;
  /** Extra classes on the wrapping span. */
  className?: string;
}

/**
 * Wraps an icon (priority, status, avatar, ...) with a subtle spinning outer
 * ring while an agent task is active for the given issue. The wrapped icon
 * is never altered, only the surrounding ring is animated, so the row's
 * visual identity stays stable.
 */
export function RunningIndicatorRing({
  issueId,
  children,
  className,
}: RunningIndicatorRingProps) {
  const isWorking = useActiveTaskStore((s) => s.tasks.has(issueId));

  return (
    <span
      data-running={isWorking || undefined}
      className={cn(
        "relative inline-flex shrink-0 items-center justify-center",
        className,
      )}
    >
      {children}
      {isWorking && (
        <span
          aria-hidden="true"
          title="Agent is working"
          className="pointer-events-none absolute -inset-[3px] rounded-full border border-info/25 border-t-info animate-spin"
        />
      )}
    </span>
  );
}
