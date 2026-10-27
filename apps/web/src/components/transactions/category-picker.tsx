"use client";

import { useMemo, useState } from "react";
import { Check, ChevronDown } from "lucide-react";
import { useTransactions } from "@/hooks/use-transactions";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import type { Category } from "@/types/models";

/** How many most-used categories are listed under "Recent". */
const RECENT_COUNT = 5;
/** Recent transactions sampled to rank categories by use (API max page). */
const SAMPLE_SIZE = 100;

/** Value the transaction dialogs read as "no category". */
export const NO_CATEGORY = "none";

interface CategoryPickerProps {
  id: string;
  /** Categories of the transaction's type, sorted by name. */
  categories: Category[];
  type: "income" | "expense";
  /** Selected category id; "" or NO_CATEGORY when none. */
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}

/**
 * Category field that opens a searchable list: the most-used categories
 * (ranked from the latest transactions of the same type) come first, then the
 * rest A–Z. Opens as a bottom sheet on phones and a dropdown on desktop.
 */
export function CategoryPicker({
  id,
  categories,
  type,
  value,
  onChange,
  disabled,
}: CategoryPickerProps) {
  const isMobile = useIsMobile();
  const [open, setOpen] = useState(false);
  const { data: sample } = useTransactions({ page_size: SAMPLE_SIZE, type });

  const recent = useMemo(() => {
    const uses = new Map<string, number>();
    for (const tx of sample?.data ?? []) {
      if (tx.category_id) {
        uses.set(tx.category_id, (uses.get(tx.category_id) ?? 0) + 1);
      }
    }
    return categories
      .filter((c) => uses.has(c.id))
      .sort((a, b) => (uses.get(b.id) ?? 0) - (uses.get(a.id) ?? 0))
      .slice(0, RECENT_COUNT);
  }, [sample?.data, categories]);

  const selected = categories.find((c) => c.id === value);

  function pick(next: string) {
    onChange(next);
    setOpen(false);
  }

  const trigger = (
    <button
      id={id}
      type="button"
      disabled={disabled}
      aria-expanded={open}
      className="border-input dark:bg-input/30 dark:hover:bg-input/50 focus-visible:border-ring focus-visible:ring-ring/50 flex h-11 w-full items-center gap-2 rounded-md border bg-transparent px-3 text-sm shadow-xs outline-none focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50 sm:h-10"
    >
      {selected ? (
        <>
          <CategoryDot color={selected.color} />
          <span className="truncate">{selected.name}</span>
        </>
      ) : (
        <span className="text-muted-foreground">Select category (optional)</span>
      )}
      <ChevronDown className="ml-auto size-4 shrink-0 text-muted-foreground" />
    </button>
  );

  const list = (
    <CategoryList
      categories={categories}
      recent={recent}
      value={selected?.id}
      onPick={pick}
      mobile={isMobile}
    />
  );

  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>Category</Label>
      {isMobile ? (
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>{trigger}</DialogTrigger>
          <DialogContent
            // Keep the keyboard down so the recent categories stay in view;
            // tapping the search box brings it up when needed.
            onOpenAutoFocus={(e) => e.preventDefault()}
          >
            <DialogHeader>
              <DialogTitle>Category</DialogTitle>
              <DialogDescription className="sr-only">
                Search or pick a category for this transaction.
              </DialogDescription>
            </DialogHeader>
            {list}
          </DialogContent>
        </Dialog>
      ) : (
        // Modal so the dialog's scroll lock doesn't swallow wheel scrolling
        // in the portalled list.
        <Popover open={open} onOpenChange={setOpen} modal>
          <PopoverTrigger asChild>{trigger}</PopoverTrigger>
          <PopoverContent
            align="start"
            className="w-(--radix-popover-trigger-width) p-0"
          >
            {list}
          </PopoverContent>
        </Popover>
      )}
    </div>
  );
}

function CategoryList({
  categories,
  recent,
  value,
  onPick,
  mobile,
}: {
  categories: Category[];
  recent: Category[];
  value: string | undefined;
  onPick: (value: string) => void;
  mobile: boolean;
}) {
  const recentIds = new Set(recent.map((c) => c.id));
  const rest = categories.filter((c) => !recentIds.has(c.id));
  const itemClass = mobile ? "py-3 text-base" : undefined;

  function item(cat: Category) {
    return (
      <CommandItem
        key={cat.id}
        value={cat.id}
        keywords={[cat.name]}
        onSelect={() => onPick(cat.id)}
        className={itemClass}
      >
        <CategoryDot color={cat.color} />
        <span className="truncate">{cat.name}</span>
        {cat.id === value && <Check className="ml-auto size-4" />}
      </CommandItem>
    );
  }

  return (
    // Match on the name only: item values are ids, which would otherwise
    // match almost any search.
    <Command
      filter={(_value, search, keywords) =>
        keywords?.some((k) => k.toLowerCase().includes(search.toLowerCase()))
          ? 1
          : 0
      }
      className={cn(
        mobile &&
          "bg-transparent **:data-[slot=command-input-wrapper]:h-12 **:data-[slot=command-input]:h-12 **:data-[slot=command-input]:text-base"
      )}
    >
      <CommandInput placeholder="Search categories" />
      {/* Fixed height on phones so the sheet doesn't jump as results narrow. */}
      <CommandList className={cn(mobile && "h-[50dvh] max-h-none")}>
        <CommandEmpty>No matching category.</CommandEmpty>
        {value && (
          <CommandGroup>
            <CommandItem
              value={NO_CATEGORY}
              keywords={["no category", "none", "clear"]}
              onSelect={() => onPick(NO_CATEGORY)}
              className={cn(itemClass, "text-muted-foreground")}
            >
              No category
            </CommandItem>
          </CommandGroup>
        )}
        {recent.length > 0 && (
          <CommandGroup heading="Recent">{recent.map(item)}</CommandGroup>
        )}
        {rest.length > 0 && (
          <CommandGroup heading={recent.length > 0 ? "All" : undefined}>
            {rest.map(item)}
          </CommandGroup>
        )}
      </CommandList>
    </Command>
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
