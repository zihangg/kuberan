import type { Reminder, ReminderKind } from "@/types/models";

/** How long before the due date each kind of reminder is sent (server constants). */
export const REMINDER_LEAD_DAYS: Record<ReminderKind, number> = {
  card_due: 3,
  loan_due: 3,
  fd_maturity: 7,
  card_annual_fee: 30,
  fee_waiver_pace: 90,
};

/** The label of a reminder kind, as used in Settings and on account pages. */
export const REMINDER_KIND_LABELS: Record<ReminderKind, string> = {
  card_due: "Credit card due dates",
  loan_due: "Loan payments",
  fd_maturity: "Fixed deposit maturity",
  card_annual_fee: "Annual fee",
  fee_waiver_pace: "Fee waiver progress",
};

/**
 * The reminder to show for an account: the next one not yet sent, else the soonest.
 * With `kinds`, only reminders of those kinds are considered.
 */
export function pickReminder(
  reminders: Reminder[],
  kinds?: ReminderKind[],
): Reminder | null {
  const pool = kinds ? reminders.filter((r) => kinds.includes(r.kind)) : reminders;
  if (pool.length === 0) return null;
  const sorted = [...pool].sort((a, b) => a.due_on.localeCompare(b.due_on));
  return sorted.find((r) => !r.sent_at) ?? sorted[0];
}

/** The reminder's status line, e.g. "Telegram, 3 days before" or "Sent for 1 Dec". */
export function reminderSummary(
  reminder: Reminder,
  formatDay: (iso: string) => string,
): string {
  if (!reminder.enabled) return "Off";
  if (reminder.sent_at) return `Sent for ${formatDay(reminder.due_on)}`;
  const lead = REMINDER_LEAD_DAYS[reminder.kind];
  return `${lead} days before, on Telegram`;
}
