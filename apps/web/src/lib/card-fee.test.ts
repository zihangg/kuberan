import { describe, expect, it } from "vitest";
import {
  annualFeeFormFrom,
  feeDateLabel,
  toAnnualFeeRequest,
  waiverFraction,
  waivedFeeLabel,
  waiverStatusLabel,
  windowElapsedFraction,
  type AnnualFeeForm,
} from "./card-fee";
import type { CardFeeSummary, FeeWaiverProgress } from "@/types/models";

describe("feeDateLabel", () => {
  it("shows day and short month when the day is known", () => {
    expect(feeDateLabel({ amount: 25000, next_fee_date: "2027-03-15", day_known: true })).toBe("15 Mar");
  });

  it("shows the full month name when the day is unknown", () => {
    expect(feeDateLabel({ amount: 25000, next_fee_date: "2027-03-01", day_known: false })).toBe("March");
  });
});

describe("waiverStatusLabel", () => {
  it("labels each status", () => {
    expect(waiverStatusLabel("met")).toBe("Met");
    expect(waiverStatusLabel("on_track")).toBe("On track");
    expect(waiverStatusLabel("behind")).toBe("Behind");
  });
});

describe("waiverFraction", () => {
  it("clamps to 1 when the target is exceeded", () => {
    expect(waiverFraction({ target: 100, actual: 150, remaining: 0 })).toBe(1);
  });
  it("is the share reached", () => {
    expect(waiverFraction({ target: 100, actual: 25, remaining: 75 })).toBe(0.25);
  });
  it("is 0 for a zero target or negative actual", () => {
    expect(waiverFraction({ target: 0, actual: 5, remaining: 0 })).toBe(0);
    expect(waiverFraction({ target: 100, actual: -5, remaining: 105 })).toBe(0);
  });
});

const base: AnnualFeeForm = {
  amount: 25000,
  month: 3,
  day: 0,
  waiverType: "none",
  waiverSpend: 0,
  waiverSwipes: 0,
  waiverWindow: "card_year",
};

describe("toAnnualFeeRequest", () => {
  it("sends only amount and month for a fee with no day or waiver", () => {
    expect(toAnnualFeeRequest(base)).toEqual({ ok: true, data: { amount: 25000, month: 3 } });
  });

  it("includes the day when known", () => {
    const r = toAnnualFeeRequest({ ...base, day: 15 });
    expect(r.ok && r.data.day).toBe(15);
  });

  it("rejects a zero amount", () => {
    expect(toAnnualFeeRequest({ ...base, amount: 0 }).ok).toBe(false);
  });

  it("rejects an out-of-range month or day", () => {
    expect(toAnnualFeeRequest({ ...base, month: 13 }).ok).toBe(false);
    expect(toAnnualFeeRequest({ ...base, day: 32 }).ok).toBe(false);
  });

  it("drops the swipes target on a spend waiver", () => {
    expect(
      toAnnualFeeRequest({ ...base, waiverType: "spend", waiverSpend: 1200000, waiverSwipes: 12 }),
    ).toEqual({
      ok: true,
      data: { amount: 25000, month: 3, waiver_type: "spend", waiver_spend: 1200000, waiver_window: "card_year" },
    });
  });

  it("drops the spend target on a swipes waiver", () => {
    const r = toAnnualFeeRequest({
      ...base,
      waiverType: "swipes",
      waiverSpend: 500,
      waiverSwipes: 12,
      waiverWindow: "calendar_year",
    });
    expect(r).toEqual({
      ok: true,
      data: { amount: 25000, month: 3, waiver_type: "swipes", waiver_swipes: 12, waiver_window: "calendar_year" },
    });
  });

  it("needs both targets for spend_or_swipes", () => {
    expect(toAnnualFeeRequest({ ...base, waiverType: "spend_or_swipes", waiverSpend: 100 }).ok).toBe(false);
    expect(toAnnualFeeRequest({ ...base, waiverType: "spend_or_swipes", waiverSwipes: 12 }).ok).toBe(false);
    const r = toAnnualFeeRequest({ ...base, waiverType: "spend_or_swipes", waiverSpend: 100, waiverSwipes: 12 });
    expect(r.ok && r.data.waiver_spend).toBe(100);
    expect(r.ok && r.data.waiver_swipes).toBe(12);
  });

  it("sends no targets without a waiver type", () => {
    const r = toAnnualFeeRequest({ ...base, waiverSpend: 100, waiverSwipes: 12 });
    expect(r).toEqual({ ok: true, data: { amount: 25000, month: 3 } });
  });

  it("requires a positive spend target on a spend waiver", () => {
    expect(toAnnualFeeRequest({ ...base, waiverType: "spend" }).ok).toBe(false);
  });
});

describe("annualFeeFormFrom", () => {
  it("maps a stored fee to the form", () => {
    const f = annualFeeFormFrom({
      id: "f",
      account_id: "a",
      user_id: "u",
      amount: 25000,
      fee_month: 3,
      fee_day: null,
      waiver_type: "swipes",
      waiver_spend: null,
      waiver_swipes: 12,
      waiver_window: "calendar_year",
      created_at: "",
      updated_at: "",
    });
    expect(f).toEqual({
      amount: 25000,
      month: 3,
      day: 0,
      waiverType: "swipes",
      waiverSpend: 0,
      waiverSwipes: 12,
      waiverWindow: "calendar_year",
    });
  });
});

describe("windowElapsedFraction", () => {
  it("is the share of the window gone by", () => {
    expect(windowElapsedFraction("2026-01-01", "2026-12-31", 0)).toBe(1);
    expect(windowElapsedFraction("2026-01-01", "2026-12-31", 365)).toBe(0);
    expect(windowElapsedFraction("2026-01-01", "2026-12-31", 182.5)).toBeCloseTo(0.5);
  });
});

describe("waivedFeeLabel", () => {
  const waiver = (window: FeeWaiverProgress["window"], start: string, end: string): FeeWaiverProgress => ({
    type: "spend",
    window,
    window_start: start,
    window_end: end,
    spend: { target: 1_200_000, actual: 100_000, remaining: 1_100_000 },
    swipes: null,
    status: "behind",
    per_month_needed: 100_000,
    days_left: 40,
  });
  const summary = (w: FeeWaiverProgress, nextFee: string): CardFeeSummary => ({
    currency: "MYR",
    fee: { amount: 25000, next_fee_date: nextFee, day_known: true },
    waiver: w,
    spent_last_12_months: 0,
  });

  it("is the upcoming fee for a card-year window", () => {
    expect(waivedFeeLabel(summary(waiver("card_year", "2026-03-16", "2027-03-15"), "2027-03-15"))).toBe(
      "the fee",
    );
  });
  it("is next year's fee for a calendar-year window", () => {
    expect(waivedFeeLabel(summary(waiver("calendar_year", "2026-01-01", "2026-12-31"), "2027-03-15"))).toBe(
      "next year's fee",
    );
  });
  it("is this year's fee when the fee falls on 31 Dec", () => {
    expect(waivedFeeLabel(summary(waiver("calendar_year", "2026-01-01", "2026-12-31"), "2026-12-31"))).toBe(
      "this year's fee",
    );
  });
});
