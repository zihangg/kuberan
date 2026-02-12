"use client";

import { useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useBudgets, useBudgetProgress } from "@/hooks/use-budgets";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardAction,
} from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CreateBudgetDialog } from "@/components/budgets/create-budget-dialog";
import { EditBudgetDialog } from "@/components/budgets/edit-budget-dialog";
import { DeleteBudgetDialog } from "@/components/budgets/delete-budget-dialog";
import { formatCurrency } from "@/lib/format";
import type { Budget, BudgetPeriod } from "@/types/models";

function BudgetProgressBar({ budgetId }: { budgetId: number }) {
  const { data: progress, isLoading } = useBudgetProgress(budgetId);

  if (isLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-3 w-full rounded-full" />
        <Skeleton className="h-4 w-3/4" />
      </div>
    );
  }

  if (!progress) return null;

  const pct = Math.min(progress.percentage, 100);
  const barColor =
    progress.percentage > 90
      ? "bg-red-500"
      : progress.percentage > 75
        ? "bg-amber-500"
        : "bg-emerald-500";

  return (
    <div className="space-y-2">
      <div className="h-3 w-full rounded-full bg-muted">
        <div
          className={`h-3 rounded-full transition-all ${barColor}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <div className="flex justify-between text-sm text-muted-foreground">
        <span>
          {formatCurrency(progress.spent)} / {formatCurrency(progress.budgeted)}{" "}
          ({progress.percentage.toFixed(1)}%)
        </span>
        <span>
          {formatCurrency(Math.abs(progress.remaining))}{" "}
          {progress.remaining >= 0 ? "remaining" : "over"}
        </span>
      </div>
    </div>
  );
}

function BudgetsGridSkeleton() {
  return (
    <div className="grid gap-4 sm:grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <Card key={i}>
          <CardHeader>
            <Skeleton className="h-5 w-1/2" />
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex gap-2">
              <Skeleton className="h-5 w-16" />
              <Skeleton className="h-5 w-16" />
            </div>
            <Skeleton className="h-5 w-24" />
            <Skeleton className="h-3 w-full rounded-full" />
            <Skeleton className="h-4 w-3/4" />
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

export default function BudgetsPage() {
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [periodFilter, setPeriodFilter] = useState<string>("all");
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [editBudget, setEditBudget] = useState<Budget | null>(null);
  const [deleteBudget, setDeleteBudget] = useState<Budget | null>(null);

  const isActive =
    statusFilter === "active"
      ? true
      : statusFilter === "inactive"
        ? false
        : undefined;
  const period =
    periodFilter === "all" ? undefined : (periodFilter as BudgetPeriod);

  const { data, isLoading } = useBudgets({ page, is_active: isActive, period });

  const budgets = data?.data ?? [];
  const totalPages = data?.total_pages ?? 1;
  const currentPage = data?.page ?? 1;

  function handleStatusChange(value: string) {
    setStatusFilter(value);
    setPage(1);
  }

  function handlePeriodChange(value: string) {
    setPeriodFilter(value);
    setPage(1);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Budgets</h1>
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Create Budget
        </Button>
      </div>

      <div className="flex flex-col sm:flex-row flex-wrap items-start sm:items-center gap-3 sm:gap-4">
        <Tabs value={statusFilter} onValueChange={handleStatusChange}>
          <TabsList>
            <TabsTrigger value="all">All</TabsTrigger>
            <TabsTrigger value="active">Active</TabsTrigger>
            <TabsTrigger value="inactive">Inactive</TabsTrigger>
          </TabsList>
        </Tabs>
        <Select value={periodFilter} onValueChange={handlePeriodChange}>
          <SelectTrigger className="w-full sm:w-[140px]">
            <SelectValue placeholder="Period" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Periods</SelectItem>
            <SelectItem value="monthly">Monthly</SelectItem>
            <SelectItem value="yearly">Yearly</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {isLoading ? (
        <BudgetsGridSkeleton />
      ) : budgets.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed p-12 text-center">
          <h3 className="text-lg font-semibold">No budgets yet</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            Create a budget to start tracking your spending.
          </p>
          <Button
            className="mt-4"
            size="sm"
            onClick={() => setCreateOpen(true)}
          >
            <Plus className="mr-2 h-4 w-4" />
            Create Budget
          </Button>
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
            {budgets.map((budget) => (
              <Card key={budget.id}>
                <CardHeader>
                  <CardTitle>{budget.name}</CardTitle>
                  <CardAction>
                    <div className="flex gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setEditBudget(budget)}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-destructive"
                        onClick={() => setDeleteBudget(budget)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </CardAction>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex flex-wrap gap-2">
                    {budget.category && (
                      <Badge variant="outline">{budget.category.name}</Badge>
                    )}
                    <Badge variant="secondary">
                      {budget.period === "monthly" ? "Monthly" : "Yearly"}
                    </Badge>
                  </div>
                  <p className="text-lg font-semibold">
                    {formatCurrency(budget.amount)}
                  </p>
                  <BudgetProgressBar budgetId={budget.id} />
                </CardContent>
              </Card>
            ))}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={currentPage <= 1}
              >
                Previous
              </Button>
              <span className="text-sm text-muted-foreground">
                Page {currentPage} of {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={currentPage >= totalPages}
              >
                Next
              </Button>
            </div>
          )}
        </>
      )}

      <CreateBudgetDialog open={createOpen} onOpenChange={setCreateOpen} />
      <EditBudgetDialog
        open={!!editBudget}
        onOpenChange={(open) => {
          if (!open) setEditBudget(null);
        }}
        budget={editBudget}
      />
      <DeleteBudgetDialog
        open={!!deleteBudget}
        onOpenChange={(open) => {
          if (!open) setDeleteBudget(null);
        }}
        budget={deleteBudget}
      />
    </div>
  );
}
