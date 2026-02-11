"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  TrendingUp,
  TrendingDown,
  DollarSign,
  BarChart3,
  PieChart,
  Landmark,
  ChevronLeft,
  ChevronRight,
  List,
} from "lucide-react";
import { AreaChart, Area, XAxis, YAxis, CartesianGrid } from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  type ChartConfig,
} from "@/components/ui/chart";
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
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { usePortfolio, useAllInvestments } from "@/hooks/use-investments";
import { usePortfolioSnapshots } from "@/hooks/use-portfolio-snapshots";
import { formatCurrency, formatDate, formatPercentage } from "@/lib/format";
import type { AssetType } from "@/types/models";

const ASSET_TYPE_LABELS: Record<AssetType, string> = {
  stock: "Stocks",
  etf: "ETFs",
  bond: "Bonds",
  crypto: "Crypto",
  reit: "REITs",
};

const PERIOD_OPTIONS = [
  { value: "1M", label: "1M", months: 1 },
  { value: "3M", label: "3M", months: 3 },
  { value: "6M", label: "6M", months: 6 },
  { value: "1Y", label: "1Y", months: 12 },
  { value: "ALL", label: "ALL", months: 120 },
] as const;

const chartConfig = {
  net_worth: { label: "Net Worth", color: "var(--chart-1)" },
} satisfies ChartConfig;

function getDateRange(months: number) {
  const to = new Date();
  const from = new Date();
  from.setMonth(from.getMonth() - months);
  return {
    from_date: from.toISOString().split("T")[0],
    to_date: to.toISOString().split("T")[0],
  };
}

function InvestmentsSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-8 w-48" />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-28" />
        ))}
      </div>
      <Skeleton className="h-[350px] w-full" />
      <Skeleton className="h-64 w-full" />
    </div>
  );
}

