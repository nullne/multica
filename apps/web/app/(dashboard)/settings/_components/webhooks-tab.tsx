"use client";

import { Fragment, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ChevronRight, ExternalLink, Webhook as WebhookIcon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { AbsoluteTime } from "@/components/common/absolute-time";
import { cn } from "@/lib/utils";
import { api } from "@/shared/api";
import type {
  WebhookEvent,
  WebhookEventStatus,
  WebhookListItem,
} from "@/shared/types";

// Tailwind classes per event status, using shadcn design tokens.
const STATUS_STYLES: Record<WebhookEventStatus, string> = {
  processed: "bg-primary/10 text-primary border-primary/20",
  filtered: "bg-muted text-muted-foreground border-transparent",
  deduped: "bg-muted text-muted-foreground border-transparent",
  error: "bg-destructive/10 text-destructive border-destructive/20",
};

function StatusBadge({ status }: { status: WebhookEventStatus }) {
  return (
    <Badge variant="outline" className={cn("text-xs font-normal", STATUS_STYLES[status])}>
      {status}
    </Badge>
  );
}

export function WebhooksTab() {
  const [webhooks, setWebhooks] = useState<WebhookListItem[]>([]);
  const [events, setEvents] = useState<WebhookEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [hooks, evts] = await Promise.all([
          api.listWebhooks(),
          api.listWebhookEvents(),
        ]);
        if (cancelled) return;
        setWebhooks(hooks);
        setEvents(evts);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : "Failed to load webhook events");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // Resolve each event's originating webhook for display.
  const webhookById = useMemo(() => {
    const map = new Map<string, WebhookListItem["webhook"]>();
    for (const item of webhooks) map.set(item.webhook.id, item.webhook);
    return map;
  }, [webhooks]);

  const toggle = (id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <h2 className="text-sm font-semibold">Webhook events</h2>
        <p className="text-xs text-muted-foreground">
          Incoming events received from GitHub and custom webhooks, including their
          processing status and any issue they created.
        </p>
      </div>

      <Card>
        <CardContent className="p-0">
          {loading ? (
            <div className="space-y-2 p-4">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-8 w-full" />
              ))}
            </div>
          ) : error ? (
            <p className="p-6 text-sm text-destructive">{error}</p>
          ) : events.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-12 text-center">
              <WebhookIcon className="h-6 w-6 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">No webhook events yet.</p>
              <p className="text-xs text-muted-foreground">
                Events appear here once a configured webhook receives a request.
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-8" />
                  <TableHead>Source</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Issue</TableHead>
                  <TableHead className="text-right">Received</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {events.map((event) => {
                  const hook = webhookById.get(event.webhook_id);
                  const isOpen = expanded.has(event.id);
                  return (
                    <Fragment key={event.id}>
                      <TableRow
                        className="cursor-pointer"
                        onClick={() => toggle(event.id)}
                      >
                        <TableCell className="align-top">
                          <ChevronRight
                            className={cn(
                              "h-4 w-4 text-muted-foreground transition-transform",
                              isOpen && "rotate-90",
                            )}
                          />
                        </TableCell>
                        <TableCell className="align-top">
                          <div className="flex flex-col gap-1">
                            <span className="text-sm font-medium">
                              {hook?.name ?? "Unknown webhook"}
                            </span>
                            {hook && (
                              <Badge
                                variant="secondary"
                                className="w-fit text-xs font-normal"
                              >
                                {hook.source_type}
                              </Badge>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="align-top">
                          <StatusBadge status={event.status} />
                        </TableCell>
                        <TableCell className="align-top">
                          {event.issue_id ? (
                            <Link
                              href={`/issues/${event.issue_id}`}
                              onClick={(e) => e.stopPropagation()}
                              className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
                            >
                              View
                              <ExternalLink className="h-3 w-3" />
                            </Link>
                          ) : (
                            <span className="text-sm text-muted-foreground">—</span>
                          )}
                        </TableCell>
                        <TableCell className="text-right align-top text-sm text-muted-foreground">
                          <AbsoluteTime value={event.created_at} style="shortDateTime" />
                        </TableCell>
                      </TableRow>
                      {isOpen && (
                        <TableRow className="hover:bg-transparent">
                          <TableCell colSpan={5} className="bg-muted/30">
                            {event.error_message && (
                              <p className="mb-2 text-sm text-destructive">
                                {event.error_message}
                              </p>
                            )}
                            <pre className="max-h-80 overflow-auto rounded bg-background p-3 text-xs">
                              {JSON.stringify(event.payload, null, 2)}
                            </pre>
                          </TableCell>
                        </TableRow>
                      )}
                    </Fragment>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
