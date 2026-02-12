"use client";

import { useMemo } from "react";
import {
  ComposedChart,
  Area,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
} from "recharts";
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
import { Skeleton } from "@/components/ui/skeleton";
import { useDailySpending } from "@/hooks/use-transactions";
import { formatCurrency } from "@/lib/format";
import { useIsMobile } from "@/hooks/use-mobile";

const chartConfig = {
  daily: { label: "Daily", color: "var(--chart-2)" },
  cumulative: { label: "Cumulative", color: "var(--chart-1)" },
} satisfies ChartConfig;

export function SpendingTrendChart() {
  const isMobile = useIsMobile();
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

  const { chartData, currentTotal, lastDay } = useMemo(() => {
    if (!data) return { chartData: [], currentTotal: 0, lastDay: "1" };

    const today = new Date();
    const todayDay = today.getDate();

    // Only include days up to today (truncate future days)
    let running = 0;
    const items = data
      .filter((item) => {
        const day = new Date(item.date).getDate();
        return day <= todayDay;
      })
      .map((item) => {
        running += item.total;
        const day = new Date(item.date).getDate().toString();
        return { date: day, daily: item.total, cumulative: running };
      });

    // Last day of the month for X-axis domain
    const year = today.getFullYear();
    const month = today.getMonth();
    const endOfMonth = new Date(year, month + 1, 0).getDate().toString();

    return {
      chartData: items,
      currentTotal: items.length > 0 ? items[items.length - 1].cumulative : 0,
      lastDay: endOfMonth,
    };
  }, [data]);

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
          <CardDescription>
            Daily &amp; cumulative spending &middot; {monthLabel}
          </CardDescription>
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
          Daily &amp; cumulative spending &middot; {monthLabel} &middot;{" "}
          {formatCurrency(currentTotal)}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="h-[250px] md:h-[300px] w-full">
          <ComposedChart accessibilityLayer data={chartData}>
            <defs>
              <linearGradient
                id="fillCumulative"
                x1="0"
                y1="0"
                x2="0"
                y2="1"
              >
                <stop
                  offset="0%"
                  stopColor="var(--color-cumulative)"
                  stopOpacity={0.3}
                />
                <stop
                  offset="95%"
                  stopColor="var(--color-cumulative)"
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
              domain={["1", lastDay]}
              minTickGap={isMobile ? 50 : 30}
            />
            <YAxis yAxisId="cumulative" hide />
            <YAxis yAxisId="daily" hide />
            <ChartTooltip
              content={({ active, payload, label }) => {
                if (!active || !payload?.length) return null;
                const daily = payload.find(
                  (p) => p.dataKey === "daily"
                )?.value as number;
                const cumulative = payload.find(
                  (p) => p.dataKey === "cumulative"
                )?.value as number;
                return (
                  <div className="border-border/50 bg-background rounded-lg border px-3 py-2 text-xs shadow-xl">
                    <div className="font-medium">Day {label}</div>
                    <div className="mt-1.5 space-y-1">
                      <div className="flex items-center gap-2">
                        <div
                          className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                          style={{
                            backgroundColor: "var(--color-daily)",
                          }}
                        />
                        <span className="text-muted-foreground">Daily</span>
                        <span className="ml-auto font-medium font-mono tabular-nums">
                          {formatCurrency(daily)}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <div
                          className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                          style={{
                            backgroundColor: "var(--color-cumulative)",
                          }}
                        />
                        <span className="text-muted-foreground">
                          Cumulative
                        </span>
                        <span className="ml-auto font-medium font-mono tabular-nums">
                          {formatCurrency(cumulative)}
                        </span>
                      </div>
                    </div>
                  </div>
                );
              }}
            />
            <Bar
              yAxisId="daily"
              dataKey="daily"
              fill="var(--color-daily)"
              radius={[2, 2, 0, 0]}
              opacity={0.6}
            />
            <Area
              yAxisId="cumulative"
              type="monotone"
              dataKey="cumulative"
              stroke="var(--color-cumulative)"
              fill="url(#fillCumulative)"
              strokeWidth={2}
            />
          </ComposedChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
