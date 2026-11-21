"use client";

import { Info } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  waivedFeeLabel,
  waiverFraction,
  waiverStatusLabel,
  windowElapsedFraction,
} from "@/lib/card-fee";
import { formatMoney } from "@/lib/format";
import type {
  CardFeeSummary,
  FeeWaiverStatus,
  FeeWaiverTarget,
} from "@/types/models";

const STATUS_TONE: Record<FeeWaiverStatus, "pos" | "accent" | "neg"> = {
  met: "pos",
  on_track: "accent",
  behind: "neg",
};

const STATUS_FILL: Record<FeeWaiverStatus, string> = {
  met: "var(--positive)",
  on_track: "var(--primary)",
  behind: "var(--negative)",
};

const DATA_NOTE =
  "Based on the transactions recorded on this card. Banks often exclude some spending (e-wallet top-ups, insurance, government payments).";

/** One target's bar, with a tick where a steady pace would be by today. */
function TargetBar({
  label,
  detail,
  target,
  elapsed,
  status,
}: {
  label: string;
  detail: string;
  target: FeeWaiverTarget;
  elapsed: number;
  status: FeeWaiverStatus;
}) {
  const pct = waiverFraction(target) * 100;
  return (
    <div className="space-y-1.5">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 text-xs">
        <span className="font-medium">{label}</span>
        <span className="text-muted-foreground">{detail}</span>
      </div>
      <div
        role="progressbar"
        aria-label={`${label}: ${Math.round(pct)}% of the target, ${Math.round(elapsed * 100)}% of the year gone`}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(pct)}
        className="bg-muted relative h-1.5 w-full rounded-full"
      >
        <div
          className="h-full rounded-full transition-[width]"
          style={{ width: `${pct}%`, backgroundColor: STATUS_FILL[status] }}
        />
        {status !== "met" && (
          <span
            aria-hidden
            title="Where a steady pace would be by today"
            className="bg-foreground/70 absolute top-[-3px] bottom-[-3px] w-0.5 rounded-full"
            style={{ left: `calc(${elapsed * 100}% - 1px)` }}
          />
        )}
      </div>
    </div>
  );
}

/**
 * Progress towards a card's fee waiver: a bar per target, the status chip and,
 * while the spend target is not met, roughly how much to spend each month.
 */
export function FeeWaiverProgress({ summary }: { summary: CardFeeSummary }) {
  const waiver = summary.waiver;
  if (!waiver) return null;
  const currency = summary.currency;
  const elapsed = windowElapsedFraction(
    waiver.window_start,
    waiver.window_end,
    waiver.days_left,
  );
  const dayWord = waiver.days_left === 1 ? "day" : "days";
  const calendarYear = waiver.window === "calendar_year";
  const windowName = calendarYear ? "this calendar year" : "this card year";
  // For a calendar-year waiver this year's spend decides next year's fee.
  const waivedFee = waivedFeeLabel(summary);

  return (
    <div className="space-y-2.5">
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-1.5">
          <p className="text-sm font-medium">Fee waiver</p>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  aria-label="How waiver progress is worked out"
                  className="text-muted-foreground hover:text-foreground rounded-sm"
                >
                  <Info className="size-3.5" />
                </button>
              </TooltipTrigger>
              <TooltipContent className="max-w-64">{DATA_NOTE}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
        <Badge tone={STATUS_TONE[waiver.status]}>
          {waiverStatusLabel(waiver.status)}
        </Badge>
      </div>

      {waiver.spend && (
        <TargetBar
          label="Spend"
          detail={`${formatMoney(waiver.spend.actual, currency)} of ${formatMoney(waiver.spend.target, currency)}`}
          target={waiver.spend}
          elapsed={elapsed}
          status={waiver.status}
        />
      )}
      {waiver.swipes && (
        <TargetBar
          label="Purchases"
          detail={`${waiver.swipes.actual} of ${waiver.swipes.target}`}
          target={waiver.swipes}
          elapsed={elapsed}
          status={waiver.status}
        />
      )}

      <p className="text-muted-foreground text-xs">
        {waiver.status === "met" ? (
          calendarYear ? (
            `Waiver reached for ${windowName}: ${waivedFee} is waived.`
          ) : (
            `Waiver reached for ${windowName}.`
          )
        ) : (
          <>
            {waiver.type === "spend_or_swipes"
              ? `Either target waives ${waivedFee}. `
              : calendarYear && `This year's spend waives ${waivedFee}. `}
            {waiver.per_month_needed !== null && waiver.per_month_needed > 0 && (
              <>
                Spend about{" "}
                <span className="text-foreground font-medium">
                  {formatMoney(waiver.per_month_needed, currency)}/month
                </span>{" "}
                to qualify ·{" "}
              </>
            )}
            <span className="whitespace-nowrap">
              {waiver.days_left} {dayWord} left
            </span>
          </>
        )}
      </p>
    </div>
  );
}
