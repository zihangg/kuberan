"use client";

import { useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import { useAccounts } from "@/hooks/use-accounts";
import { useCreateTransfer } from "@/hooks/use-transactions";
import { toRFC3339 } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
    if (error.code === "SAME_ACCOUNT") {
      return "Cannot transfer to the same account";
    }
    if (error.code === "INSUFFICIENT_BALANCE") {
      return "Insufficient balance for this transfer";
    }
    return error.message;
  }
  return "An unexpected error occurred";
}

function todayISO(): string {
  return new Date().toISOString().split("T")[0];
}

interface CreateTransferDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Pre-select the "from" account when opening from account detail page */
  defaultFromAccountId?: number;
}

export function CreateTransferDialog({
  open,
  onOpenChange,
  defaultFromAccountId,
}: CreateTransferDialogProps) {
  const [fromAccountId, setFromAccountId] = useState<string>(
    defaultFromAccountId ? String(defaultFromAccountId) : ""
  );
  const [toAccountId, setToAccountId] = useState<string>("");
  const [amount, setAmount] = useState(0);
  const [description, setDescription] = useState("");
  const [date, setDate] = useState(todayISO());
  const [error, setError] = useState("");

  const createTransfer = useCreateTransfer();
  const { data: accountsData } = useAccounts({ page_size: 100 });

  const accounts = accountsData?.data ?? [];
  const cashAccounts = accounts.filter((a) => a.type === "cash" && a.is_active);
  const toAccounts = cashAccounts.filter(
    (a) => String(a.id) !== fromAccountId
  );
  const isSubmitting = createTransfer.isPending;

  function resetForm() {
    setFromAccountId(defaultFromAccountId ? String(defaultFromAccountId) : "");
    setToAccountId("");
    setAmount(0);
    setDescription("");
    setDate(todayISO());
    setError("");
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  }

  function handleFromAccountChange(value: string) {
    setFromAccountId(value);
    // Reset "to" if it's now the same as "from"
    if (toAccountId === value) {
      setToAccountId("");
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    const from = Number(fromAccountId);
    const to = Number(toAccountId);

    if (!from) {
      setError("Please select a source account");
      return;
    }
    if (!to) {
      setError("Please select a destination account");
      return;
    }
    if (from === to) {
      setError("Cannot transfer to the same account");
      return;
    }
    if (amount <= 0) {
      setError("Amount must be greater than zero");
      return;
    }

    createTransfer.mutate(
      {
        from_account_id: from,
        to_account_id: to,
        amount,
        description: description.trim() || undefined,
        date: date ? toRFC3339(date) : undefined,
      },
      {
        onSuccess: () => {
          toast.success("Transfer completed");
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
          <DialogTitle>Transfer Funds</DialogTitle>
          <DialogDescription>
            Move money between your cash accounts.
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {/* From Account */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="transfer-from">From Account</Label>
            <Select
              value={fromAccountId}
              onValueChange={handleFromAccountChange}
              disabled={isSubmitting || !!defaultFromAccountId}
            >
              <SelectTrigger id="transfer-from" className="w-full">
                <SelectValue placeholder="Select source account" />
              </SelectTrigger>
              <SelectContent>
                {cashAccounts.map((a) => (
                  <SelectItem key={a.id} value={String(a.id)}>
                    {a.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* To Account */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="transfer-to">To Account</Label>
            <Select
              value={toAccountId}
              onValueChange={setToAccountId}
              disabled={isSubmitting}
            >
              <SelectTrigger id="transfer-to" className="w-full">
                <SelectValue placeholder="Select destination account" />
              </SelectTrigger>
              <SelectContent>
                {toAccounts.map((a) => (
                  <SelectItem key={a.id} value={String(a.id)}>
                    {a.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Amount */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="transfer-amount">Amount</Label>
            <CurrencyInput
              id="transfer-amount"
              value={amount}
              onChange={setAmount}
              placeholder="0.00"
              disabled={isSubmitting}
            />
          </div>

          {/* Description */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="transfer-description">Description</Label>
            <Input
              id="transfer-description"
              placeholder="Optional description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={isSubmitting}
            />
          </div>

          {/* Date */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="transfer-date">Date</Label>
            <Input
              id="transfer-date"
              type="date"
              value={date}
              onChange={(e) => setDate(e.target.value)}
              disabled={isSubmitting}
            />
          </div>

          <DialogFooter>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Transferring..." : "Transfer Funds"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
