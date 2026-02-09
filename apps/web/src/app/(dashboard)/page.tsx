"use client";

import { useState } from "react";
import Link from "next/link";
import {
  Wallet,
  ArrowLeftRight,
  Plus,
  ArrowUpRight,
  ArrowDownRight,
  TrendingUp,
} from "lucide-react";
import { useAuth } from "@/hooks/use-auth";
import { useAccounts } from "@/hooks/use-accounts";
import { useAccountTransactions } from "@/hooks/use-transactions";
import { formatCurrency, formatDate } from "@/lib/format";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { CreateTransactionDialog } from "@/components/transactions/create-transaction-dialog";
import type { Account, Transaction, TransactionType } from "@/types/models";

const ACCOUNT_TYPE_LABELS: Record<string, string> = {
  cash: "Cash",
  investment: "Investment",
  debt: "Debt",
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

function DashboardSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-8 w-64" />
      <Skeleton className="h-32 w-full" />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-28" />
        ))}
      </div>
      <Skeleton className="h-64 w-full" />
    </div>
  );
}

function TotalBalanceCard({ accounts }: { accounts: Account[] }) {
  const total = accounts.reduce((sum, a) => sum + a.balance, 0);
  return (
    <Card>
      <CardHeader>
        <CardDescription>Total Balance</CardDescription>
        <CardTitle className="text-3xl">{formatCurrency(total)}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-muted-foreground">
          Across {accounts.length} account{accounts.length !== 1 ? "s" : ""}
        </p>
      </CardContent>
    </Card>
  );
}

function AccountCard({ account }: { account: Account }) {
  return (
    <Link href={`/accounts/${account.id}`}>
      <Card className="transition-colors hover:bg-accent/50">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">{account.name}</CardTitle>
            <Badge variant="secondary">
              {ACCOUNT_TYPE_LABELS[account.type] ?? account.type}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <p className="text-2xl font-semibold">
            {formatCurrency(account.balance, account.currency)}
          </p>
        </CardContent>
      </Card>
    </Link>
  );
}

function TransactionRow({ transaction }: { transaction: Transaction }) {
  const config = TRANSACTION_TYPE_CONFIG[transaction.type];
  const Icon = config.icon;
  const isNegative =
    transaction.type === "expense" || transaction.type === "transfer";

  return (
    <div className="flex items-center justify-between py-3">
      <div className="flex items-center gap-3">
        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-muted">
          <Icon className={`h-4 w-4 ${config.color}`} />
        </div>
        <div>
          <p className="text-sm font-medium">
            {transaction.description || config.label}
          </p>
          <p className="text-xs text-muted-foreground">
            {formatDate(transaction.date)}
          </p>
        </div>
      </div>
      <span className={`text-sm font-medium ${config.color}`}>
        {isNegative ? "-" : "+"}
        {formatCurrency(transaction.amount)}
      </span>
    </div>
  );
}

export default function DashboardPage() {
  const { user } = useAuth();
  const [txDialogOpen, setTxDialogOpen] = useState(false);
  const { data: accountsData, isLoading: accountsLoading } = useAccounts();

  const accounts = accountsData?.data ?? [];
  const firstAccountId = accounts.length > 0 ? accounts[0].id : 0;

  const { data: transactionsData, isLoading: transactionsLoading } =
    useAccountTransactions(firstAccountId, { page_size: 5 });

  const transactions = transactionsData?.data ?? [];

  if (accountsLoading) {
    return <DashboardSkeleton />;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">
          {user?.first_name
            ? `Welcome back, ${user.first_name}`
            : "Dashboard"}
        </h1>
        <div className="flex gap-2">
          <Button asChild variant="outline" size="sm">
            <Link href="/accounts">
              <Wallet className="mr-2 h-4 w-4" />
              Add Account
            </Link>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setTxDialogOpen(true)}
          >
            <Plus className="mr-2 h-4 w-4" />
            Add Transaction
          </Button>
        </div>
      </div>

      {accounts.length === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Get Started</CardTitle>
            <CardDescription>
              Create your first account to start tracking your finances.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button asChild>
              <Link href="/accounts">
                <Plus className="mr-2 h-4 w-4" />
                Create Account
              </Link>
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          <TotalBalanceCard accounts={accounts} />

          <div>
            <div className="mb-3 flex items-center justify-between">
              <h2 className="text-lg font-semibold">Accounts</h2>
              {accounts.length > 6 && (
                <Button asChild variant="link" size="sm">
                  <Link href="/accounts">View all</Link>
                </Button>
              )}
            </div>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {accounts.slice(0, 6).map((account) => (
                <AccountCard key={account.id} account={account} />
              ))}
            </div>
          </div>

          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>Recent Transactions</CardTitle>
                {firstAccountId > 0 && (
                  <Button asChild variant="link" size="sm">
                    <Link href={`/accounts/${firstAccountId}`}>View all</Link>
                  </Button>
                )}
              </div>
            </CardHeader>
            <CardContent>
              {transactionsLoading ? (
                <div className="space-y-3">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <Skeleton key={i} className="h-12 w-full" />
                  ))}
                </div>
              ) : transactions.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No transactions yet. Add a transaction to see it here.
                </p>
              ) : (
                <div className="divide-y">
                  {transactions.map((tx) => (
                    <TransactionRow key={tx.id} transaction={tx} />
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </>
      )}

      <CreateTransactionDialog
        open={txDialogOpen}
        onOpenChange={setTxDialogOpen}
      />
    </div>
  );
}
