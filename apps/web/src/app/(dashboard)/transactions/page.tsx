"use client";

import { useState } from "react";
import {
  ArrowDownRight,
  ArrowLeftRight,
  ArrowUpRight,
  ChevronLeft,
  ChevronRight,
  Plus,
  TrendingUp,
} from "lucide-react";
import { useAccounts } from "@/hooks/use-accounts";
import { useTransactions } from "@/hooks/use-transactions";
import { useCategories } from "@/hooks/use-categories";
import { formatCurrency, formatDate } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { CreateTransactionDialog } from "@/components/transactions/create-transaction-dialog";
import { CreateTransferDialog } from "@/components/transactions/create-transfer-dialog";
import { EditTransactionDialog } from "@/components/transactions/edit-transaction-dialog";
import type { Transaction, TransactionType } from "@/types/models";
import type { UserTransactionFilters } from "@/types/api";

const TRANSACTION_TYPE_CONFIG: Record<
  TransactionType,
  { label: string; color: string; icon: typeof ArrowUpRight }
> = {
  income: { label: "Income", color: "text-green-600", icon: ArrowUpRight },
  expense: { label: "Expense", color: "text-red-600", icon: ArrowDownRight },
  transfer: {
    label: "Transfer",
    color: "text-blue-600",
    icon: ArrowLeftRight,
  },
  investment: {
    label: "Investment",
    color: "text-purple-600",
    icon: TrendingUp,
  },
};

const PAGE_SIZE = 20;

function TransactionsTableSkeleton() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-10 w-full" />
      {Array.from({ length: 8 }).map((_, i) => (
        <Skeleton key={i} className="h-12 w-full" />
      ))}
    </div>
  );
}

function TransactionRow({
  transaction,
  accountName,
  onClick,
}: {
  transaction: Transaction;
  accountName: string;
  onClick?: () => void;
}) {
  const config = TRANSACTION_TYPE_CONFIG[transaction.type];
  const Icon = config.icon;
  const isNegative =
    transaction.type === "expense" || transaction.type === "transfer";

  return (
    <TableRow className={onClick ? "cursor-pointer" : ""} onClick={onClick}>
      <TableCell className="text-muted-foreground">
        {formatDate(transaction.date)}
      </TableCell>
      <TableCell>
        <div className="flex items-center gap-2">
          <div className="flex h-6 w-6 items-center justify-center rounded-full bg-muted">
            <Icon className={`h-3 w-3 ${config.color}`} />
          </div>
          {transaction.description || config.label}
        </div>
      </TableCell>
      <TableCell className="text-muted-foreground">{accountName}</TableCell>
      <TableCell>
        <Badge variant="outline" className={config.color}>
          {config.label}
        </Badge>
      </TableCell>
      <TableCell>
        {transaction.category?.name ?? (
          <span className="text-muted-foreground">-</span>
        )}
      </TableCell>
      <TableCell className={`text-right font-medium ${config.color}`}>
        {isNegative ? "-" : "+"}
        {formatCurrency(transaction.amount)}
      </TableCell>
    </TableRow>
  );
}

