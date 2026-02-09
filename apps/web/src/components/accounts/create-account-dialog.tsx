"use client";

import { useState } from "react";
import { toast } from "sonner";
import { ApiClientError } from "@/lib/api-client";
import {
  useCreateCashAccount,
  useCreateInvestmentAccount,
} from "@/hooks/use-accounts";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { CurrencyInput } from "@/components/ui/currency-input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const CURRENCIES = ["USD", "EUR", "GBP", "CAD", "AUD", "JPY", "INR"];

function getErrorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
    return error.message;
  }
  return "An unexpected error occurred";
}

interface CreateAccountDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateAccountDialog({
  open,
  onOpenChange,
}: CreateAccountDialogProps) {
  const [tab, setTab] = useState<string>("cash");

  // Cash account fields
  const [cashName, setCashName] = useState("");
  const [cashDescription, setCashDescription] = useState("");
  const [cashCurrency, setCashCurrency] = useState("USD");
  const [cashBalance, setCashBalance] = useState(0);

  // Investment account fields
  const [investName, setInvestName] = useState("");
  const [investDescription, setInvestDescription] = useState("");
  const [investCurrency, setInvestCurrency] = useState("USD");
  const [investBroker, setInvestBroker] = useState("");
  const [investAccountNumber, setInvestAccountNumber] = useState("");

  const [error, setError] = useState("");

  const createCash = useCreateCashAccount();
  const createInvestment = useCreateInvestmentAccount();

  const isSubmitting = createCash.isPending || createInvestment.isPending;

  function resetForm() {
    setCashName("");
    setCashDescription("");
    setCashCurrency("USD");
    setCashBalance(0);
    setInvestName("");
    setInvestDescription("");
    setInvestCurrency("USD");
    setInvestBroker("");
    setInvestAccountNumber("");
    setError("");
    setTab("cash");
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  }

  async function handleSubmitCash(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    const name = cashName.trim();
    if (!name) {
      setError("Name is required");
      return;
    }
    if (name.length > 100) {
      setError("Name must be 100 characters or less");
      return;
    }
    if (cashDescription.length > 500) {
      setError("Description must be 500 characters or less");
      return;
    }

    createCash.mutate(
      {
        name,
        description: cashDescription.trim() || undefined,
        currency: cashCurrency,
        initial_balance: cashBalance > 0 ? cashBalance : undefined,
      },
      {
        onSuccess: (account) => {
          toast.success(`Account "${account.name}" created`);
          handleOpenChange(false);
        },
        onError: (err) => setError(getErrorMessage(err)),
      }
    );
  }

  async function handleSubmitInvestment(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    const name = investName.trim();
    if (!name) {
      setError("Name is required");
      return;
    }
    if (name.length > 100) {
      setError("Name must be 100 characters or less");
      return;
    }
    if (investDescription.length > 500) {
      setError("Description must be 500 characters or less");
      return;
    }
    if (investBroker.length > 100) {
      setError("Broker must be 100 characters or less");
      return;
    }
    if (investAccountNumber.length > 50) {
      setError("Account number must be 50 characters or less");
      return;
    }

    createInvestment.mutate(
      {
        name,
        description: investDescription.trim() || undefined,
        currency: investCurrency,
        broker: investBroker.trim() || undefined,
        account_number: investAccountNumber.trim() || undefined,
      },
      {
        onSuccess: (account) => {
          toast.success(`Account "${account.name}" created`);
          handleOpenChange(false);
        },
        onError: (err) => setError(getErrorMessage(err)),
      }
    );
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create Account</DialogTitle>
          <DialogDescription>
            Add a new cash or investment account.
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        )}

        <Tabs value={tab} onValueChange={setTab}>
          <TabsList className="w-full">
            <TabsTrigger value="cash" className="flex-1">
              Cash Account
            </TabsTrigger>
            <TabsTrigger value="investment" className="flex-1">
              Investment Account
            </TabsTrigger>
          </TabsList>

          <TabsContent value="cash">
            <form onSubmit={handleSubmitCash} className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="cash-name">Name</Label>
                <Input
                  id="cash-name"
                  placeholder="e.g. Checking Account"
                  value={cashName}
                  onChange={(e) => setCashName(e.target.value)}
                  disabled={isSubmitting}
                  maxLength={100}
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="cash-description">Description</Label>
                <Input
                  id="cash-description"
                  placeholder="Optional description"
                  value={cashDescription}
                  onChange={(e) => setCashDescription(e.target.value)}
                  disabled={isSubmitting}
                  maxLength={500}
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="cash-currency">Currency</Label>
                <Select
                  value={cashCurrency}
                  onValueChange={setCashCurrency}
                  disabled={isSubmitting}
                >
                  <SelectTrigger id="cash-currency" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {CURRENCIES.map((c) => (
                      <SelectItem key={c} value={c}>
                        {c}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="cash-balance">Initial Balance</Label>
                <CurrencyInput
                  id="cash-balance"
                  value={cashBalance}
                  onChange={setCashBalance}
                  placeholder="0.00"
                  disabled={isSubmitting}
                />
              </div>
              <DialogFooter>
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting ? "Creating..." : "Create Account"}
                </Button>
              </DialogFooter>
            </form>
          </TabsContent>

          <TabsContent value="investment">
            <form
              onSubmit={handleSubmitInvestment}
              className="flex flex-col gap-4"
            >
              <div className="flex flex-col gap-2">
                <Label htmlFor="invest-name">Name</Label>
                <Input
                  id="invest-name"
                  placeholder="e.g. Brokerage Account"
                  value={investName}
                  onChange={(e) => setInvestName(e.target.value)}
                  disabled={isSubmitting}
                  maxLength={100}
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="invest-description">Description</Label>
                <Input
                  id="invest-description"
                  placeholder="Optional description"
                  value={investDescription}
                  onChange={(e) => setInvestDescription(e.target.value)}
                  disabled={isSubmitting}
                  maxLength={500}
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="invest-currency">Currency</Label>
                <Select
                  value={investCurrency}
                  onValueChange={setInvestCurrency}
                  disabled={isSubmitting}
                >
                  <SelectTrigger id="invest-currency" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {CURRENCIES.map((c) => (
                      <SelectItem key={c} value={c}>
                        {c}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="invest-broker">Broker</Label>
                <Input
                  id="invest-broker"
                  placeholder="e.g. Fidelity"
                  value={investBroker}
                  onChange={(e) => setInvestBroker(e.target.value)}
                  disabled={isSubmitting}
                  maxLength={100}
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="invest-account-number">Account Number</Label>
                <Input
                  id="invest-account-number"
                  placeholder="Optional account number"
                  value={investAccountNumber}
                  onChange={(e) => setInvestAccountNumber(e.target.value)}
                  disabled={isSubmitting}
                  maxLength={50}
                />
              </div>
              <DialogFooter>
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting ? "Creating..." : "Create Account"}
                </Button>
              </DialogFooter>
            </form>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
