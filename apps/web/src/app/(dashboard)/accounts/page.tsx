"use client";

import { useState } from "react";
import Link from "next/link";
import { Plus } from "lucide-react";
import { useAccounts } from "@/hooks/use-accounts";
import { formatCurrency, formatDate } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { CreateAccountDialog } from "@/components/accounts/create-account-dialog";
import type { Account, AccountType } from "@/types/models";

const ACCOUNT_TYPE_LABELS: Record<AccountType, string> = {
  cash: "Cash",
  investment: "Investment",
  debt: "Debt",
  credit_card: "Credit Card",
};

function AccountCard({ account }: { account: Account }) {
  return (
    <Link href={`/accounts/${account.id}`}>
      <Card className="transition-colors hover:bg-accent/50 cursor-pointer">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between gap-2">
            <CardTitle className="text-base truncate">{account.name}</CardTitle>
            <Badge variant="secondary" className="shrink-0">
              {ACCOUNT_TYPE_LABELS[account.type]}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-2">
          <p className="text-2xl font-semibold">
            {formatCurrency(account.balance, account.currency)}
          </p>
          <div className="flex items-center justify-between text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <Badge variant={account.is_active ? "outline" : "secondary"} className="text-xs">
                {account.is_active ? "Active" : "Inactive"}
              </Badge>
              <span className="text-xs">{account.currency}</span>
            </div>
            <span className="text-xs">{formatDate(account.created_at)}</span>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}

function AccountsTableSkeleton() {
  return (
    <>
      {/* Mobile: Card skeletons */}
      <div className="md:hidden grid gap-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-32 w-full rounded-lg" />
        ))}
      </div>

      {/* Desktop: Table skeleton */}
      <div className="hidden md:block space-y-3">
        <Skeleton className="h-10 w-full" />
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    </>
  );
}

function AccountRow({ account }: { account: Account }) {
  return (
    <TableRow className="cursor-pointer">
      <TableCell>
        <Link
          href={`/accounts/${account.id}`}
          className="block font-medium hover:underline"
        >
          {account.name}
        </Link>
      </TableCell>
      <TableCell>
        <Badge variant="secondary">
          {ACCOUNT_TYPE_LABELS[account.type]}
        </Badge>
      </TableCell>
      <TableCell className="text-right font-medium">
        {formatCurrency(account.balance, account.currency)}
      </TableCell>
      <TableCell>{account.currency}</TableCell>
      <TableCell>
        <Badge variant={account.is_active ? "outline" : "secondary"}>
          {account.is_active ? "Active" : "Inactive"}
        </Badge>
      </TableCell>
      <TableCell className="text-muted-foreground">
        {formatDate(account.created_at)}
      </TableCell>
    </TableRow>
  );
}

export default function AccountsPage() {
  const [dialogOpen, setDialogOpen] = useState(false);
  const { data, isLoading } = useAccounts();

  const accounts = data?.data ?? [];
  const totalPages = data?.total_pages ?? 1;
  const currentPage = data?.page ?? 1;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Accounts</h1>
        <Button size="sm" onClick={() => setDialogOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Create Account
        </Button>
      </div>

      {isLoading ? (
        <AccountsTableSkeleton />
      ) : accounts.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed p-12 text-center">
          <h3 className="text-lg font-semibold">No accounts yet</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            Create your first account to get started.
          </p>
          <Button className="mt-4" size="sm" onClick={() => setDialogOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create Account
          </Button>
        </div>
      ) : (
        <>
          {/* Mobile: Card Grid */}
          <div className="md:hidden">
            <div className="grid gap-3">
              {accounts.map((account) => (
                <AccountCard key={account.id} account={account} />
              ))}
            </div>
          </div>

          {/* Desktop: Table */}
          <div className="hidden md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead className="text-right">Balance</TableHead>
                  <TableHead>Currency</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {accounts.map((account) => (
                  <AccountRow key={account.id} account={account} />
                ))}
              </TableBody>
            </Table>
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2">
              <span className="text-sm text-muted-foreground">
                Page {currentPage} of {totalPages}
              </span>
            </div>
          )}
        </>
      )}

      <CreateAccountDialog open={dialogOpen} onOpenChange={setDialogOpen} />
    </div>
  );
}
