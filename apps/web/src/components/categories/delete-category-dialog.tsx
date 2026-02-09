"use client";

import { useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import { useDeleteCategory } from "@/hooks/use-categories";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { Category } from "@/types/models";

function getErrorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
    if (error.code === "CATEGORY_HAS_CHILDREN") {
      return "This category has child categories. Delete them first before deleting this category.";
    }
    return error.message;
  }
  return "An unexpected error occurred";
}

interface DeleteCategoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  category: Category | null;
}

export function DeleteCategoryDialog({
  open,
  onOpenChange,
  category,
}: DeleteCategoryDialogProps) {
  const [error, setError] = useState("");
  const deleteCategory = useDeleteCategory();
  const isDeleting = deleteCategory.isPending;

  function handleDelete() {
    if (!category) return;
    setError("");

    deleteCategory.mutate(category.id, {
      onSuccess: () => {
        toast.success(`Category "${category.name}" deleted`);
        onOpenChange(false);
      },
      onError: (err) => setError(getErrorMessage(err)),
    });
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) setError("");
    onOpenChange(nextOpen);
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Delete Category</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete &quot;{category?.name}&quot;?
            Existing transactions will keep their category reference for
            historical records.
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        )}

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => handleOpenChange(false)}
            disabled={isDeleting}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={isDeleting}
          >
            {isDeleting ? "Deleting..." : "Delete"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