function NetWorthChart() {
  const [period, setPeriod] = useState("1Y");

  const { from_date, to_date } = useMemo(() => {
    const opt = PERIOD_OPTIONS.find((p) => p.value === period);
    return getDateRange(opt?.months ?? 12);
  }, [period]);

  const { data: snapshotsData, isLoading } = usePortfolioSnapshots({
    from_date,
    to_date,
    page_size: 1000,
  });

  const chartData = useMemo(() => {
    if (!snapshotsData?.data) return [];
    return snapshotsData.data.map((s) => ({
      date: formatDate(s.recorded_at),
      net_worth: s.total_net_worth / 100,
    }));
  }, [snapshotsData]);

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-36" />
          <Skeleton className="h-4 w-48" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-[300px] w-full" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Net Worth</CardTitle>
            <CardDescription>Portfolio value over time</CardDescription>
          </div>
          <Tabs value={period} onValueChange={setPeriod}>
            <TabsList>
              {PERIOD_OPTIONS.map((opt) => (
                <TabsTrigger key={opt.value} value={opt.value}>
                  {opt.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
      </CardHeader>
      <CardContent>
        {chartData.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-10 text-muted-foreground">
            <p className="text-sm">
              No snapshot data available. Snapshots are generated periodically by
              the pipeline.
            </p>
          </div>
        ) : (
          <ChartContainer
            config={chartConfig}
            className="min-h-[300px] w-full"
          >
            <AreaChart accessibilityLayer data={chartData}>
              <defs>
                <linearGradient
                  id="fillNetWorth"
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop
                    offset="0%"
                    stopColor="var(--color-net_worth)"
                    stopOpacity={0.3}
                  />
                  <stop
                    offset="95%"
                    stopColor="var(--color-net_worth)"
                    stopOpacity={0.05}
                  />
                </linearGradient>
              </defs>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="date"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
              />
              <YAxis
                hide
                domain={["dataMin - 100", "dataMax + 100"]}
              />
              <ChartTooltip
                content={({ active, payload, label }) => {
                  if (!active || !payload?.length) return null;
                  const value = payload[0].value as number;
                  return (
                    <div className="border-border/50 bg-background rounded-lg border px-3 py-2 text-xs shadow-xl">
                      <div className="font-medium">{label}</div>
                      <div className="mt-1 font-mono font-medium tabular-nums">
                        {formatCurrency(value * 100)}
                      </div>
                    </div>
                  );
                }}
              />
              <Area
                type="monotone"
                dataKey="net_worth"
                stroke="var(--color-net_worth)"
                fill="url(#fillNetWorth)"
                strokeWidth={2}
              />
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}

const HOLDINGS_PAGE_SIZE = 20;

function AllHoldingsTable() {
  const router = useRouter();
  const [page, setPage] = useState(1);
  const { data, isLoading } = useAllInvestments({
    page,
    page_size: HOLDINGS_PAGE_SIZE,
  });

  const investments = data?.data ?? [];
  const totalPages = data?.total_pages ?? 1;
  const totalItems = data?.total_items ?? 0;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <List className="h-4 w-4 text-muted-foreground" />
          <CardTitle>All Holdings</CardTitle>
        </div>
        <CardDescription>
          {totalItems} holding{totalItems !== 1 ? "s" : ""} across all
          investment accounts
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-3">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        ) : investments.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-lg border border-dashed p-8 text-center">
            <h3 className="text-lg font-semibold">No holdings yet</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              Add investments to your accounts to see them here.
            </p>
          </div>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Symbol</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Account</TableHead>
                  <TableHead className="text-right">Qty</TableHead>
                  <TableHead className="text-right">Price</TableHead>
                  <TableHead className="text-right">Market Value</TableHead>
                  <TableHead className="text-right">Unrealized G/L</TableHead>
                  <TableHead className="text-right">Realized G/L</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {investments.map((inv) => {
                  const marketValue = Math.round(
                    inv.quantity * inv.current_price
                  );
                  const gainLoss = marketValue - inv.cost_basis;
                  const isPositive = gainLoss >= 0;
                  return (
                    <TableRow
                      key={inv.id}
                      className="cursor-pointer"
                      onClick={() => router.push(`/investments/${inv.id}`)}
                    >
                      <TableCell className="font-mono font-semibold">
                        {inv.security.symbol}
                      </TableCell>
                      <TableCell>{inv.security.name}</TableCell>
                      <TableCell>
                        <Link
                          href={`/accounts/${inv.account_id}`}
                          className="text-primary hover:underline"
                          onClick={(e) => e.stopPropagation()}
                        >
                          {inv.account?.name ?? `Account #${inv.account_id}`}
                        </Link>
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {inv.quantity.toFixed(
                          Number.isInteger(inv.quantity) ? 0 : 6
                        )}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {formatCurrency(inv.current_price)}
                      </TableCell>
                      <TableCell className="text-right font-medium font-mono tabular-nums">
                        {formatCurrency(marketValue)}
                      </TableCell>
                      <TableCell
                        className={`text-right font-medium font-mono tabular-nums ${
                          isPositive ? "text-green-600" : "text-red-600"
                        }`}
                      >
                        {isPositive ? "+" : ""}
                        {formatCurrency(gainLoss)}
                      </TableCell>
                      <TableCell
                        className={`text-right font-medium font-mono tabular-nums ${
                          inv.realized_gain_loss >= 0
                            ? "text-green-600"
                            : "text-red-600"
                        }`}
                      >
                        {inv.realized_gain_loss >= 0 ? "+" : ""}
                        {formatCurrency(inv.realized_gain_loss)}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>

            {totalPages > 1 && (
              <div className="mt-4 flex items-center justify-between">
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
  );
}

export default function InvestmentsPage() {
  const { data: portfolio, isLoading } = usePortfolio();

  if (isLoading) {
    return <InvestmentsSkeleton />;
  }

  if (!portfolio) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold">Investments</h1>
        <Card>
          <CardHeader>
            <CardTitle>No Portfolio Data</CardTitle>
            <CardDescription>
              Add investments to your investment accounts to see your portfolio
              here.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              Go to an{" "}
              <Link href="/accounts" className="text-primary underline">
                investment account
              </Link>{" "}
              and add your first investment to get started.
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  const gainLossColor =
    portfolio.total_gain_loss >= 0
      ? "text-green-600 dark:text-green-400"
      : "text-red-600 dark:text-red-400";

  const realizedGainLossColor =
    portfolio.total_realized_gain_loss >= 0
      ? "text-green-600 dark:text-green-400"
      : "text-red-600 dark:text-red-400";

  const holdingsCount = Object.values(portfolio.holdings_by_type).reduce(
    (sum, h) => sum + h.count,
    0
  );

  const holdingsEntries = Object.entries(portfolio.holdings_by_type).filter(
    ([, h]) => h.count > 0
  ) as [AssetType, { value: number; count: number }][];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Investments</h1>

      {/* Summary Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <Card className="gap-2">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardDescription>Total Value</CardDescription>
              <DollarSign className="h-4 w-4 text-muted-foreground" />
            </div>
            <CardTitle className="text-2xl">
              {formatCurrency(portfolio.total_value)}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              Current market value
            </p>
          </CardContent>
        </Card>

        <Card className="gap-2">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardDescription>Cost Basis</CardDescription>
              <Landmark className="h-4 w-4 text-muted-foreground" />
            </div>
            <CardTitle className="text-2xl">
              {formatCurrency(portfolio.total_cost_basis)}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">Total invested</p>
          </CardContent>
        </Card>

        <Card className="gap-2">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardDescription>Unrealized G/L</CardDescription>
              {portfolio.total_gain_loss >= 0 ? (
                <TrendingUp className="h-4 w-4 text-green-600" />
              ) : (
                <TrendingDown className="h-4 w-4 text-red-600" />
              )}
            </div>
            <CardTitle
              className={`text-2xl whitespace-nowrap ${gainLossColor}`}
            >
              {portfolio.total_gain_loss >= 0 ? "+" : "-"}{" "}
              {formatCurrency(Math.abs(portfolio.total_gain_loss))}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className={`text-sm ${gainLossColor}`}>
              {portfolio.total_gain_loss >= 0 ? "+" : "-"}
              {formatPercentage(Math.abs(portfolio.gain_loss_pct))}
            </p>
          </CardContent>
        </Card>

        <Card className="gap-2">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardDescription>Realized G/L</CardDescription>
              {portfolio.total_realized_gain_loss >= 0 ? (
                <TrendingUp className="h-4 w-4 text-green-600" />
              ) : (
                <TrendingDown className="h-4 w-4 text-red-600" />
              )}
            </div>
            <CardTitle
              className={`text-2xl whitespace-nowrap ${realizedGainLossColor}`}
            >
              {portfolio.total_realized_gain_loss >= 0 ? "+" : "-"}{" "}
              {formatCurrency(Math.abs(portfolio.total_realized_gain_loss))}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              From closed positions
            </p>
          </CardContent>
        </Card>

        <Card className="gap-2">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardDescription>Holdings</CardDescription>
              <BarChart3 className="h-4 w-4 text-muted-foreground" />
            </div>
            <CardTitle className="text-2xl">{holdingsCount}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              Across {holdingsEntries.length} asset type
              {holdingsEntries.length !== 1 ? "s" : ""}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Net Worth Chart */}
      <NetWorthChart />

      {/* Holdings by Type */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <PieChart className="h-4 w-4 text-muted-foreground" />
            <CardTitle>Holdings by Asset Type</CardTitle>
          </div>
          <CardDescription>
            Breakdown of your portfolio by asset class
          </CardDescription>
        </CardHeader>
        <CardContent>
          {holdingsEntries.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No holdings to display.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b text-left text-sm text-muted-foreground">
                    <th className="pb-2 font-medium">Asset Type</th>
                    <th className="pb-2 text-right font-medium">Holdings</th>
                    <th className="pb-2 text-right font-medium">
                      Total Value
                    </th>
                    <th className="pb-2 text-right font-medium">Allocation</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {holdingsEntries.map(([type, holding]) => {
                    const allocation =
                      portfolio.total_value > 0
                        ? (holding.value / portfolio.total_value) * 100
                        : 0;
                    return (
                      <tr key={type} className="text-sm">
                        <td className="py-3">
                          <Badge variant="secondary">
                            {ASSET_TYPE_LABELS[type] ?? type}
                          </Badge>
                        </td>
                        <td className="py-3 text-right font-mono tabular-nums">
                          {holding.count}
                        </td>
                        <td className="py-3 text-right font-mono tabular-nums">
                          {formatCurrency(holding.value)}
                        </td>
                        <td className="py-3 text-right font-mono tabular-nums">
                          {formatPercentage(allocation)}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* All Holdings Table */}
      <AllHoldingsTable />
    </div>
  );
}
