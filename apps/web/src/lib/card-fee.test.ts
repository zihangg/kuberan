import { describe, expect, it } from "vitest";
import { feeDateLabel, waiverFraction, waiverStatusLabel } from "./card-fee";

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
