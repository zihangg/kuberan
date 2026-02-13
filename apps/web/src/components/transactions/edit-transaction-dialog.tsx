"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import { useAccounts } from "@/hooks/use-accounts";
import {
  useUpdateTransaction,
  useDeleteTransaction,
} from "@/hooks/use-transactions";
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
import { Badge } from "@/components/ui/badge";
import { formatCurrency, formatDate, toRFC3339 } from "@/lib/format";
import type { Transaction, TransactionType } from "@/types/models";
import type { UpdateTransactionRequest } from "@/types/api";

function getErrorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
    switch (error.code) {
      case "TRANSACTION_NOT_EDITABLE":
        return "This transaction type cannot be edited";
      case "INVALID_TYPE_CHANGE":
        return "Cannot change to this transaction type";
      case "ACCOUNT_NOT_FOUND":
        return "Selected account not found";
      default:
        return error.message;
    }
  }
  return "An unexpected error occurred";
}

interface EditTransactionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  transaction: Transaction | null;
}

export function EditTransactionDialog({
  open,
  onOpenChange,
  transaction,
}: EditTransactionDialogProps) {
  const [type, setType] = useState<TransactionType>("expense");
  const [accountId, setAccountId] = useState<string>("");
  const [amount, setAmount] = useState(0);
  const [categoryId, setCategoryId] = useState<string>("");
  const [description, setDescription] = useState("");
  const [date, setDate] = useState("");
  const [error, setError] = useState("");
  const [deleteOpen, setDeleteOpen] = useState(false);

  const updateTransaction = useUpdateTransaction(transaction?.id ?? "");
  const deleteTransaction = useDeleteTransaction();
  const { data: accountsData } = useAccounts({ page_size: 100 });
  const { data: categoriesData } = useCategories({
    page_size: 100,
    type: type === "income" ? "income" : "expense",
  });

  const accounts = accountsData?.data ?? [];
  const categories = categoriesData?.data ?? [];
  const isSaving = updateTransaction.isPending;
  const isDeleting = deleteTransaction.isPending;
  const isEditable =
    transaction?.type === "income" || transaction?.type === "expense";

  // Sync form state when transaction changes
  useEffect(() => {
    if (transaction) {
      setType(transaction.type);
      setAccountId(transaction.account_id);
      setAmount(transaction.amount);
      setCategoryId(
        transaction.category_id ?? "none"
      );
      setDescription(transaction.description ?? "");
      setDate(transaction.date ? transaction.date.split("T")[0] : "");
      setError("");
    }
  }, [transaction]);

  function handleTypeChange(newType: TransactionType) {
    setType(newType);
    setCategoryId("none");
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!transaction) return;
    setError("");

    if (!accountId) {
      setError("Please select an account");
      return;
    }
    if (amount <= 0) {
      setError("Amount must be greater than zero");
      return;
    }

    // Build payload with only changed fields
    const payload: UpdateTransactionRequest = {};

    if (accountId !== transaction.account_id)
      payload.account_id = accountId;
    if (type !== transaction.type) payload.type = type;
    if (amount !== transaction.amount) payload.amount = amount;

    // Category: compare new value with original
    const newCatId =
      categoryId && categoryId !== "none" ? categoryId : null;
    const origCatId = transaction.category_id ?? null;
    if (newCatId !== origCatId) payload.category_id = newCatId;

    const trimmedDesc = description.trim();
    if (trimmedDesc !== (transaction.description ?? ""))
      payload.description = trimmedDesc;

    const origDate = transaction.date ? transaction.date.split("T")[0] : "";
    if (date !== origDate) payload.date = date ? toRFC3339(date) : undefined;

    if (Object.keys(payload).length === 0) {
      onOpenChange(false);
      return;
    }

    updateTransaction.mutate(payload, {
      onSuccess: () => {
        toast.success("Transaction updated");
        onOpenChange(false);
      },
      onError: (err) => setError(getErrorMessage(err)),
    });
  }

  function handleDelete() {
    if (!transaction) return;
    deleteTransaction.mutate(transaction.id, {
      onSuccess: () => {
        toast.success("Transaction deleted");
        setDeleteOpen(false);
        onOpenChange(false);
      },
      onError: (err) =>
        toast.error(
          err instanceof ApiClientError
            ? err.message
            : "Failed to delete transaction"
        ),
    });
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {isEditable ? "Edit Transaction" : "Transaction Details"}
            </DialogTitle>
            <DialogDescription>
              {isEditable
                ? "Update transaction details."
                : "Transfer and investment transactions cannot be edited. Delete and recreate if needed."}
            </DialogDescription>
          </DialogHeader>

          {error && (
            <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
              {error}
            </div>
          )}

          {isEditable ? (
            <form onSubmit={handleSubmit} className="flex flex-col gap-5">
              {/* Type toggle */}
              <div className="flex flex-col gap-2">
                <Label>Type</Label>
                <div className="flex flex-col sm:flex-row gap-2">
                  <Button
                    type="button"
                    variant={type === "expense" ? "default" : "outline"}
                    size="sm"
                    className="flex-1"
                    onClick={() => handleTypeChange("expense")}
                    disabled={isSaving}
                  >
                    Expense
                  </Button>
                  <Button
                    type="button"
                    variant={type === "income" ? "default" : "outline"}
                    size="sm"
                    className="flex-1"
                    onClick={() => handleTypeChange("income")}
                    disabled={isSaving}
                  >
                    Income
                  </Button>
                </div>
              </div>

              {/* Account */}
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-tx-account">Account</Label>
                <Select
                  value={accountId}
                  onValueChange={setAccountId}
                  disabled={isSaving}
                >
                  <SelectTrigger id="edit-tx-account" className="w-full">
                    <SelectValue placeholder="Select account" />
                  </SelectTrigger>
                  <SelectContent>
                    {accounts.map((a) => (
                      <SelectItem key={a.id} value={a.id}>
                        {a.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Amount */}
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-tx-amount">Amount</Label>
                <CurrencyInput
                  id="edit-tx-amount"
                  value={amount}
                  onChange={setAmount}
                  placeholder="0.00"
                  disabled={isSaving}
                />
              </div>

              {/* Category */}
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-tx-category">Category</Label>
                <Select
                  value={categoryId}
                  onValueChange={setCategoryId}
                  disabled={isSaving}
                >
                  <SelectTrigger id="edit-tx-category" className="w-full">
                    <SelectValue placeholder="Select category (optional)" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">No category</SelectItem>
                    {categories.map((cat) => (
                      <SelectItem key={cat.id} value={cat.id}>
                        {cat.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Description */}
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-tx-description">Description</Label>
                <Input
                  id="edit-tx-description"
                  placeholder="Optional description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  disabled={isSaving}
                  maxLength={500}
                />
              </div>

              {/* Date */}
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-tx-date">Date</Label>
                <Input
                  id="edit-tx-date"
                  type="date"
                  value={date}
                  onChange={(e) => setDate(e.target.value)}
                  disabled={isSaving}
                />
              </div>

              <DialogFooter className="flex gap-2 sm:justify-between">
                <Button
                  type="button"
                  variant="destructive"
                  onClick={() => setDeleteOpen(true)}
                  disabled={isSaving || isDeleting}
                >
                  Delete
                </Button>
                <Button type="submit" disabled={isSaving || isDeleting}>
                  {isSaving ? "Saving..." : "Save Changes"}
                </Button>
              </DialogFooter>
            </form>
          ) : (
            /* Read-only view for transfer/investment */
            <div className="flex flex-col gap-3">
              <div className="flex items-center gap-2">
                <Label>Type</Label>
                <Badge variant="secondary" className="capitalize">
                  {transaction?.type}
                </Badge>
              </div>
              <div className="flex items-center justify-between">
                <Label>Amount</Label>
                <span className="text-sm font-medium">
                  {transaction
                    ? formatCurrency(
                        transaction.amount,
                        transaction.account?.currency
                      )
                    : ""}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <Label>Account</Label>
                <span className="text-sm">
                  {transaction?.account?.name ?? "—"}
                </span>
              </div>
              {transaction?.to_account && (
                <div className="flex items-center justify-between">
                  <Label>To Account</Label>
                  <span className="text-sm">
                    {transaction.to_account.name}
                  </span>
                </div>
              )}
              <div className="flex items-center justify-between">
                <Label>Description</Label>
                <span className="text-sm">
                  {transaction?.description || "—"}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <Label>Date</Label>
                <span className="text-sm">
                  {transaction ? formatDate(transaction.date) : ""}
                </span>
              </div>

              <DialogFooter>
                <Button
                  variant="destructive"
                  onClick={() => setDeleteOpen(true)}
                  disabled={isDeleting}
                >
                  Delete Transaction
                </Button>
              </DialogFooter>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Transaction</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete this transaction? This will reverse
              its balance impact on the account.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={handleDelete}
              disabled={isDeleting}
            >
              {isDeleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
