import type {
  CardFeeInfo,
  FeeWaiverStatus,
  FeeWaiverTarget,
} from "@/types/models";

/** When the next fee is charged: "15 Mar", or "March" when only the month is known. */
export function feeDateLabel(fee: CardFeeInfo): string {
  const [year, month, day] = fee.next_fee_date.split("T")[0].split("-").map(Number);
  const date = new Date(year, month - 1, day);
  if (fee.day_known) {
    return date.toLocaleDateString("en-MY", { day: "numeric", month: "short" });
  }
  return date.toLocaleDateString("en-MY", { month: "long" });
}

/** The status of a waiver as a short label. */
export function waiverStatusLabel(status: FeeWaiverStatus): string {
  switch (status) {
    case "met":
      return "Met";
    case "on_track":
      return "On track";
    case "behind":
      return "Behind";
  }
}

/** How far a waiver target is reached, 0 to 1. */
export function waiverFraction(t: FeeWaiverTarget): number {
  if (t.target <= 0) return 0;
  return Math.min(1, Math.max(0, t.actual / t.target));
}
