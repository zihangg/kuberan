"use client";

import { useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useCategories } from "@/hooks/use-categories";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
import { CreateCategoryDialog } from "@/components/categories/create-category-dialog";
import { EditCategoryDialog } from "@/components/categories/edit-category-dialog";
import { DeleteCategoryDialog } from "@/components/categories/delete-category-dialog";
import type { Category, CategoryType } from "@/types/models";

function CategoriesTableSkeleton() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-10 w-full" />
      {Array.from({ length: 5 }).map((_, i) => (
        <Skeleton key={i} className="h-12 w-full" />
      ))}
    </div>
  );
}

/** Sort flat category list into tree order: parents first, children indented after. */
function treeOrder(categories: Category[]): (Category & { isChild: boolean })[] {
  const result: (Category & { isChild: boolean })[] = [];
  const childrenMap = new Map<number, Category[]>();
  const topLevel: Category[] = [];

  for (const cat of categories) {
    if (cat.parent_id && categories.some((c) => c.id === cat.parent_id)) {
      const siblings = childrenMap.get(cat.parent_id) ?? [];
      siblings.push(cat);
      childrenMap.set(cat.parent_id, siblings);
    } else {
      topLevel.push(cat);
    }
  }

  for (const parent of topLevel) {
    result.push({ ...parent, isChild: false });
    const children = childrenMap.get(parent.id) ?? [];
    for (const child of children) {
      result.push({ ...child, isChild: true });
    }
  }

  return result;
}

export default function CategoriesPage() {
  const [typeFilter, setTypeFilter] = useState<string>("all");
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [editCategory, setEditCategory] = useState<Category | null>(null);
  const [deleteCategory, setDeleteCategory] = useState<Category | null>(null);

  const filterType = typeFilter === "all" ? undefined : (typeFilter as CategoryType);
  const { data, isLoading } = useCategories({ page, type: filterType });

  const categories = data?.data ?? [];
  const totalPages = data?.total_pages ?? 1;
  const currentPage = data?.page ?? 1;
  const ordered = treeOrder(categories);

  function handleTypeChange(value: string) {
    setTypeFilter(value);
    setPage(1);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Categories</h1>
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Create Category
        </Button>
      </div>

      <Tabs value={typeFilter} onValueChange={handleTypeChange}>
        <TabsList>
          <TabsTrigger value="all">All</TabsTrigger>
          <TabsTrigger value="income">Income</TabsTrigger>
          <TabsTrigger value="expense">Expense</TabsTrigger>
        </TabsList>
      </Tabs>

      {isLoading ? (
        <CategoriesTableSkeleton />
      ) : categories.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed p-12 text-center">
          <h3 className="text-lg font-semibold">No categories yet</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            Create your first category to organize your transactions.
          </p>
          <Button className="mt-4" size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create Category
          </Button>
        </div>
      ) : (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Color</TableHead>
                <TableHead>Icon</TableHead>
                <TableHead>Description</TableHead>
                <TableHead className="w-[80px]">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {ordered.map((cat) => (
                <TableRow key={cat.id}>
                  <TableCell className={cat.isChild ? "pl-8" : ""}>
                    <span className="font-medium">{cat.name}</span>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={cat.type === "income" ? "default" : "destructive"}
                    >
                      {cat.type === "income" ? "Income" : "Expense"}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {cat.color ? (
                      <div
                        className="h-5 w-5 rounded-full border"
                        style={{ backgroundColor: cat.color }}
                      />
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </TableCell>
                  <TableCell>
                    {cat.icon || <span className="text-muted-foreground">-</span>}
                  </TableCell>
                  <TableCell className="max-w-[200px] truncate">
                    {cat.description || (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        onClick={() => setEditCategory(cat)}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-destructive"
                        onClick={() => setDeleteCategory(cat)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={currentPage <= 1}
              >
                Previous
              </Button>
              <span className="text-sm text-muted-foreground">
                Page {currentPage} of {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={currentPage >= totalPages}
              >
                Next
              </Button>
            </div>
          )}
        </>
      )}

      <CreateCategoryDialog open={createOpen} onOpenChange={setCreateOpen} />
      <EditCategoryDialog
        open={!!editCategory}
        onOpenChange={(open) => {
          if (!open) setEditCategory(null);
        }}
        category={editCategory}
      />
      <DeleteCategoryDialog
        open={!!deleteCategory}
        onOpenChange={(open) => {
          if (!open) setDeleteCategory(null);
        }}
        category={deleteCategory}
      />
    </div>
  );
}
