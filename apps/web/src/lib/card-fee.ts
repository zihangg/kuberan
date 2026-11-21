import { z } from "zod";
import type { SetAnnualFeeRequest } from "@/types/api";
import type {
  CardAnnualFee,
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

/** The annual fee dialog's fields; 0 means "not set" for day and targets. */
export interface AnnualFeeForm {
  /** Cents. */
  amount: number;
  month: number;
  day: number;
  waiverType: "none" | "spend" | "swipes" | "spend_or_swipes";
  /** Cents. */
  waiverSpend: number;
  waiverSwipes: number;
  waiverWindow: "card_year" | "calendar_year";
}

/** Mirrors the server's rules for PUT /accounts/:id/annual-fee. */
export const annualFeeSchema = z
  .object({
    amount: z.number().int().positive("Enter the fee amount"),
    month: z.number().int().min(1).max(12),
    day: z.number().int().min(0).max(31),
    waiverType: z.enum(["none", "spend", "swipes", "spend_or_swipes"]),
    waiverSpend: z.number().int().min(0),
    waiverSwipes: z.number().int().min(0),
    waiverWindow: z.enum(["card_year", "calendar_year"]),
  })
  .superRefine((f, ctx) => {
    const needsSpend = f.waiverType === "spend" || f.waiverType === "spend_or_swipes";
    const needsSwipes = f.waiverType === "swipes" || f.waiverType === "spend_or_swipes";
    if (needsSpend && f.waiverSpend <= 0) {
      ctx.addIssue({ code: "custom", path: ["waiverSpend"], message: "Enter how much you need to spend" });
    }
    if (needsSwipes && f.waiverSwipes <= 0) {
      ctx.addIssue({ code: "custom", path: ["waiverSwipes"], message: "Enter how many purchases you need" });
    }
  });

/**
 * Validates the dialog's fields and builds the request, sending only the
 * fields that apply (no day when unknown, no target the waiver type ignores).
 */
export function toAnnualFeeRequest(
  form: AnnualFeeForm,
): { ok: true; data: SetAnnualFeeRequest } | { ok: false; error: string } {
  const parsed = annualFeeSchema.safeParse(form);
  if (!parsed.success) {
    return { ok: false, error: parsed.error.issues[0]?.message ?? "Check the fields" };
  }
  const f = parsed.data;
  const data: SetAnnualFeeRequest = { amount: f.amount, month: f.month };
  if (f.day > 0) data.day = f.day;
  if (f.waiverType !== "none") {
    data.waiver_type = f.waiverType;
    data.waiver_window = f.waiverWindow;
    if (f.waiverType !== "swipes") data.waiver_spend = f.waiverSpend;
    if (f.waiverType !== "spend") data.waiver_swipes = f.waiverSwipes;
  }
  return { ok: true, data };
}

/** The dialog's fields for an existing fee, or the defaults for a new one. */
export function annualFeeFormFrom(fee: CardAnnualFee | null | undefined): AnnualFeeForm {
  return {
    amount: fee?.amount ?? 0,
    month: fee?.fee_month ?? new Date().getMonth() + 1,
    day: fee?.fee_day ?? 0,
    waiverType: fee?.waiver_type ?? "none",
    waiverSpend: fee?.waiver_spend ?? 0,
    waiverSwipes: fee?.waiver_swipes ?? 0,
    waiverWindow: fee?.waiver_window ?? "card_year",
  };
}

/** Fraction of a waiver window gone by, 0 to 1, from its dates and the days left. */
export function windowElapsedFraction(
  windowStart: string,
  windowEnd: string,
  daysLeft: number,
): number {
  const start = Date.parse(windowStart.split("T")[0]);
  const end = Date.parse(windowEnd.split("T")[0]);
  const total = Math.round((end - start) / 86_400_000) + 1;
  if (!Number.isFinite(total) || total <= 0) return 0;
  return Math.min(1, Math.max(0, (total - daysLeft) / total));
}
