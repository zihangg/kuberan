"use client";

import { useMemo } from "react";
import { Label, Pie, PieChart } from "recharts";
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
import { useSpendingByCategory } from "@/hooks/use-transactions";
import { formatCurrency } from "@/lib/format";

const TOP_N = 3;
const OTHERS_COLOR = "#6B7280";

export function ExpenditureChart() {
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

  const { data, isLoading } = useSpendingByCategory(fromDate, toDate);

  const { chartConfig, chartData } = useMemo(() => {
    if (!data?.items)
      return { chartConfig: {} as ChartConfig, chartData: [] };

    const sorted = [...data.items];
    const top = sorted.slice(0, TOP_N);
    const rest = sorted.slice(TOP_N);

    // Build display items: top N + optional "Others" bucket
    const items = [...top];
    if (rest.length > 0) {
      const othersTotal = rest.reduce((sum, item) => sum + item.total, 0);
      items.push({
        category_id: null,
        category_name: "Others",
        category_color: OTHERS_COLOR,
        category_icon: "",
        total: othersTotal,
      });
    }

    // Build chart config from display items
    const config: ChartConfig = {
      total: { label: "Spending" },
    };
    for (const item of items) {
      const key = item.category_name
        .toLowerCase()
        .replace(/\s+/g, "-")
        .replace(/[^a-z0-9-]/g, "");
      config[key] = { label: item.category_name, color: item.category_color };
    }

    // Build chart data from display items
    const cData = items.map((item) => ({
      name: item.category_name,
      value: item.total,
      fill: item.category_color,
      icon: item.category_icon,
    }));

    return { chartConfig: config, chartData: cData };
  }, [data]);

  if (isLoading) {
    return (
      <Card className="flex flex-col">
        <CardHeader className="items-center pb-0">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-4 w-28" />
        </CardHeader>
        <CardContent className="flex flex-1 items-center justify-center">
          <Skeleton className="h-[200px] w-[200px] rounded-full" />
        </CardContent>
      </Card>
    );
  }

  if (!data || data.items.length === 0) {
    return (
      <Card className="flex flex-col">
        <CardHeader className="items-center pb-0">
          <CardTitle>Spending Breakdown</CardTitle>
          <CardDescription>{monthLabel}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-1 flex-col items-center justify-center py-10 text-muted-foreground">
          <p className="text-sm">No expenses this month</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="flex flex-col">
      <CardHeader className="items-center pb-0">
        <CardTitle>Spending Breakdown</CardTitle>
        <CardDescription>{monthLabel}</CardDescription>
      </CardHeader>
      <CardContent className="flex-1 pb-0">
        <ChartContainer
          config={chartConfig}
          className="mx-auto aspect-square max-h-[250px]"
        >
          <PieChart>
            <ChartTooltip
              cursor={false}
              content={({ active, payload }) => {
                if (!active || !payload?.length) return null;
                const item = payload[0];
                const value = item.value as number;
                const pct =
                  data.total_spent > 0
                    ? ((value / data.total_spent) * 100).toFixed(1)
                    : "0";
                return (
                  <div className="border-border/50 bg-background rounded-lg border px-3 py-2 text-xs shadow-xl">
                    <div className="flex items-center gap-2">
                      <div
                        className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                        style={{ backgroundColor: item.payload.fill }}
                      />
                      <span className="font-medium">
                        {item.name}
                      </span>
                    </div>
                    <div className="text-muted-foreground mt-1">
                      {formatCurrency(value)} ({pct}%)
                    </div>
                  </div>
                );
              }}
            />
            <Pie
              data={chartData}
              dataKey="value"
              nameKey="name"
              innerRadius={70}
              strokeWidth={5}
            >
              <Label
                content={({ viewBox }) => {
                  if (viewBox && "cx" in viewBox && "cy" in viewBox) {
                    return (
                      <text
                        x={viewBox.cx}
                        y={viewBox.cy}
                        textAnchor="middle"
                        dominantBaseline="middle"
                      >
                        <tspan
                          x={viewBox.cx}
                          y={viewBox.cy}
                          className="fill-foreground text-base font-bold"
                        >
                          {formatCurrency(data.total_spent)}
                        </tspan>
                        <tspan
                          x={viewBox.cx}
                          y={(viewBox.cy || 0) + 18}
                          className="fill-muted-foreground text-xs"
                        >
                          This Month
                        </tspan>
                      </text>
                    );
                  }
                }}
              />
            </Pie>
          </PieChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
