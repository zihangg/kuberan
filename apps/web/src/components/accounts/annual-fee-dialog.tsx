"use client";

import { useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import { useDeleteAnnualFee, useSetAnnualFee } from "@/hooks/use-card-fee";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { CurrencyInput } from "@/components/ui/currency-input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  annualFeeFormFrom,
  toAnnualFeeRequest,
  type AnnualFeeForm,
} from "@/lib/card-fee";
import type { Account } from "@/types/models";

const MONTH_NAMES = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const DAYS = Array.from({ length: 31 }, (_, i) => i + 1);

const WAIVER_TYPES: { value: AnnualFeeForm["waiverType"]; label: string }[] = [
  { value: "none", label: "No, it can't be waived" },
  { value: "spend", label: "Spend a minimum amount" },
  { value: "swipes", label: "Make a number of purchases" },
  { value: "spend_or_swipes", label: "Either one" },
];

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof ApiClientError ? err.message : fallback;
}

/**
 * Adds or edits a card's annual fee: amount, month, optional day and the
 * waiver condition. Editing also offers "Remove annual fee". The form starts
 * from the stored fee on mount, so give it a new `key` each time it opens.
 */
export function AnnualFeeDialog({
  account,
  open,
  onOpenChange,
}: {
  account: Account;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const existing = account.annual_fee ?? null;
  const [form, setForm] = useState<AnnualFeeForm>(() => annualFeeFormFrom(existing));
  const [error, setError] = useState("");
  const [confirmRemove, setConfirmRemove] = useState(false);
  const setFee = useSetAnnualFee(account.id);
  const removeFee = useDeleteAnnualFee(account.id);
  const busy = setFee.isPending || removeFee.isPending;

  function patch(p: Partial<AnnualFeeForm>) {
    setForm((f) => ({ ...f, ...p }));
  }

  function save(e: React.FormEvent) {
    e.preventDefault();
    const req = toAnnualFeeRequest(form);
    if (!req.ok) {
      setError(req.error);
      return;
    }
    setError("");
    setFee.mutate(req.data, {
      onSuccess: () => {
        toast.success(existing ? "Annual fee updated" : "Annual fee added");
        onOpenChange(false);
      },
      onError: (err) => setError(errorMessage(err, "Couldn't save the annual fee")),
    });
  }

  function remove() {
    removeFee.mutate(undefined, {
      onSuccess: () => {
        toast.success("Annual fee removed");
        setConfirmRemove(false);
        onOpenChange(false);
      },
      onError: (err) =>
        toast.error(errorMessage(err, "Couldn't remove the annual fee")),
    });
  }

  const showSpend = form.waiverType === "spend" || form.waiverType === "spend_or_swipes";
  const showSwipes = form.waiverType === "swipes" || form.waiverType === "spend_or_swipes";

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{existing ? "Edit annual fee" : "Add annual fee"}</DialogTitle>
            <DialogDescription>
              We&apos;ll remind you on Telegram a month before it&apos;s charged
              and track any waiver on this card.
            </DialogDescription>
          </DialogHeader>

          {error && (
            <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
              {error}
            </div>
          )}

          <form onSubmit={save} className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <Label htmlFor="annual-fee-amount">Amount</Label>
              <CurrencyInput
                id="annual-fee-amount"
                currency={account.currency}
                value={form.amount}
                onChange={(amount) => patch({ amount })}
                disabled={busy}
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="flex min-w-0 flex-col gap-2">
                <Label htmlFor="annual-fee-month">Month</Label>
                <Select
                  value={String(form.month)}
                  onValueChange={(v) => patch({ month: Number(v) })}
                  disabled={busy}
                >
                  <SelectTrigger id="annual-fee-month" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {MONTH_NAMES.map((m, i) => (
                      <SelectItem key={m} value={String(i + 1)}>
                        {m}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex min-w-0 flex-col gap-2">
                <Label htmlFor="annual-fee-day">Day (optional)</Label>
                <Select
                  value={String(form.day)}
                  onValueChange={(v) => patch({ day: Number(v) })}
                  disabled={busy}
                >
                  <SelectTrigger id="annual-fee-day" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="0">Not sure</SelectItem>
                    {DAYS.map((d) => (
                      <SelectItem key={d} value={String(d)}>
                        {d}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <fieldset className="flex flex-col gap-4 border-t pt-4">
              <legend className="sr-only">Waiver</legend>
              <div className="flex flex-col gap-2">
                <Label htmlFor="annual-fee-waiver">Can it be waived?</Label>
                <Select
                  value={form.waiverType}
                  onValueChange={(v) =>
                    patch({ waiverType: v as AnnualFeeForm["waiverType"] })
                  }
                  disabled={busy}
                >
                  <SelectTrigger id="annual-fee-waiver" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {WAIVER_TYPES.map((t) => (
                      <SelectItem key={t.value} value={t.value}>
                        {t.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {showSpend && (
                <div className="flex flex-col gap-2">
                  <Label htmlFor="annual-fee-spend">Spend at least</Label>
                  <CurrencyInput
                    id="annual-fee-spend"
                    currency={account.currency}
                    value={form.waiverSpend}
                    onChange={(waiverSpend) => patch({ waiverSpend })}
                    disabled={busy}
                  />
                </div>
              )}

              {showSwipes && (
                <div className="flex flex-col gap-2">
                  <Label htmlFor="annual-fee-swipes">Purchases needed</Label>
                  <Input
                    id="annual-fee-swipes"
                    type="number"
                    inputMode="numeric"
                    min={1}
                    step={1}
                    value={form.waiverSwipes > 0 ? String(form.waiverSwipes) : ""}
                    onChange={(e) =>
                      patch({
                        waiverSwipes: Math.max(0, Math.floor(Number(e.target.value) || 0)),
                      })
                    }
                    placeholder="e.g. 12"
                    disabled={busy}
                  />
                </div>
              )}

              {form.waiverType !== "none" && (
                <div className="flex flex-col gap-2">
                  <Label htmlFor="annual-fee-window">Counted over</Label>
                  <Select
                    value={form.waiverWindow}
                    onValueChange={(v) =>
                      patch({ waiverWindow: v as AnnualFeeForm["waiverWindow"] })
                    }
                    disabled={busy}
                  >
                    <SelectTrigger id="annual-fee-window" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="card_year">
                        The card year (up to the fee date)
                      </SelectItem>
                      <SelectItem value="calendar_year">
                        The calendar year (January to December)
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              )}
            </fieldset>

            <DialogFooter className="gap-2 sm:justify-between">
              {existing ? (
                <Button
                  type="button"
                  variant="ghost"
                  className="text-destructive hover:text-destructive"
                  onClick={() => setConfirmRemove(true)}
                  disabled={busy}
                >
                  Remove annual fee
                </Button>
              ) : (
                <span className="hidden sm:block" />
              )}
              <Button type="submit" disabled={busy}>
                {setFee.isPending ? "Saving..." : "Save"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={confirmRemove} onOpenChange={setConfirmRemove}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove annual fee</AlertDialogTitle>
            <AlertDialogDescription>
              This removes the fee, its waiver and their reminders from this
              card. Your transactions are not changed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removeFee.isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={(e) => {
                e.preventDefault();
                remove();
              }}
              disabled={removeFee.isPending}
            >
              {removeFee.isPending ? "Removing..." : "Remove"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
