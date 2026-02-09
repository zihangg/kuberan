"use client";

import { useMemo } from "react";
import { AreaChart, Area, XAxis, CartesianGrid } from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useDailySpending } from "@/hooks/use-transactions";
import { formatCurrency } from "@/lib/format";

const chartConfig = {
  cumulative: { label: "Total Spent", color: "var(--chart-1)" },
} satisfies ChartConfig;

export function SpendingTrendChart() {
  const { fromDate, toDate, monthLabel } = useMemo(() => {
    const now = new Date();
    const year = now.getFullYear();
    const month = now.getMonth();
    const from = new Date(year, month, 1);
    const to = new Date(year, month + 1, 0, 23, 59, 59);
    return {
      fromDate: from.toISOString(),
      toDate: to.toISOString(),
      monthLabel: from.toLocaleDateString("en-US", {
        month: "long",
        year: "numeric",
      }),
    };
  }, []);

  const { data, isLoading } = useDailySpending(fromDate, toDate);

  const chartData = useMemo(() => {
    if (!data) return [];
    let running = 0;
    return data.map((item) => {
      running += item.total;
      const day = new Date(item.date).getDate().toString();
      return { date: day, daily: item.total, cumulative: running };
    });
  }, [data]);

  const currentTotal = chartData.length > 0 ? chartData[chartData.length - 1].cumulative : 0;

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-36" />
          <Skeleton className="h-4 w-48" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-[200px] w-full" />
        </CardContent>
      </Card>
    );
  }

  if (!data || data.every((d) => d.total === 0)) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Spending Trend</CardTitle>
          <CardDescription>Cumulative spending &middot; {monthLabel}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
          <p className="text-sm">No spending data this month</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Spending Trend</CardTitle>
        <CardDescription>
          Cumulative spending &middot; {monthLabel} &middot;{" "}
          {formatCurrency(currentTotal)}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="min-h-[200px] w-full">
          <AreaChart accessibilityLayer data={chartData}>
            <defs>
              <linearGradient id="fillCumulative" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="var(--color-cumulative)" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--color-cumulative)" stopOpacity={0.05} />
              </linearGradient>
            </defs>
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="date"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
            />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  formatter={(value) => formatCurrency(value as number)}
                />
              }
            />
            <Area
              type="monotone"
              dataKey="cumulative"
              stroke="var(--color-cumulative)"
              fill="url(#fillCumulative)"
              strokeWidth={2}
            />
          </AreaChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
