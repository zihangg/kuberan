"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  ChevronLeft,
  ChevronRight,
  DollarSign,
  Hash,
  TrendingUp,
  TrendingDown,
  Wallet,
  BarChart3,
  ShoppingCart,
} from "lucide-react";

import {
  useInvestment,
  useInvestmentTransactions,
} from "@/hooks/use-investments";
import { formatCurrency, formatDate } from "@/lib/format";
import { RecordBuyDialog } from "@/components/investments/record-buy-dialog";
import { RecordSellDialog } from "@/components/investments/record-sell-dialog";
import { RecordDividendDialog } from "@/components/investments/record-dividend-dialog";
import { RecordSplitDialog } from "@/components/investments/record-split-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { AssetType, InvestmentTransactionType } from "@/types/models";

const ASSET_TYPE_LABELS: Record<AssetType, string> = {
  stock: "Stock",
  etf: "ETF",
  bond: "Bond",
  crypto: "Crypto",
  reit: "REIT",
};

const TX_TYPE_CONFIG: Record<
  InvestmentTransactionType,
  { label: string; color: string }
> = {
  buy: { label: "Buy", color: "text-green-600" },
  sell: { label: "Sell", color: "text-red-600" },
  dividend: { label: "Dividend", color: "text-blue-600" },
  split: { label: "Split", color: "text-purple-600" },
  transfer: { label: "Transfer", color: "text-orange-600" },
};

const PAGE_SIZE = 20;

function InvestmentDetailSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-6 w-32" />
      <div className="space-y-2">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-6 w-48" />
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="h-24" />
        ))}
      </div>
      <Skeleton className="h-64" />
    </div>
  );
}

