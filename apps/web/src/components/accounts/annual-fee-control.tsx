"use client";

import { useState } from "react";
import Link from "next/link";
import { toast } from "sonner";
import { Receipt } from "lucide-react";
import { ApiClientError } from "@/lib/api-client";
import { useFeeWaiver } from "@/hooks/use-card-fee";
import { useAccountReminders, useSetReminderEnabled } from "@/hooks/use-reminders";
import { useTelegramLink } from "@/hooks/use-telegram";
import { AnnualFeeDialog } from "@/components/accounts/annual-fee-dialog";
import { FeeWaiverProgress } from "@/components/accounts/fee-waiver-progress";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { feeDateLabel } from "@/lib/card-fee";
import { formatDate, formatMoney } from "@/lib/format";
import { pickReminder, reminderSummary } from "@/lib/reminders";
import type { Account } from "@/types/models";

/**
 * A credit card's annual fee: an "Add annual fee" button until one is set,
 * then the fee with its reminder switch and the waiver progress. A sibling of
 * `ReminderControl` on the card page.
 */
export function AnnualFeeControl({ account }: { account: Account }) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogKey, setDialogKey] = useState(0);
  const hasFee = !!account.annual_fee;
  const { data: summary } = useFeeWaiver(hasFee ? account.id : "");
  const { data: reminders } = useAccountReminders(hasFee ? account.id : "");
  const setEnabled = useSetReminderEnabled(account.id);
  const { data: telegram } = useTelegramLink();
  const linked =
    !!telegram?.link && telegram.link.telegram_user_id > 0 && telegram.link.is_active;
  const reminder = pickReminder(reminders ?? [], ["card_annual_fee"]);
  // ReminderControl already nudges to link Telegram while the payment reminder is on.
  const paymentReminderOn = !!pickReminder(reminders ?? [], ["card_due"])?.enabled;

  function openDialog() {
    setDialogKey((k) => k + 1);
    setDialogOpen(true);
  }

  const dialog = (
    <AnnualFeeDialog
      key={dialogKey}
      account={account}
      open={dialogOpen}
      onOpenChange={setDialogOpen}
    />
  );

  if (!hasFee) {
    return (
      <>
        <Button variant="ghost" size="sm" className="-ml-2 w-fit" onClick={openDialog}>
          <Receipt className="size-4" />
          Add annual fee
        </Button>
        {dialog}
      </>
    );
  }

  const fee = summary?.fee;
  const currency = summary?.currency ?? account.currency;
  const amount = fee?.amount ?? account.annual_fee?.amount ?? 0;
  const when = fee
    ? `${fee.day_known ? "around" : "in"} ${feeDateLabel(fee)}`
    : "";

  return (
    <div className="space-y-3">
      <div className="space-y-1.5">
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2">
            <Receipt
              className={`size-4 shrink-0 ${reminder?.enabled ? "text-primary" : "text-muted-foreground"}`}
            />
            <div className="min-w-0">
              <p className="text-sm font-medium">
                Annual fee · {formatMoney(amount, currency)}
                {when && (
                  <>
                    {" · "}
                    <span className="whitespace-nowrap">{when}</span>
                  </>
                )}
              </p>
              <p className="text-muted-foreground text-xs">
                {reminder && (
                  <>
                    {reminder.enabled
                      ? reminderSummary(reminder, formatDate)
                      : "Reminder off"}
                    {" · "}
                  </>
                )}
                <button
                  type="button"
                  className="text-primary hover:underline"
                  onClick={openDialog}
                >
                  Edit
                </button>
              </p>
            </div>
          </div>
          {reminder && (
            <Switch
              aria-label="Remind me about the annual fee on Telegram"
              checked={reminder.enabled}
              disabled={setEnabled.isPending}
              onCheckedChange={(enabled) =>
                setEnabled.mutate(
                  { id: reminder.id, enabled },
                  {
                    onError: (err) =>
                      toast.error(
                        err instanceof ApiClientError
                          ? err.message
                          : "Couldn't update the reminder",
                      ),
                  },
                )
              }
            />
          )}
        </div>
        {reminder?.enabled && !linked && !paymentReminderOn && (
          <p className="text-muted-foreground text-xs">
            Reminders arrive on Telegram.{" "}
            <Link href="/settings?tab=telegram" className="text-primary hover:underline">
              Link your account
            </Link>{" "}
            to receive them.
          </p>
        )}
      </div>

      {summary?.waiver ? (
        <FeeWaiverProgress summary={summary} />
      ) : (
        summary && (
          <p className="text-muted-foreground text-xs">
            {formatMoney(summary.spent_last_12_months, currency)} spent on this
            card in the last 12 months
          </p>
        )
      )}
      {dialog}
    </div>
  );
}