export default function TransactionsPage() {
  const [page, setPage] = useState(1);
  const [accountFilter, setAccountFilter] = useState<string>("all");
  const [typeFilter, setTypeFilter] = useState<string>("all");
  const [categoryFilter, setCategoryFilter] = useState<string>("all");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [txDialogOpen, setTxDialogOpen] = useState(false);
  const [transferDialogOpen, setTransferDialogOpen] = useState(false);
  const [editTxOpen, setEditTxOpen] = useState(false);
  const [selectedTransaction, setSelectedTransaction] = useState<Transaction | null>(null);

  const filters: UserTransactionFilters = {
    page,
    page_size: PAGE_SIZE,
    account_id:
      accountFilter !== "all" ? Number(accountFilter) : undefined,
    type:
      typeFilter !== "all" ? (typeFilter as TransactionType) : undefined,
    category_id:
      categoryFilter !== "all" ? Number(categoryFilter) : undefined,
    from_date: fromDate || undefined,
    to_date: toDate || undefined,
  };

  const { data, isLoading } = useTransactions(filters);
  const { data: accountsData } = useAccounts({ page_size: 100 });
  const { data: categoriesData } = useCategories({ page_size: 100 });

  const transactions = data?.data ?? [];
  const totalPages = data?.total_pages ?? 1;
  const accounts = accountsData?.data ?? [];
  const categories = categoriesData?.data ?? [];

  // Build account name lookup
  const accountNameMap = new Map(accounts.map((a) => [a.id, a.name]));

  const hasActiveFilters =
    accountFilter !== "all" ||
    typeFilter !== "all" ||
    categoryFilter !== "all" ||
    fromDate !== "" ||
    toDate !== "";

  function resetPage() {
    setPage(1);
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Transactions</h1>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setTransferDialogOpen(true)}
          >
            <ArrowLeftRight className="mr-2 h-4 w-4" />
            Transfer
          </Button>
          <Button size="sm" onClick={() => setTxDialogOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Add Transaction
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <div className="space-y-1">
          <Label className="text-xs">Account</Label>
          <Select
            value={accountFilter}
            onValueChange={(val) => {
              setAccountFilter(val);
              resetPage();
            }}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Accounts</SelectItem>
              {accounts.map((a) => (
                <SelectItem key={a.id} value={String(a.id)}>
                  {a.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1">
          <Label className="text-xs">Type</Label>
          <Select
            value={typeFilter}
            onValueChange={(val) => {
              setTypeFilter(val);
              resetPage();
            }}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Types</SelectItem>
              <SelectItem value="income">Income</SelectItem>
              <SelectItem value="expense">Expense</SelectItem>
              <SelectItem value="transfer">Transfer</SelectItem>
              <SelectItem value="investment">Investment</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1">
          <Label className="text-xs">Category</Label>
          <Select
            value={categoryFilter}
            onValueChange={(val) => {
              setCategoryFilter(val);
              resetPage();
            }}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Categories</SelectItem>
              {categories.map((cat) => (
                <SelectItem key={cat.id} value={String(cat.id)}>
                  {cat.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1">
          <Label className="text-xs">From Date</Label>
          <Input
            type="date"
            value={fromDate}
            onChange={(e) => {
              setFromDate(e.target.value);
              resetPage();
            }}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">To Date</Label>
          <Input
            type="date"
            value={toDate}
            onChange={(e) => {
              setToDate(e.target.value);
              resetPage();
            }}
          />
        </div>
      </div>

      {/* Content */}
      {isLoading ? (
        <TransactionsTableSkeleton />
      ) : transactions.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed p-12 text-center">
          <h3 className="text-lg font-semibold">No transactions found</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            {hasActiveFilters
              ? "No transactions match your filters. Try adjusting them."
              : "Add your first transaction to get started."}
          </p>
          {!hasActiveFilters && (
            <Button
              className="mt-4"
              size="sm"
              onClick={() => setTxDialogOpen(true)}
            >
              <Plus className="mr-2 h-4 w-4" />
              Add Transaction
            </Button>
          )}
        </div>
      ) : (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Date</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Account</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Category</TableHead>
                <TableHead className="text-right">Amount</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {transactions.map((tx) => (
                <TransactionRow
                  key={tx.id}
                  transaction={tx}
                  accountName={
                    accountNameMap.get(tx.account_id) ?? `Account #${tx.account_id}`
                  }
                  onClick={() => {
                    setSelectedTransaction(tx);
                    setEditTxOpen(true);
                  }}
                />
              ))}
            </TableBody>
          </Table>

          {totalPages > 1 && (
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">
                Page {page} of {totalPages}
              </span>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => p - 1)}
                >
                  <ChevronLeft className="h-4 w-4" />
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
                >
                  Next
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      <CreateTransactionDialog
        open={txDialogOpen}
        onOpenChange={setTxDialogOpen}
      />
      <CreateTransferDialog
        open={transferDialogOpen}
        onOpenChange={setTransferDialogOpen}
      />
      <EditTransactionDialog
        open={editTxOpen}
        onOpenChange={setEditTxOpen}
        transaction={selectedTransaction}
      />
    </div>
  );
}
