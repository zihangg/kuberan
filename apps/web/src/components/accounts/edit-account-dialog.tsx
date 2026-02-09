"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import { useUpdateAccount } from "@/hooks/use-accounts";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { Account } from "@/types/models";

const ACCOUNT_TYPE_LABELS: Record<string, string> = {
  cash: "Cash",
  investment: "Investment",
  debt: "Debt",
  credit_card: "Credit Card",
};

function getErrorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
    if (
      error.code === "INVALID_INPUT" &&
      (error.message.toLowerCase().includes("already exists") ||
        error.message.toLowerCase().includes("duplicate"))
    ) {
      return "An account with this name already exists";
    }
    return error.message;
  }
  return "An unexpected error occurred";
}

interface EditAccountDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  account: Account | null;
}

export function EditAccountDialog({
  open,
  onOpenChange,
  account,
}: EditAccountDialogProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");

  const updateAccount = useUpdateAccount(account?.id ?? 0);
  const isSubmitting = updateAccount.isPending;

  // Sync form state when account changes
  useEffect(() => {
    if (account) {
      setName(account.name);
      setDescription(account.description ?? "");
      setError("");
    }
  }, [account]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!account) return;
    setError("");

    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("Name is required");
      return;
    }
    if (trimmedName.length > 100) {
      setError("Name must be 100 characters or less");
      return;
    }
    if (description.length > 500) {
      setError("Description must be 500 characters or less");
      return;
    }

    // Build payload with only changed fields
    const payload: Record<string, unknown> = {};
    if (trimmedName !== account.name) payload.name = trimmedName;
    const newDesc = description.trim();
    if (newDesc !== (account.description ?? "")) payload.description = newDesc;

    if (Object.keys(payload).length === 0) {
      onOpenChange(false);
      return;
    }

    updateAccount.mutate(payload, {
      onSuccess: (updated) => {
        toast.success(`Account "${updated.name}" updated`);
        onOpenChange(false);
      },
      onError: (err) => setError(getErrorMessage(err)),
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Edit Account</DialogTitle>
          <DialogDescription>Update account details.</DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label>Type</Label>
            <div className="flex items-center gap-2">
              <Badge variant="secondary">
                {ACCOUNT_TYPE_LABELS[account?.type ?? ""] ?? account?.type}
              </Badge>
              <span className="text-sm text-muted-foreground">
                {account?.currency}
              </span>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-account-name">Name</Label>
            <Input
              id="edit-account-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={isSubmitting}
              maxLength={100}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-account-description">Description</Label>
            <Input
              id="edit-account-description"
              placeholder="Optional description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={isSubmitting}
              maxLength={500}
            />
          </div>

          <DialogFooter>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Saving..." : "Save Changes"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