export default function InvestmentDetailPage() {
  const params = useParams();
  const investmentId = Number(params.id);

  const [txPage, setTxPage] = useState(1);
  const [buyOpen, setBuyOpen] = useState(false);
  const [sellOpen, setSellOpen] = useState(false);
  const [dividendOpen, setDividendOpen] = useState(false);
  const [splitOpen, setSplitOpen] = useState(false);

  const { data: investment, isLoading } = useInvestment(investmentId);
  const { data: txData, isLoading: txLoading } = useInvestmentTransactions(
    investmentId,
    { page: txPage, page_size: PAGE_SIZE }
  );

  const transactions = txData?.data ?? [];
  const txTotalPages = txData?.total_pages ?? 1;

  if (isLoading) {
    return <InvestmentDetailSkeleton />;
  }

  if (!investment) {
    return (
      <div className="space-y-6">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/investments">
            <ArrowLeft className="mr-1 h-4 w-4" />
            Back to Investments
          </Link>
        </Button>
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed p-12 text-center">
          <h3 className="text-lg font-semibold">Investment not found</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            The investment you are looking for does not exist or has been
            removed.
          </p>
        </div>
      </div>
    );
  }

  const isClosed = investment.quantity === 0;
  const marketValue = Math.round(investment.quantity * investment.current_price);
  const gainLoss = marketValue - investment.cost_basis;
  const gainLossPct =
    investment.cost_basis > 0 ? (gainLoss / investment.cost_basis) * 100 : 0;
  const isPositive = gainLoss >= 0;
  const hasRealizedGainLoss = investment.realized_gain_loss !== 0;
  const isRealizedPositive = investment.realized_gain_loss >= 0;

  return (
    <div className="space-y-6">
      {/* Back button */}
      <Button variant="ghost" size="sm" asChild>
        <Link href="/investments">
          <ArrowLeft className="mr-1 h-4 w-4" />
          Back to Investments
        </Link>
      </Button>

      {/* Header */}
      <div className="space-y-1">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold font-mono">
            {investment.security.symbol}
          </h1>
          <Badge variant="outline">
            {ASSET_TYPE_LABELS[investment.security.asset_type]}
          </Badge>
          {isClosed && (
            <Badge variant="secondary">Position Closed</Badge>
          )}
        </div>
        <p className="text-lg text-muted-foreground">
          {investment.security.name}
        </p>
      </div>

      {/* Stat cards */}
      {isClosed ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardDescription>Realized Gain / Loss</CardDescription>
              {isRealizedPositive ? (
                <TrendingUp className="h-4 w-4 text-green-600" />
              ) : (
                <TrendingDown className="h-4 w-4 text-red-600" />
              )}
            </CardHeader>
            <CardContent>
              <p
                className={`text-2xl font-bold font-mono whitespace-nowrap ${isRealizedPositive ? "text-green-600" : "text-red-600"}`}
              >
                {isRealizedPositive ? "+" : "-"}{" "}
                {formatCurrency(Math.abs(investment.realized_gain_loss))}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardDescription>Current Price</CardDescription>
              <DollarSign className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold font-mono">
                {formatCurrency(investment.current_price)}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardDescription>Account</CardDescription>
              <Wallet className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <Link
                href={`/accounts/${investment.account_id}`}
                className="text-lg font-semibold text-primary hover:underline"
              >
                {investment.account?.name ?? `Account #${investment.account_id}`}
              </Link>
            </CardContent>
          </Card>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardDescription>Current Price</CardDescription>
              <DollarSign className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold font-mono">
                {formatCurrency(investment.current_price)}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardDescription>Quantity</CardDescription>
              <Hash className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold font-mono">
                {investment.quantity.toLocaleString(undefined, {
                  maximumFractionDigits: 6,
                })}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardDescription>Market Value</CardDescription>
              <BarChart3 className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold font-mono">
                {formatCurrency(marketValue)}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardDescription>Cost Basis</CardDescription>
              <ShoppingCart className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold font-mono">
                {formatCurrency(investment.cost_basis)}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardDescription>{hasRealizedGainLoss ? "Unrealized Gain / Loss" : "Gain / Loss"}</CardDescription>
              {isPositive ? (
                <TrendingUp className="h-4 w-4 text-green-600" />
              ) : (
                <TrendingDown className="h-4 w-4 text-red-600" />
              )}
            </CardHeader>
            <CardContent>
              <p
                className={`text-2xl font-bold font-mono whitespace-nowrap ${isPositive ? "text-green-600" : "text-red-600"}`}
              >
                {isPositive ? "+" : "-"}{" "}
                {formatCurrency(Math.abs(gainLoss))}
              </p>
              <p
                className={`text-sm ${isPositive ? "text-green-600" : "text-red-600"}`}
              >
                {isPositive ? "+" : "-"}
                {Math.abs(gainLossPct).toFixed(2)}%
              </p>
            </CardContent>
          </Card>

          {hasRealizedGainLoss && (
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardDescription>Realized Gain / Loss</CardDescription>
                {isRealizedPositive ? (
                  <TrendingUp className="h-4 w-4 text-green-600" />
                ) : (
                  <TrendingDown className="h-4 w-4 text-red-600" />
                )}
              </CardHeader>
              <CardContent>
                <p
                  className={`text-2xl font-bold font-mono whitespace-nowrap ${isRealizedPositive ? "text-green-600" : "text-red-600"}`}
                >
                  {isRealizedPositive ? "+" : "-"}{" "}
                  {formatCurrency(Math.abs(investment.realized_gain_loss))}
                </p>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardDescription>Account</CardDescription>
              <Wallet className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <Link
                href={`/accounts/${investment.account_id}`}
                className="text-lg font-semibold text-primary hover:underline"
              >
                {investment.account?.name ?? `Account #${investment.account_id}`}
              </Link>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Action buttons */}
      <div className="flex flex-wrap gap-2">
        <Button size="sm" variant="default" onClick={() => setBuyOpen(true)}>
          Buy
        </Button>
        <Button size="sm" variant="outline" onClick={() => setSellOpen(true)} disabled={isClosed}>
          Sell
        </Button>
        <Button size="sm" variant="outline" onClick={() => setDividendOpen(true)}>
          Dividend
        </Button>
        <Button size="sm" variant="outline" onClick={() => setSplitOpen(true)} disabled={isClosed}>
          Split
        </Button>
      </div>

      <RecordBuyDialog
        investmentId={investmentId}
        open={buyOpen}
        onOpenChange={setBuyOpen}
      />
      <RecordSellDialog
        investmentId={investmentId}
        currentQuantity={investment.quantity}
        open={sellOpen}
        onOpenChange={setSellOpen}
      />
      <RecordDividendDialog
        investmentId={investmentId}
        open={dividendOpen}
        onOpenChange={setDividendOpen}
      />
      <RecordSplitDialog
        investmentId={investmentId}
        open={splitOpen}
        onOpenChange={setSplitOpen}
      />

      {/* Transaction history */}
      <Card>
        <CardHeader>
          <CardTitle>Transaction History</CardTitle>
          <CardDescription>
            All transactions for {investment.security.symbol}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {txLoading ? (
            <div className="space-y-3">
              <Skeleton className="h-10 w-full" />
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : transactions.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-lg border border-dashed p-12 text-center">
              <h3 className="text-lg font-semibold">No transactions</h3>
              <p className="mt-1 text-sm text-muted-foreground">
                No transaction history available for this investment.
              </p>
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Date</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead className="text-right">Quantity</TableHead>
                    <TableHead className="text-right">Price/Unit</TableHead>
                    <TableHead className="text-right">Total</TableHead>
                    <TableHead className="text-right">Fee</TableHead>
                    <TableHead className="text-right">Realized P&L</TableHead>
                    <TableHead>Notes</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {transactions.map((tx) => {
                    const config = TX_TYPE_CONFIG[tx.type];
                    return (
                      <TableRow key={tx.id}>
                        <TableCell className="text-muted-foreground">
                          {formatDate(tx.date)}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className={config.color}>
                            {config.label}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right font-mono">
                          {tx.type === "split"
                            ? `${tx.split_ratio}:1`
                            : tx.quantity.toLocaleString(undefined, {
                                maximumFractionDigits: 6,
                              })}
                        </TableCell>
                        <TableCell className="text-right font-mono">
                          {tx.type === "split" || tx.type === "dividend"
                            ? "-"
                            : formatCurrency(tx.price_per_unit)}
                        </TableCell>
                        <TableCell
                          className={`text-right font-mono font-medium ${config.color}`}
                        >
                          {tx.type === "split"
                            ? "-"
                            : formatCurrency(tx.total_amount)}
                        </TableCell>
                        <TableCell className="text-right font-mono text-muted-foreground">
                          {tx.fee > 0 ? formatCurrency(tx.fee) : "-"}
                        </TableCell>
                        <TableCell className="text-right font-mono font-medium">
                          {tx.type === "sell" ? (
                            <span className={tx.realized_gain_loss >= 0 ? "text-green-600" : "text-red-600"}>
                              {tx.realized_gain_loss >= 0 ? "+" : ""}
                              {formatCurrency(tx.realized_gain_loss)}
                            </span>
                          ) : (
                            "-"
                          )}
                        </TableCell>
                        <TableCell className="max-w-[200px] truncate text-muted-foreground">
                          {tx.notes || "-"}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>

              {txTotalPages > 1 && (
                <div className="mt-4 flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">
                    Page {txPage} of {txTotalPages}
                  </span>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={txPage <= 1}
                      onClick={() => setTxPage((p) => p - 1)}
                    >
                      <ChevronLeft className="h-4 w-4" />
                      Previous
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={txPage >= txTotalPages}
                      onClick={() => setTxPage((p) => p + 1)}
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
    </div>
  );
}
