"use client";

import { useMemo } from "react";
import { BarChart, Bar, XAxis, CartesianGrid } from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
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
import { useMonthlySummary } from "@/hooks/use-transactions";
import { formatCurrency } from "@/lib/format";

const chartConfig = {
  income: { label: "Income", color: "#22C55E" },
  expenses: { label: "Expenses", color: "#EF4444" },
} satisfies ChartConfig;

export function IncomeExpensesChart() {
  const { data, isLoading } = useMonthlySummary(6);

  const chartData = useMemo(() => {
    if (!data) return [];
    return data.map((item) => {
      const date = new Date(item.month + "-01");
      return {
        month: date.toLocaleDateString("en-US", { month: "short" }),
        income: item.income,
        expenses: item.expenses,
      };
    });
  }, [data]);

  const netSavings = useMemo(() => {
    if (!data) return 0;
    return data.reduce((acc, item) => acc + item.income - item.expenses, 0);
  }, [data]);

  const hasData = useMemo(() => {
    if (!data) return false;
    return data.some((item) => item.income > 0 || item.expenses > 0);
  }, [data]);

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-44" />
          <Skeleton className="h-4 w-32" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-[300px] w-full" />
        </CardContent>
      </Card>
    );
  }

  if (!hasData) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Income vs. Expenses</CardTitle>
          <CardDescription>Last 6 months</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
          <p className="text-sm">No transaction data yet</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Income vs. Expenses</CardTitle>
        <CardDescription>
          <span>Last 6 months &middot; Net: </span>
          <span className={netSavings >= 0 ? "text-green-600" : "text-red-600"}>
            {netSavings >= 0 ? "+" : ""}
            {formatCurrency(netSavings)}
          </span>
        </CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="h-[180px] md:h-[250px] w-full">
          <BarChart accessibilityLayer data={chartData}>
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="month"
              tickLine={false}
              tickMargin={10}
              axisLine={false}
            />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  formatter={(value) => formatCurrency(value as number)}
                />
              }
            />
            <ChartLegend content={<ChartLegendContent />} />
            <Bar
              dataKey="income"
              fill="var(--color-income)"
              radius={[4, 4, 0, 0]}
              barSize={20}
            />
            <Bar
              dataKey="expenses"
              fill="var(--color-expenses)"
              radius={[4, 4, 0, 0]}
              barSize={20}
            />
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
