"use client";

import { useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import { useAccounts } from "@/hooks/use-accounts";
import { useCreateTransaction } from "@/hooks/use-transactions";
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

function getErrorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
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
  const [type, setType] = useState<TransactionType>("expense");
  const [amount, setAmount] = useState(0);
  const [categoryId, setCategoryId] = useState<string>("");
  const [description, setDescription] = useState("");
  const [date, setDate] = useState(todayISO());
  const [error, setError] = useState("");

  const createTransaction = useCreateTransaction();
  const { data: accountsData } = useAccounts({ page_size: 100 });
  const { data: categoriesData } = useCategories({ page_size: 100, type: type === "income" ? "income" : "expense" });

  const accounts = accountsData?.data ?? [];
  const categories = categoriesData?.data ?? [];
  const isSubmitting = createTransaction.isPending;

  function resetForm() {
    setAccountId(defaultAccountId ? String(defaultAccountId) : "");
    setType("expense");
    setAmount(0);
    setCategoryId("");
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

  function handleTypeChange(newType: TransactionType) {
    setType(newType);
    setCategoryId(""); // reset category when type changes
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    const selectedAccountId = Number(accountId);
    if (!selectedAccountId) {
      setError("Please select an account");
      return;
    }

    if (amount <= 0) {
      setError("Amount must be greater than zero");
      return;
    }

    createTransaction.mutate(
      {
        account_id: selectedAccountId,
        type,
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

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Add Transaction</DialogTitle>
          <DialogDescription>
            Record an income or expense transaction.
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {/* Account selector (hidden if pre-selected) */}
          {!defaultAccountId && (
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
            </div>
          </div>

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

          {/* Category */}
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
              {isSubmitting ? "Creating..." : "Add Transaction"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
