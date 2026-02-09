"use client";

import { useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import { useAccounts } from "@/hooks/use-accounts";
import { useCreateTransaction, useCreateTransfer } from "@/hooks/use-transactions";
import { useCategories } from "@/hooks/use-categories";
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
import { toRFC3339 } from "@/lib/format";
import type { TransactionType } from "@/types/models";

type DialogType = TransactionType | "transfer";

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

interface CreateTransactionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Pre-select an account when opening from account detail page */
  defaultAccountId?: number;
}

export function CreateTransactionDialog({
  open,
  onOpenChange,
  defaultAccountId,
}: CreateTransactionDialogProps) {
  const [accountId, setAccountId] = useState<string>(
    defaultAccountId ? String(defaultAccountId) : ""
  );
  const [type, setType] = useState<DialogType>("expense");
  const [amount, setAmount] = useState(0);
  const [categoryId, setCategoryId] = useState<string>("");
  const [description, setDescription] = useState("");
  const [date, setDate] = useState(todayISO());
  const [error, setError] = useState("");

  // Transfer-specific state
  const [fromAccountId, setFromAccountId] = useState<string>(
    defaultAccountId ? String(defaultAccountId) : ""
  );
  const [toAccountId, setToAccountId] = useState<string>("");

  const createTransaction = useCreateTransaction();
  const createTransfer = useCreateTransfer();
  const { data: accountsData } = useAccounts({ page_size: 100 });
  const { data: categoriesData } = useCategories({
    page_size: 100,
    type: type === "income" ? "income" : "expense",
  });

  const accounts = accountsData?.data ?? [];
  const categories = categoriesData?.data ?? [];
  const isTransfer = type === "transfer";
  const isSubmitting = createTransaction.isPending || createTransfer.isPending;

  // For transfer: filter to accounts, exclude selected from-account for to-account
  const toAccounts = accounts.filter(
    (a) => a.is_active && String(a.id) !== fromAccountId
  );

  function resetForm() {
    setAccountId(defaultAccountId ? String(defaultAccountId) : "");
    setType("expense");
    setAmount(0);
    setCategoryId("");
    setDescription("");
    setDate(todayISO());
    setError("");
    setFromAccountId(defaultAccountId ? String(defaultAccountId) : "");
    setToAccountId("");
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  }

  function handleTypeChange(newType: DialogType) {
    setType(newType);
    setCategoryId("");
    if (newType === "transfer") {
      // Pre-fill from-account with existing account selection or default
      setFromAccountId(accountId || (defaultAccountId ? String(defaultAccountId) : ""));
    } else {
      // When switching back from transfer, restore account from from-account
      if (type === "transfer" && fromAccountId) {
        setAccountId(fromAccountId);
      }
    }
  }

  function handleFromAccountChange(value: string) {
    setFromAccountId(value);
    if (toAccountId === value) {
      setToAccountId("");
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (amount <= 0) {
      setError("Amount must be greater than zero");
      return;
    }

    if (isTransfer) {
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
    } else {
      const selectedAccountId = Number(accountId);
      if (!selectedAccountId) {
        setError("Please select an account");
        return;
      }

      createTransaction.mutate(
        {
          account_id: selectedAccountId,
          type: type as TransactionType,
          amount,
          category_id: categoryId && categoryId !== "none" ? Number(categoryId) : undefined,
          description: description.trim() || undefined,
          date: date ? toRFC3339(date) : undefined,
        },
        {
          onSuccess: () => {
            toast.success("Transaction created");
            handleOpenChange(false);
          },
          onError: (err) => setError(getErrorMessage(err)),
        }
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {isTransfer ? "Transfer Funds" : "Add Transaction"}
          </DialogTitle>
          <DialogDescription>
            {isTransfer
              ? "Move money between your accounts."
              : "Record an income or expense transaction."}
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {/* Type */}
          <div className="flex flex-col gap-2">
            <Label>Type</Label>
            <div className="flex gap-2">
              <Button
                type="button"
                variant={type === "expense" ? "default" : "outline"}
                size="sm"
                className="flex-1"
                onClick={() => handleTypeChange("expense")}
                disabled={isSubmitting}
              >
                Expense
              </Button>
              <Button
                type="button"
                variant={type === "income" ? "default" : "outline"}
                size="sm"
                className="flex-1"
                onClick={() => handleTypeChange("income")}
                disabled={isSubmitting}
              >
                Income
              </Button>
              <Button
                type="button"
                variant={type === "transfer" ? "default" : "outline"}
                size="sm"
                className="flex-1"
                onClick={() => handleTypeChange("transfer")}
                disabled={isSubmitting}
              >
                Transfer
              </Button>
            </div>
          </div>

          {/* Account selector for expense/income */}
          {!isTransfer && !defaultAccountId && (
            <div className="flex flex-col gap-2">
              <Label htmlFor="tx-account">Account</Label>
              <Select
                value={accountId}
                onValueChange={setAccountId}
                disabled={isSubmitting}
              >
                <SelectTrigger id="tx-account" className="w-full">
                  <SelectValue placeholder="Select account" />
                </SelectTrigger>
                <SelectContent>
                  {accounts.map((a) => (
                    <SelectItem key={a.id} value={String(a.id)}>
                      {a.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {/* From/To Account selectors for transfer */}
          {isTransfer && (
            <>
              <div className="flex flex-col gap-2">
                <Label htmlFor="tx-from-account">From Account</Label>
                <Select
                  value={fromAccountId}
                  onValueChange={handleFromAccountChange}
                  disabled={isSubmitting}
                >
                  <SelectTrigger id="tx-from-account" className="w-full">
                    <SelectValue placeholder="Select source account" />
                  </SelectTrigger>
                  <SelectContent>
                    {accounts.filter((a) => a.is_active).map((a) => (
                      <SelectItem key={a.id} value={String(a.id)}>
                        {a.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="tx-to-account">To Account</Label>
                <Select
                  value={toAccountId}
                  onValueChange={setToAccountId}
                  disabled={isSubmitting}
                >
                  <SelectTrigger id="tx-to-account" className="w-full">
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
            </>
          )}

          {/* Amount */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="tx-amount">Amount</Label>
            <CurrencyInput
              id="tx-amount"
              value={amount}
              onChange={setAmount}
              placeholder="0.00"
              disabled={isSubmitting}
            />
          </div>

          {/* Category (hidden for transfers) */}
          {!isTransfer && (
            <div className="flex flex-col gap-2">
              <Label htmlFor="tx-category">Category</Label>
              <Select
                value={categoryId}
                onValueChange={setCategoryId}
                disabled={isSubmitting}
              >
                <SelectTrigger id="tx-category" className="w-full">
                  <SelectValue placeholder="Select category (optional)" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">No category</SelectItem>
                  {categories.map((cat) => (
                    <SelectItem key={cat.id} value={String(cat.id)}>
                      {cat.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {/* Description */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="tx-description">Description</Label>
            <Input
              id="tx-description"
              placeholder="Optional description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={isSubmitting}
            />
          </div>

          {/* Date */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="tx-date">Date</Label>
            <Input
              id="tx-date"
              type="date"
              value={date}
              onChange={(e) => setDate(e.target.value)}
              disabled={isSubmitting}
            />
          </div>

          <DialogFooter>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting
                ? isTransfer
                  ? "Transferring..."
                  : "Creating..."
                : isTransfer
                  ? "Transfer Funds"
                  : "Add Transaction"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
