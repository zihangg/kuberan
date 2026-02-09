"use client";

import { useMemo } from "react";
import { PieChart, Pie, Cell, Label } from "recharts";
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
import { useSpendingByCategory } from "@/hooks/use-transactions";
import { formatCurrency } from "@/lib/format";

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

  const TOP_N = 3;
  const OTHERS_COLOR = "#6B7280";

  const { displayItems, chartConfig, chartData } = useMemo(() => {
    if (!data?.items)
      return { displayItems: [], chartConfig: {} as ChartConfig, chartData: [] };

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
    const config: ChartConfig = {};
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

    return { displayItems: items, chartConfig: config, chartData: cData };
  }, [data]);

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-4 w-28" />
        </CardHeader>
        <CardContent className="flex items-center justify-center">
          <Skeleton className="h-[200px] w-[200px] rounded-full" />
        </CardContent>
      </Card>
    );
  }

  if (!data || data.items.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Spending Breakdown</CardTitle>
          <CardDescription>{monthLabel}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col items-center justify-center py-10 text-muted-foreground">
          <p className="text-sm">No expenses this month</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Spending Breakdown</CardTitle>
        <CardDescription>{monthLabel}</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="mx-auto aspect-square max-h-[250px]">
          <PieChart>
            <ChartTooltip
              content={
                <ChartTooltipContent
                  formatter={(value) => formatCurrency(value as number)}
                />
              }
            />
            <Pie
              data={chartData}
              dataKey="value"
              nameKey="name"
              innerRadius={60}
              outerRadius={80}
              paddingAngle={2}
            >
              {chartData.map((entry, index) => (
                <Cell key={index} fill={entry.fill} />
              ))}
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
                          className="fill-foreground text-lg font-bold"
                        >
                          {formatCurrency(data.total_spent)}
                        </tspan>
                        <tspan
                          x={viewBox.cx}
                          y={(viewBox.cy || 0) + 20}
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
        <div className="mt-4 space-y-2">
          {displayItems.map((item) => {
            const pct =
              data.total_spent > 0
                ? ((item.total / data.total_spent) * 100).toFixed(1)
                : "0";
            return (
              <div key={item.category_name} className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2">
                  <div
                    className="h-3 w-3 rounded-full shrink-0"
                    style={{ backgroundColor: item.category_color }}
                  />
                  <span>
                    {item.category_icon && `${item.category_icon} `}
                    {item.category_name}
                  </span>
                </div>
                <span className="text-muted-foreground">
                  {formatCurrency(item.total)} ({pct}%)
                </span>
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}
