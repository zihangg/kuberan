"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  ArrowLeftRight,
  ArrowUpRight,
  ArrowDownRight,
  Pencil,
  Plus,
  TrendingUp,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { useAccount } from "@/hooks/use-accounts";
import { useAccountTransactions } from "@/hooks/use-transactions";
import { useCategories } from "@/hooks/use-categories";
import { formatCurrency, formatDate } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
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
import { EditAccountDialog } from "@/components/accounts/edit-account-dialog";
import { EditTransactionDialog } from "@/components/transactions/edit-transaction-dialog";
import type { Transaction, TransactionType } from "@/types/models";
import type { TransactionFilters } from "@/types/api";

const ACCOUNT_TYPE_LABELS: Record<string, string> = {
  cash: "Cash",
  investment: "Investment",
  debt: "Debt",
  credit_card: "Credit Card",
};

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

function AccountDetailSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-6 w-32" />
      <div className="space-y-2">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-4 w-96" />
      </div>
      <Skeleton className="h-10 w-full" />
      <div className="space-y-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    </div>
  );
}

function TransactionsTableRow({
  transaction,
  onClick,
}: {
  transaction: Transaction;
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

export default function AccountDetailPage() {
  const params = useParams();
  const router = useRouter();
  const accountId = Number(params.id);

  const [page, setPage] = useState(1);
  const [filters, setFilters] = useState<TransactionFilters>({});
  const [txDialogOpen, setTxDialogOpen] = useState(false);
  const [transferDialogOpen, setTransferDialogOpen] = useState(false);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [editTxOpen, setEditTxOpen] = useState(false);
  const [selectedTransaction, setSelectedTransaction] = useState<Transaction | null>(null);

  const { data: account, isLoading: accountLoading } = useAccount(accountId);
  const { data: transactionsData, isLoading: transactionsLoading } =
    useAccountTransactions(accountId, {
      ...filters,
      page,
      page_size: PAGE_SIZE,
    });
  const { data: categoriesData } = useCategories({ page_size: 100 });

  const transactions = transactionsData?.data ?? [];
  const totalPages = transactionsData?.total_pages ?? 1;
  const totalItems = transactionsData?.total_items ?? 0;
  const categories = categoriesData?.data ?? [];

  if (accountLoading) {
    return <AccountDetailSkeleton />;
  }

  if (!account) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <h2 className="text-lg font-semibold">Account not found</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          This account may have been deleted or you don&apos;t have access.
        </p>
        <Button
          className="mt-4"
          variant="outline"
          onClick={() => router.push("/accounts")}
        >
          Back to Accounts
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Back navigation */}
      <Button asChild variant="ghost" size="sm">
        <Link href="/accounts">
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back to Accounts
        </Link>
      </Button>

      {/* Account header */}
      <div>
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold">{account.name}</h1>
          <Badge variant="secondary">
            {ACCOUNT_TYPE_LABELS[account.type] ?? account.type}
          </Badge>
          <Badge variant={account.is_active ? "outline" : "secondary"}>
            {account.is_active ? "Active" : "Inactive"}
          </Badge>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={() => setEditDialogOpen(true)}
          >
            <Pencil className="h-4 w-4" />
            <span className="sr-only">Edit account</span>
          </Button>
        </div>
        <p className="mt-1 text-3xl font-semibold">
          {formatCurrency(account.balance, account.currency)}
        </p>
        {account.description && (
          <p className="mt-1 text-sm text-muted-foreground">
            {account.description}
          </p>
        )}
        {account.type === "investment" && (
          <div className="mt-2 flex gap-4 text-sm text-muted-foreground">
            {account.broker && <span>Broker: {account.broker}</span>}
            {account.account_number && (
              <span>Account: {account.account_number}</span>
            )}
          </div>
        )}
      </div>

      {/* Transactions section */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Transactions</CardTitle>
              {totalItems > 0 && (
                <CardDescription>
                  {totalItems} transaction{totalItems !== 1 ? "s" : ""}
                </CardDescription>
              )}
            </div>
            <div className="flex gap-2">
              {account.type === "cash" && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setTransferDialogOpen(true)}
                >
                  <ArrowLeftRight className="mr-2 h-4 w-4" />
                  Transfer
                </Button>
              )}
              <Button size="sm" onClick={() => setTxDialogOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                Add Transaction
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Filters */}
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div className="space-y-1">
              <Label className="text-xs">From Date</Label>
              <Input
                type="date"
                value={filters.from_date ?? ""}
                onChange={(e) => {
                  setFilters((f) => ({
                    ...f,
                    from_date: e.target.value || undefined,
                  }));
                  setPage(1);
                }}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">To Date</Label>
              <Input
                type="date"
                value={filters.to_date ?? ""}
                onChange={(e) => {
                  setFilters((f) => ({
                    ...f,
                    to_date: e.target.value || undefined,
                  }));
                  setPage(1);
                }}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Type</Label>
              <Select
                value={filters.type ?? "all"}
                onValueChange={(val) => {
                  setFilters((f) => ({
                    ...f,
                    type:
                      val === "all"
                        ? undefined
                        : (val as TransactionType),
                  }));
                  setPage(1);
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
                value={
                  filters.category_id ? String(filters.category_id) : "all"
                }
                onValueChange={(val) => {
                  setFilters((f) => ({
                    ...f,
                    category_id:
                      val === "all" ? undefined : Number(val),
                  }));
                  setPage(1);
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
          </div>

          {/* Transactions table */}
          {transactionsLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : transactions.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-lg border border-dashed p-8 text-center">
              <h3 className="text-lg font-semibold">No transactions yet</h3>
              <p className="mt-1 text-sm text-muted-foreground">
                {Object.values(filters).some((v) => v !== undefined)
                  ? "No transactions match your filters. Try adjusting them."
                  : "Add your first transaction to get started."}
              </p>
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Date</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Category</TableHead>
                    <TableHead className="text-right">Amount</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {transactions.map((tx) => (
                    <TransactionsTableRow
                      key={tx.id}
                      transaction={tx}
                      onClick={() => {
                        setSelectedTransaction(tx);
                        setEditTxOpen(true);
                      }}
                    />
                  ))}
                </TableBody>
              </Table>

              {/* Pagination */}
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
        </CardContent>
      </Card>

      {/* Investment accounts: placeholder for investments tab */}
      {account.type === "investment" && (
        <Card>
          <CardHeader>
            <CardTitle>Investments</CardTitle>
            <CardDescription>
              Investment holdings for this account will be shown here.
            </CardDescription>
          </CardHeader>
        </Card>
      )}

      <CreateTransactionDialog
        open={txDialogOpen}
        onOpenChange={setTxDialogOpen}
        defaultAccountId={accountId}
      />

      <CreateTransferDialog
        open={transferDialogOpen}
        onOpenChange={setTransferDialogOpen}
        defaultFromAccountId={accountId}
      />

      <EditAccountDialog
        open={editDialogOpen}
        onOpenChange={setEditDialogOpen}
        account={account}
      />
      <EditTransactionDialog
        open={editTxOpen}
        onOpenChange={setEditTxOpen}
        transaction={selectedTransaction}
      />
    </div>
  );
}
