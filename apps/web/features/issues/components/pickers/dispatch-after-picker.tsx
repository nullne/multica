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
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { formatAbsoluteTime, formatRelativeTime } from "@/shared/utils";

const DEFAULT_HOUR = 9;

const MINUTES_PER_UNIT = {
  minutes: 1,
  hours: 60,
  days: 60 * 24,
} as const;

type DurationUnit = keyof typeof MINUTES_PER_UNIT;

const RELATIVE_PRESETS: { label: string; amount: number; unit: DurationUnit }[] = [
  { label: "15 minutes", amount: 15, unit: "minutes" },
  { label: "30 minutes", amount: 30, unit: "minutes" },
  { label: "1 hour", amount: 1, unit: "hours" },
  { label: "3 hours", amount: 3, unit: "hours" },
  { label: "1 day", amount: 1, unit: "days" },
  { label: "3 days", amount: 3, unit: "days" },
];

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
// not dispatched. It offers a relative "in N minutes/hours/days" tab (default)
// and an absolute calendar + time tab. Both tabs stage a pending value and
// only commit it when the Set button is pressed.
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
  const [tab, setTab] = useState<"relative" | "absolute">("relative");
  const [customAmount, setCustomAmount] = useState("30");
  const [customUnit, setCustomUnit] = useState<DurationUnit>("minutes");
  // Pending absolute selection — committed via the Set button.
  const [pendingDate, setPendingDate] = useState<Date | undefined>(undefined);
  const date = dispatchAfter ? new Date(dispatchAfter) : undefined;
  const isPending = date ? date > new Date() : false;

  const customMinutes =
    Number.parseFloat(customAmount) * MINUTES_PER_UNIT[customUnit];
  const customValid = Number.isFinite(customMinutes) && customMinutes > 0;

  const commitRelative = () => {
    if (!customValid) return;
    onChange(new Date(Date.now() + customMinutes * 60_000).toISOString());
    setOpen(false);
  };

  // A pending absolute time must be in the future to be committable.
  const pendingInFuture = pendingDate ? pendingDate > new Date() : false;

  const commitAbsolute = () => {
    if (!pendingDate || !pendingInFuture) return;
    onChange(pendingDate.toISOString());
    setOpen(false);
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) {
          // Stage the current value when the popover opens. A stored value is
          // an absolute timestamp, so land on the tab that can display it.
          setPendingDate(date);
          setTab(date ? "absolute" : "relative");
        }
      }}
    >
      <PopoverTrigger
        className={triggerRender ? undefined : "flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors"}
        render={triggerRender}
      >
        {customTrigger ?? (
          <>
            <Timer className="h-3.5 w-3.5 text-muted-foreground" />
            {date && dispatchAfter ? (
              <span className={isPending ? "text-primary" : "text-muted-foreground"}>
                {formatAbsoluteTime(dispatchAfter, "shortDateTime")}
                <span className="text-muted-foreground"> ({formatRelativeTime(dispatchAfter)})</span>
              </span>
            ) : (
              <span className="text-muted-foreground">Dispatch after</span>
            )}
          </>
        )}
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <Tabs
          value={tab}
          onValueChange={(value) => setTab(value as "relative" | "absolute")}
        >
          <div className="border-b px-2 pt-2 pb-1.5">
            <TabsList className="w-full">
              <TabsTrigger value="relative" className="text-xs">
                In…
              </TabsTrigger>
              <TabsTrigger value="absolute" className="text-xs">
                At time
              </TabsTrigger>
            </TabsList>
          </div>
          <TabsContent value="relative" className="w-60 p-2">
            <div className="grid grid-cols-2 gap-1">
              {RELATIVE_PRESETS.map((preset) => {
                const selected =
                  customUnit === preset.unit &&
                  Number.parseFloat(customAmount) === preset.amount;
                return (
                  <Button
                    key={preset.label}
                    variant={selected ? "secondary" : "ghost"}
                    size="xs"
                    onClick={() => {
                      setCustomAmount(String(preset.amount));
                      setCustomUnit(preset.unit);
                    }}
                    className="justify-start font-normal"
                  >
                    {preset.label}
                  </Button>
                );
              })}
            </div>
            <div className="mt-2 flex items-center gap-1.5 border-t pt-2">
              <Input
                type="number"
                min={1}
                value={customAmount}
                onChange={(e) => setCustomAmount(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") commitRelative();
                }}
                className="h-7 w-16 px-2 text-xs"
                aria-label="Duration amount"
              />
              <NativeSelect
                size="sm"
                className="flex-1"
                value={customUnit}
                onChange={(e) => setCustomUnit(e.target.value as DurationUnit)}
                aria-label="Duration unit"
              >
                <NativeSelectOption value="minutes">minutes</NativeSelectOption>
                <NativeSelectOption value="hours">hours</NativeSelectOption>
                <NativeSelectOption value="days">days</NativeSelectOption>
              </NativeSelect>
              <Button size="xs" disabled={!customValid} onClick={commitRelative}>
                Set
              </Button>
            </div>
          </TabsContent>
          <TabsContent value="absolute">
            <Calendar
              mode="single"
              selected={pendingDate}
              disabled={{ before: new Date() }}
              onSelect={(d: Date | undefined) => {
                if (!d) return;
                // Keep the previously selected time when changing the date.
                const next = new Date(d);
                if (pendingDate) {
                  next.setHours(pendingDate.getHours(), pendingDate.getMinutes(), 0, 0);
                } else {
                  next.setHours(DEFAULT_HOUR, 0, 0, 0);
                }
                setPendingDate(next);
              }}
            />
            <div className="border-t px-3 py-2 flex items-center gap-1.5">
              <input
                type="time"
                value={pendingDate ? toTimeInputValue(pendingDate) : `${String(DEFAULT_HOUR).padStart(2, "0")}:00`}
                disabled={!pendingDate}
                onChange={(e) => {
                  if (!pendingDate || !e.target.value) return;
                  setPendingDate(combineDateAndTime(pendingDate, e.target.value));
                }}
                className="rounded border px-1.5 py-0.5 text-xs bg-transparent outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
              />
              <Button
                size="xs"
                disabled={!pendingInFuture}
                onClick={commitAbsolute}
                className="ml-auto"
              >
                Set
              </Button>
            </div>
          </TabsContent>
        </Tabs>
        {date && (
          <div className="border-t px-2 py-1.5 flex items-center justify-between gap-2">
            <span className="text-xs text-muted-foreground truncate">
              {formatAbsoluteTime(dispatchAfter, "shortDateTime")}
            </span>
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
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}
