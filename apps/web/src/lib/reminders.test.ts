import { describe, expect, it } from "vitest";
import { pickReminder, reminderSummary, REMINDER_KIND_LABELS, REMINDER_LEAD_DAYS } from "./reminders";
import type { Reminder } from "@/types/models";

function rem(over: Partial<Reminder>): Reminder {
  return {
    id: "r",
    user_id: "u",
    kind: "card_due",
    ref_id: "a",
    due_on: "2026-12-01T00:00:00Z",
    remind_at: "2026-11-28T01:00:00Z",
    enabled: true,
    created_at: "",
    updated_at: "",
    ...over,
  } as Reminder;
}

describe("pickReminder", () => {
  it("is null without reminders", () => {
    expect(pickReminder([])).toBeNull();
  });

  it("prefers the soonest reminder that has not been sent", () => {
    const sent = rem({ id: "sent", due_on: "2026-11-01T00:00:00Z", sent_at: "2026-10-29T01:00:00Z" });
    const next = rem({ id: "next", due_on: "2026-12-01T00:00:00Z" });
    const later = rem({ id: "later", due_on: "2027-01-01T00:00:00Z" });
    expect(pickReminder([later, sent, next])?.id).toBe("next");
  });

  it("falls back to the soonest when all are sent", () => {
    const a = rem({ id: "a", due_on: "2026-11-01T00:00:00Z", sent_at: "x" });
    const b = rem({ id: "b", due_on: "2026-12-01T00:00:00Z", sent_at: "x" });
    expect(pickReminder([b, a])?.id).toBe("a");
  });
});

describe("pickReminder with kinds", () => {
  it("only considers the given kinds", () => {
    const fee = rem({ id: "fee", kind: "card_annual_fee", due_on: "2026-11-15T00:00:00Z" });
    const due = rem({ id: "due", kind: "card_due", due_on: "2026-12-01T00:00:00Z" });
    expect(pickReminder([fee, due], ["card_due"])?.id).toBe("due");
    expect(pickReminder([fee], ["card_due"])).toBeNull();
  });

  it("has labels and lead times for the fee kinds", () => {
    expect(REMINDER_KIND_LABELS.card_annual_fee).toBe("Annual fee");
    expect(REMINDER_KIND_LABELS.fee_waiver_pace).toBe("Fee waiver progress");
    expect(REMINDER_LEAD_DAYS.card_annual_fee).toBe(30);
    expect(REMINDER_LEAD_DAYS.fee_waiver_pace).toBe(90);
  });
});

describe("reminderSummary", () => {
  const day = () => "1 Dec";

  it("says when it is off", () => {
    expect(reminderSummary(rem({ enabled: false }), day)).toBe("Off");
  });

  it("gives the lead time per kind", () => {
    expect(reminderSummary(rem({}), day)).toBe("3 days before, on Telegram");
    expect(reminderSummary(rem({ kind: "fd_maturity" }), day)).toBe(
      "7 days before, on Telegram",
    );
  });

  it("says when it was sent", () => {
    expect(reminderSummary(rem({ sent_at: "2026-11-28T01:00:00Z" }), day)).toBe(
      "Sent for 1 Dec",
    );
  });
});
