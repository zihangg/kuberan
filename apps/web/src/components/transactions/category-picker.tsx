"use client";

import { useMemo } from "react";
import { useTransactions } from "@/hooks/use-transactions";
import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import type { Category } from "@/types/models";

/** How many categories get a one-tap chip. */
const CHIP_COUNT = 6;
/** Recent transactions sampled to rank categories by use (API max page). */
const SAMPLE_SIZE = 100;

/** Value the transaction dialogs read as "no category". */
export const NO_CATEGORY = "none";

interface CategoryPickerProps {
  id: string;
  /** Categories of the transaction's type. */
  categories: Category[];
  type: "income" | "expense";
  /** Selected category id; "" or NO_CATEGORY when none. */
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}

/**
 * Category choice tuned for touch: the most-used categories (ranked from the
 * latest transactions of the same type) are one-tap chips, and "More" opens the
 * full list. Tapping the selected chip clears the category.
 */
export function CategoryPicker({
  id,
  categories,
  type,
  value,
  onChange,
  disabled,
}: CategoryPickerProps) {
  const { data: recent } = useTransactions({ page_size: SAMPLE_SIZE, type });

  const chips = useMemo(() => {
    const uses = new Map<string, number>();
    for (const tx of recent?.data ?? []) {
      if (tx.category_id) {
        uses.set(tx.category_id, (uses.get(tx.category_id) ?? 0) + 1);
      }
    }
    // Most used first; unused categories keep their alphabetical order.
    const ranked = categories
      .map((c, i) => ({ c, i, n: uses.get(c.id) ?? 0 }))
      .sort((a, b) => b.n - a.n || a.i - b.i)
      .map((r) => r.c);
    const top = ranked.slice(0, CHIP_COUNT);
    // A category picked from "More" (or already on the transaction) stays
    // visible as a chip so the choice is shown and can be cleared.
    const selected = categories.find((c) => c.id === value);
    if (selected && !top.some((c) => c.id === selected.id)) {
      top[Math.min(top.length, CHIP_COUNT - 1)] = selected;
    }
    return top;
  }, [recent?.data, categories, value]);

  const labelId = `${id}-label`;

  return (
    <div className="flex flex-col gap-1.5">
      <Label id={labelId}>Category</Label>
      <div
        role="group"
        aria-labelledby={labelId}
        className="flex flex-wrap gap-1.5"
      >
        {chips.map((cat) => {
          const active = cat.id === value;
          return (
            <button
              key={cat.id}
              type="button"
              aria-pressed={active}
              disabled={disabled}
              onClick={() => onChange(active ? NO_CATEGORY : cat.id)}
              className={cn(
                "inline-flex min-h-10 items-center gap-1.5 rounded-full border px-3 text-sm font-medium transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50 sm:min-h-8",
                active
                  ? "border-primary bg-primary/10 text-foreground"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              <CategoryDot color={cat.color} />
              {cat.name}
            </button>
          );
        })}
        {categories.length > chips.length && (
          <Select value="" onValueChange={onChange} disabled={disabled}>
            <SelectTrigger
              id={id}
              size="sm"
              aria-label="More categories"
              className="h-auto min-h-10 rounded-full px-3 font-medium text-muted-foreground data-[size=sm]:h-auto sm:min-h-8"
            >
              More
            </SelectTrigger>
            {/* Popper: with no item selected, item-aligned can open off-screen. */}
            <SelectContent position="popper" className="max-h-72">
              {categories.map((cat) => (
                <SelectItem key={cat.id} value={cat.id}>
                  <CategoryDot color={cat.color} />
                  {cat.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>
      {categories.length === 0 && (
        <p className="text-sm text-muted-foreground">
          No {type} categories yet.
        </p>
      )}
    </div>
  );
}

function CategoryDot({ color }: { color: string }) {
  if (!color) return null;
  return (
    <span
      aria-hidden
      className="size-2 shrink-0 rounded-full"
      style={{ backgroundColor: color }}
    />
  );
}
