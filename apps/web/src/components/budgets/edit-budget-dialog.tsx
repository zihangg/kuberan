"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import { useUpdateBudget } from "@/hooks/use-budgets";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
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
import { formatDate } from "@/lib/format";
import type { Budget, BudgetPeriod } from "@/types/models";

function getErrorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
    if (error.code === "BUDGET_NOT_FOUND") {
      return "Budget not found";
    }
    return error.message;
  }
  return "An unexpected error occurred";
}

interface EditBudgetDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  budget: Budget | null;
}

export function EditBudgetDialog({
  open,
  onOpenChange,
  budget,
}: EditBudgetDialogProps) {
  const [name, setName] = useState("");
  const [amount, setAmount] = useState(0);
  const [period, setPeriod] = useState<BudgetPeriod>("monthly");
  const [endDate, setEndDate] = useState("");
  const [error, setError] = useState("");

  const updateBudget = useUpdateBudget(budget?.id ?? "");
  const isSubmitting = updateBudget.isPending;

  // Sync form state when budget changes
  useEffect(() => {
    if (budget) {
      setName(budget.name);
      setAmount(budget.amount);
      setPeriod(budget.period);
      setEndDate(
        budget.end_date ? budget.end_date.split("T")[0] : ""
      );
      setError("");
    }
  }, [budget]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!budget) return;
    setError("");

    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("Name is required");
      return;
    }
    if (trimmedName.length > 100) {
      setError("Name must be 100 characters or less");
      return;
    }
    if (amount <= 0) {
      setError("Amount must be greater than zero");
      return;
    }

    // Build payload with only changed fields
    const payload: Record<string, unknown> = {};
    if (trimmedName !== budget.name) payload.name = trimmedName;
    if (amount !== budget.amount) payload.amount = amount;
    if (period !== budget.period) payload.period = period;
    const origEndDate = budget.end_date ? budget.end_date.split("T")[0] : "";
    if (endDate !== origEndDate) {
      payload.end_date = endDate ? new Date(endDate).toISOString() : null;
    }

    if (Object.keys(payload).length === 0) {
      onOpenChange(false);
      return;
    }

    updateBudget.mutate(payload, {
      onSuccess: (updated) => {
        toast.success(`Budget "${updated.name}" updated`);
        onOpenChange(false);
      },
      onError: (err) => setError(getErrorMessage(err)),
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Edit Budget</DialogTitle>
          <DialogDescription>Update budget details.</DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-5">
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-budget-name">Name</Label>
            <Input
              id="edit-budget-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={isSubmitting}
              maxLength={100}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>Category</Label>
            <Badge variant="outline" className="w-fit">
              {budget?.category?.name ?? "Unknown"}
            </Badge>
          </div>

          <div className="flex flex-col gap-2">
            <Label>Start Date</Label>
            <span className="text-sm text-muted-foreground">
              {budget?.start_date ? formatDate(budget.start_date) : "-"}
            </span>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-budget-amount">Amount</Label>
            <CurrencyInput
              id="edit-budget-amount"
              value={amount}
              onChange={setAmount}
              placeholder="0.00"
              disabled={isSubmitting}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-budget-period">Period</Label>
            <Select
              value={period}
              onValueChange={(v) => setPeriod(v as BudgetPeriod)}
              disabled={isSubmitting}
            >
              <SelectTrigger id="edit-budget-period" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="monthly">Monthly</SelectItem>
                <SelectItem value="yearly">Yearly</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-budget-end">End Date (optional)</Label>
            <Input
              id="edit-budget-end"
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              disabled={isSubmitting}
            />
          </div>

          <DialogFooter>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Saving..." : "Save Changes"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
