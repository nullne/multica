"use client";

import { useState } from "react";
import { Timer } from "lucide-react";
import { Calendar } from "@/components/ui/calendar";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import { Button } from "@/components/ui/button";
import { formatAbsoluteTime } from "@/shared/utils";

const DEFAULT_HOUR = 9;

function combineDateAndTime(date: Date, time: string): Date {
  const next = new Date(date);
  const [hours = "0", minutes = "0"] = time.split(":");
  next.setHours(parseInt(hours, 10), parseInt(minutes, 10), 0, 0);
  return next;
}

function toTimeInputValue(date: Date): string {
  const hh = String(date.getHours()).padStart(2, "0");
  const mm = String(date.getMinutes()).padStart(2, "0");
  return `${hh}:${mm}`;
}

// DispatchAfterPicker selects the time before which an agent assignment is
// not dispatched. Unlike DueDatePicker it needs minute precision, so it pairs
// the calendar with a time input.
export function DispatchAfterPicker({
  dispatchAfter,
  onChange,
  trigger: customTrigger,
  triggerRender,
}: {
  dispatchAfter: string | null;
  onChange: (value: string | null) => void;
  trigger?: React.ReactNode;
  triggerRender?: React.ReactElement;
}) {
  const [open, setOpen] = useState(false);
  const date = dispatchAfter ? new Date(dispatchAfter) : undefined;
  const isPending = date ? date > new Date() : false;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        className={triggerRender ? undefined : "flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors"}
        render={triggerRender}
      >
        {customTrigger ?? (
          <>
            <Timer className="h-3.5 w-3.5 text-muted-foreground" />
            {date ? (
              <span className={isPending ? "text-primary" : "text-muted-foreground"}>
                {formatAbsoluteTime(dispatchAfter, "shortDateTime")}
              </span>
            ) : (
              <span className="text-muted-foreground">Dispatch after</span>
            )}
          </>
        )}
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <Calendar
          mode="single"
          selected={date}
          onSelect={(d: Date | undefined) => {
            if (!d) return;
            // Keep the previously selected time when changing the date.
            const next = new Date(d);
            if (date) {
              next.setHours(date.getHours(), date.getMinutes(), 0, 0);
            } else {
              next.setHours(DEFAULT_HOUR, 0, 0, 0);
            }
            onChange(next.toISOString());
          }}
        />
        <div className="border-t px-3 py-2 flex items-center justify-between gap-2">
          <input
            type="time"
            value={date ? toTimeInputValue(date) : `${String(DEFAULT_HOUR).padStart(2, "0")}:00`}
            disabled={!date}
            onChange={(e) => {
              if (!date || !e.target.value) return;
              onChange(combineDateAndTime(date, e.target.value).toISOString());
            }}
            className="rounded border px-1.5 py-0.5 text-xs bg-transparent outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
          />
          {date && (
            <Button
              variant="ghost"
              size="xs"
              onClick={() => {
                onChange(null);
                setOpen(false);
              }}
              className="text-muted-foreground hover:text-foreground"
            >
              Clear
            </Button>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
