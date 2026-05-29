"use client";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import {
  formatAbsoluteTime,
  formatAbsoluteTimeTooltip,
  type AbsoluteTimeStyle,
} from "@/shared/utils";

interface AbsoluteTimeProps {
  value: string | null | undefined;
  style?: AbsoluteTimeStyle;
  fallback?: string;
  className?: string;
}

export function AbsoluteTime({
  value,
  style = "localeDateTime",
  fallback = "—",
  className,
}: AbsoluteTimeProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span className={cn("cursor-default", className)}>
            {formatAbsoluteTime(value, style, fallback)}
          </span>
        }
      />
      <TooltipContent side="top">
        {formatAbsoluteTimeTooltip(value, fallback)}
      </TooltipContent>
    </Tooltip>
  );
}
