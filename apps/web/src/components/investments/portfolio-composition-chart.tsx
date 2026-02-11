"use client";

import { useMemo } from "react";
import { BarChart, Bar, XAxis, YAxis } from "recharts";
import {
  ChartContainer,
  ChartTooltip,
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
import { usePortfolioSnapshots } from "@/hooks/use-portfolio-snapshots";
import { formatCurrency } from "@/lib/format";

const chartConfig = {
  cash: { label: "Cash", color: "var(--chart-2)" }, // green
  investments: { label: "Investments", color: "var(--chart-1)" }, // blue
  debt: { label: "Debt", color: "var(--chart-3)" }, // amber/orange
} satisfies ChartConfig;

export function PortfolioCompositionChart() {
  // Fetch latest snapshot with wide date range
  const { from_date, to_date } = useMemo(() => {
    const to = new Date();
    const from = new Date();
    from.setFullYear(from.getFullYear() - 10);
    return {
      from_date: from.toISOString().split("T")[0],
      to_date: to.toISOString().split("T")[0],
    };
  }, []);

  const { data, isLoading } = usePortfolioSnapshots({
    from_date,
    to_date,
    page_size: 1,
  });

  const chartData = useMemo(() => {
    if (!data?.data || data.data.length === 0) return null;

    const latestSnapshot = data.data[0];
    return [
      {
        category: "Portfolio",
        cash: latestSnapshot.cash_balance / 100,
        investments: latestSnapshot.investment_value / 100,
        debt: Math.abs(latestSnapshot.debt_balance) / 100, // debt is negative, make positive for display
      },
    ];
  }, [data]);

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-44" />
          <Skeleton className="h-4 w-48" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-[250px] w-full" />
        </CardContent>
      </Card>
    );
  }

  if (!chartData) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Portfolio Composition</CardTitle>
          <CardDescription>Breakdown of your net worth</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
          <p className="text-sm">
            No snapshot data available. Snapshots are generated periodically by
            the pipeline.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Portfolio Composition</CardTitle>
        <CardDescription>Breakdown of your net worth</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="min-h-[250px] w-full">
          <BarChart
            accessibilityLayer
            data={chartData}
            layout="vertical"
            margin={{ left: 0, right: 12 }}
          >
            <XAxis type="number" hide />
            <YAxis
              dataKey="category"
              type="category"
              tickLine={false}
              axisLine={false}
              hide
            />
            <ChartTooltip
              cursor={false}
              content={({ active, payload }) => {
                if (!active || !payload?.length) return null;
                return (
                  <div className="border-border/50 bg-background rounded-lg border px-3 py-2 text-xs shadow-xl">
                    <div className="space-y-1.5">
                      {payload.map((item) => (
                        <div key={item.dataKey} className="flex items-center gap-2">
                          <div
                            className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                            style={{ backgroundColor: item.color }}
                          />
                          <span className="font-medium">{item.name}:</span>
                          <span className="text-muted-foreground ml-auto font-mono tabular-nums">
                            {formatCurrency((item.value as number) * 100)}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                );
              }}
            />
            <ChartLegend content={<ChartLegendContent />} />
            <Bar
              dataKey="cash"
              stackId="a"
              fill="var(--color-cash)"
              radius={[4, 0, 0, 4]}
            />
            <Bar
              dataKey="investments"
              stackId="a"
              fill="var(--color-investments)"
              radius={[0, 0, 0, 0]}
            />
            <Bar
              dataKey="debt"
              stackId="a"
              fill="var(--color-debt)"
              radius={[0, 4, 4, 0]}
            />
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
