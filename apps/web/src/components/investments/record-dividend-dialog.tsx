"use client";

import { useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import { useRecordDividend } from "@/hooks/use-investments";
import { toRFC3339 } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { CurrencyInput } from "@/components/ui/currency-input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

function getErrorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
    return error.message;
  }
  return "An unexpected error occurred";
}

interface RecordDividendDialogProps {
  investmentId: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function RecordDividendDialog({
  investmentId,
  open,
  onOpenChange,
}: RecordDividendDialogProps) {
  const [date, setDate] = useState("");
  const [amount, setAmount] = useState(0);
  const [dividendType, setDividendType] = useState("cash");
  const [notes, setNotes] = useState("");
  const [error, setError] = useState("");

  const recordDividend = useRecordDividend(investmentId);
  const isSubmitting = recordDividend.isPending;

  function resetForm() {
    setDate("");
    setAmount(0);
    setDividendType("cash");
    setNotes("");
    setError("");
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (!date) {
      setError("Date is required");
      return;
    }

    if (amount <= 0) {
      setError("Amount must be greater than zero");
      return;
    }

    recordDividend.mutate(
      {
        date: toRFC3339(date),
        amount,
        dividend_type: dividendType || undefined,
        notes: notes.trim() || undefined,
      },
      {
        onSuccess: () => {
          toast.success("Dividend recorded");
          handleOpenChange(false);
        },
        onError: (err) => setError(getErrorMessage(err)),
      }
    );
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Record Dividend</DialogTitle>
          <DialogDescription>
            Record a dividend payment for this investment.
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-5">
          <div className="flex flex-col gap-2">
            <Label htmlFor="div-date">Date</Label>
            <Input
              id="div-date"
              type="date"
              value={date}
              onChange={(e) => setDate(e.target.value)}
              disabled={isSubmitting}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="div-amount">Amount</Label>
            <CurrencyInput
              id="div-amount"
              value={amount}
              onChange={setAmount}
              placeholder="0.00"
              disabled={isSubmitting}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="div-type">Dividend Type</Label>
            <Select
              value={dividendType}
              onValueChange={setDividendType}
              disabled={isSubmitting}
            >
              <SelectTrigger id="div-type">
                <SelectValue placeholder="Select type" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="cash">Cash</SelectItem>
                <SelectItem value="stock">Stock</SelectItem>
                <SelectItem value="special">Special</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="div-notes">Notes (optional)</Label>
            <Textarea
              id="div-notes"
              placeholder="Optional notes..."
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              disabled={isSubmitting}
            />
          </div>

          <DialogFooter>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Recording..." : "Record Dividend"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
