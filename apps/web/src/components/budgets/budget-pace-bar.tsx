"use client";

import type { ReactNode } from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { BudgetStatus } from "@/lib/budget-pace";

const STATUS_FILL: Record<BudgetStatus, string> = {
  on_track: "var(--primary)",
  close: "var(--warning)",
  over: "var(--negative)",
};

/**
 * Budget progress bar with a pace marker: a thin tick where an even spend
 * would be by now. The fill is capped at 100%; `color` overrides the
 * on-track fill (e.g. the category colour).
 *
 * When spending runs ahead of an even pace (on-track budgets only), the part
 * of the fill past the marker turns amber, so the bar says it without words.
 * `markerLabel` prints a one-word caption under the marker.
 *
 * With `tip`, hovering or focusing the bar shows it in a tooltip. The trigger
 * is lifted above a card's stretched click target (budget cards make the whole
 * card a button) and has a taller invisible hit area, so it is easy to reach.
 */
export function BudgetPaceBar({
  percentage,
  elapsed,
  status,
  color,
  thick,
  label,
  tip,
  markerLabel,
  className,
}: {
  /** spent / budgeted * 100, may exceed 100. */
  percentage: number;
  /** Fraction of the period elapsed, 0 to 1; omit to hide the marker. */
  elapsed?: number;
  status: BudgetStatus;
  color?: string;
  thick?: boolean;
  /** Accessible name; the bar is announced as a progress bar. */
  label: string;
  /** Tooltip content explaining the bar and its pace marker. */
  tip?: ReactNode;
  /** A short caption under the marker (e.g. "Today"). */
  markerLabel?: string;
  className?: string;
}) {
  const width = Math.min(Math.max(percentage, 0), 100);
  const base = status === "on_track" && color ? color : STATUS_FILL[status];
  const markerPct =
    elapsed === undefined ? null : Math.min(Math.max(elapsed, 0), 1) * 100;
  // Ahead of an even pace (by more than 2 points): colour the overshoot.
  const ahead =
    status === "on_track" && markerPct !== null && width > markerPct + 2;
  const stop = ahead && markerPct !== null ? (markerPct / width) * 100 : 0;
  const fillStyle = ahead
    ? {
        width: `${width}%`,
        background: `linear-gradient(to right, ${base} ${stop}%, var(--warning) ${stop}%)`,
      }
    : { width: `${width}%`, backgroundColor: base };
  const bar = (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(width)}
      className={cn(
        "bg-muted relative w-full rounded-full",
        thick ? "h-2.5" : "h-1.5",
        className
      )}
    >
      <div
        className="h-full rounded-full transition-[width]"
        style={fillStyle}
      />
      {elapsed !== undefined && (
        <span
          aria-hidden
          className="bg-foreground/70 absolute top-[-3px] bottom-[-3px] w-0.5 rounded-full"
          style={{ left: `calc(${markerPct}% - 1px)` }}
        />
      )}
      {markerLabel && markerPct !== null && (
        <span
          aria-hidden
          className="text-muted-foreground absolute top-full mt-1.5 -translate-x-1/2 text-[10px] leading-none whitespace-nowrap"
          style={{ left: `${markerPct}%` }}
        >
          {markerLabel}
        </span>
      )}
    </div>
  );

  if (!tip) return bar;
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <div
            tabIndex={0}
            className="focus-visible:ring-ring/50 relative z-10 -my-2 cursor-help rounded-md py-2 outline-none focus-visible:ring-[3px]"
          >
            {bar}
          </div>
        </TooltipTrigger>
        <TooltipContent side="bottom" className="max-w-64 text-left">
          {tip}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
