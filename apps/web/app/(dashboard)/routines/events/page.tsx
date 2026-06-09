"use client";

import { useCallback, useEffect, useState } from "react";
import { ChevronLeft, Copy, History, Loader2 } from "lucide-react";
import { AbsoluteTime } from "@/components/common/absolute-time";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { api } from "@/shared/api";
import type { RoutineEvent } from "@/shared/types";
import { toast } from "sonner";

const PAGE_SIZE = 20;

const STATUS_FILTERS = [
  { value: "", label: "All statuses" },
  { value: "received", label: "Received" },
  { value: "processed", label: "Processed" },
  { value: "filtered", label: "Filtered" },
  { value: "deduped", label: "Deduped" },
  { value: "no_matching_trigger", label: "No matching trigger" },
  { value: "parse_error", label: "Parse error" },
  { value: "error", label: "Error" },
];

const SOURCE_FILTERS = [
  { value: "", label: "All sources" },
  { value: "github", label: "GitHub" },
  { value: "api", label: "API" },
  { value: "schedule", label: "Schedule" },
  { value: "alert", label: "Alert" },
];

export default function RoutineEventsPage() {
  const [events, setEvents] = useState<RoutineEvent[]>([]);
  const [nextOffset, setNextOffset] = useState(0);
  const [loadingInitial, setLoadingInitial] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [statusFilter, setStatusFilter] = useState("");
  const [sourceFilter, setSourceFilter] = useState("");
  const [eventTypeFilter, setEventTypeFilter] = useState("");

  const loadPage = useCallback(async (offset: number) => {
    const isInitial = offset === 0;
    if (isInitial) {
      setLoadingInitial(true);
    } else {
      setLoadingMore(true);
    }
    try {
      const page = await api.listRoutineEvents({
        limit: PAGE_SIZE,
        offset,
        status: statusFilter || undefined,
        source_type: sourceFilter || undefined,
        event_type: eventTypeFilter.trim() || undefined,
      });
      setEvents((current) => (isInitial ? page : [...current, ...page]));
      setNextOffset(offset + PAGE_SIZE);
      setHasMore(page.length === PAGE_SIZE);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to load routine events");
    } finally {
      if (isInitial) {
        setLoadingInitial(false);
      } else {
        setLoadingMore(false);
      }
    }
  }, [eventTypeFilter, sourceFilter, statusFilter]);

  useEffect(() => {
    void loadPage(0);
  }, [loadPage]);

  const handleScroll = useCallback(
    (event: React.UIEvent<HTMLDivElement>) => {
      if (loadingInitial || loadingMore || !hasMore) return;
      const target = event.currentTarget;
      const distanceFromBottom = target.scrollHeight - target.scrollTop - target.clientHeight;
      if (distanceFromBottom < 160) {
        void loadPage(nextOffset);
      }
    },
    [hasMore, loadPage, loadingInitial, loadingMore, nextOffset],
  );

  return (
    <main
      data-testid="routine-events-scroll"
      onScroll={handleScroll}
      className="h-full min-h-0 overflow-y-auto bg-background"
    >
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-5 px-6 py-5">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <History className="size-4" />
              <a href="/routines" className="hover:text-foreground">Routines</a>
              <span>/</span>
              <span>Events</span>
            </div>
            <h1 className="text-2xl font-semibold tracking-tight">Routine events</h1>
            <p className="text-sm text-muted-foreground">
              Recent routine event log entries across this workspace.
            </p>
          </div>
          <a href="/routines" className={buttonVariants({ variant: "outline" })}>
            <ChevronLeft className="size-4" />
            Back to routines
          </a>
        </div>

        <section className="grid gap-3 rounded-xl border bg-muted/20 p-4 md:grid-cols-[1fr_1fr_2fr]">
          <div className="grid gap-1.5">
            <Label htmlFor="routine-event-status-filter">Status</Label>
            <NativeSelect
              id="routine-event-status-filter"
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value)}
            >
              {STATUS_FILTERS.map((option) => (
                <NativeSelectOption key={option.value || "all"} value={option.value}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="routine-event-source-filter">Source</Label>
            <NativeSelect
              id="routine-event-source-filter"
              value={sourceFilter}
              onChange={(event) => setSourceFilter(event.target.value)}
            >
              {SOURCE_FILTERS.map((option) => (
                <NativeSelectOption key={option.value || "all"} value={option.value}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="routine-event-type-filter">Event type</Label>
            <Input
              id="routine-event-type-filter"
              value={eventTypeFilter}
              onChange={(event) => setEventTypeFilter(event.target.value)}
              placeholder="github.pull_request.opened"
            />
          </div>
        </section>

        {loadingInitial ? (
          <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
            Loading routine events...
          </div>
        ) : events.length === 0 ? (
          <div className="rounded-xl border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
            No routine events yet
          </div>
        ) : (
          <section className="space-y-2">
            {events.map((event) => (
              <RoutineEventRow key={event.id} event={event} />
            ))}
          </section>
        )}

        {!loadingInitial && (
          <div className="flex justify-center py-4 text-sm text-muted-foreground">
            {loadingMore ? (
              <span className="inline-flex items-center gap-2">
                <Loader2 className="size-4 animate-spin" />
                Loading more events...
              </span>
            ) : hasMore ? (
              "Scroll to load more"
            ) : (
              "No more events"
            )}
          </div>
        )}
      </div>
    </main>
  );
}

function RoutineEventRow({ event }: { event: RoutineEvent }) {
  const [isOpen, setIsOpen] = useState(false);
  const body = formatEventBody(event.payload, event.data);
  const copyPayload = () => {
    void window.navigator.clipboard.writeText(body).then(() => {
      toast.success("Payload copied");
    });
  };
  return (
    <article
      className="cursor-pointer rounded-xl border bg-muted/20 px-4 py-3 transition-colors hover:bg-muted/30"
      role="button"
      tabIndex={0}
      aria-expanded={isOpen}
      onClick={() => {
        if (hasSelectedText()) return;
        setIsOpen((open) => !open);
      }}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          setIsOpen((open) => !open);
        }
      }}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <Badge variant="secondary">{event.source_type || "routine"}</Badge>
            <span className="truncate text-sm font-medium">{event.event_type || "routine"}</span>
          </div>
          <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
            <AbsoluteTime value={event.created_at} style="shortDateTime" />
            {event.dedup_key && <span className="truncate">Dedup: {event.dedup_key}</span>}
            {event.external_delivery_id && <span className="truncate">Delivery: {event.external_delivery_id}</span>}
          </div>
          {event.error_message && (
            <p className="text-sm text-destructive">{event.error_message}</p>
          )}
        </div>
        <Badge variant={event.status === "error" || event.status === "parse_error" ? "destructive" : "outline"}>
          {event.status}
        </Badge>
      </div>
      {isOpen && (
        <div className="mt-3">
          <div className="mb-2 flex justify-end">
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              copyPayload();
            }}
            aria-label="Copy payload preview"
              className="inline-flex h-6 items-center gap-1 rounded-md px-2 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          >
            <Copy className="size-3.5" />
            Copy
          </button>
          </div>
          <div className="overflow-hidden rounded-lg bg-background">
          <pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words p-3 text-xs text-muted-foreground">
            {body}
          </pre>
        </div>
        </div>
      )}
    </article>
  );
}

function hasSelectedText() {
  return (window.getSelection()?.toString().trim().length ?? 0) > 0;
}

function formatEventBody(payload: unknown, data: unknown) {
  return JSON.stringify({ payload, data }, null, 2);
}
