"use client";

import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { Plus } from "lucide-react";
import { useAuth } from "@/hooks/use-auth";
import { AppSidebar } from "@/components/layout/app-sidebar";
import { AppHeader } from "@/components/layout/app-header";
import {
  CommandPalette,
  type CommandPaletteRef,
} from "@/components/layout/command-palette";
import { CreateTransactionDialog } from "@/components/transactions/create-transaction-dialog";
import { Button } from "@/components/ui/button";
import { SidebarProvider, SidebarInset } from "@/components/ui/sidebar";
import { Skeleton } from "@/components/ui/skeleton";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { isLoading, isAuthenticated } = useAuth();
  const router = useRouter();
  const commandPaletteRef = useRef<CommandPaletteRef>(null);
  const [quickAddOpen, setQuickAddOpen] = useState(false);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <Skeleton className="h-8 w-32" />
          <Skeleton className="h-4 w-48" />
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <AppHeader
          onOpenCommandPalette={() => commandPaletteRef.current?.toggle()}
        />
        <main className="flex-1 overflow-auto scrollbar-slim">
          {/* Mobile bottom padding clears the floating add button. */}
          <div className="mx-auto w-full max-w-[1600px] px-4 pt-5 pb-24 sm:px-6 sm:py-7 lg:px-8">
            {children}
          </div>
        </main>
      </SidebarInset>
      <CommandPalette ref={commandPaletteRef} />
      {/* Mobile: thumb-reachable quick add on every page. */}
      <Button
        size="icon-lg"
        onClick={() => setQuickAddOpen(true)}
        className="fixed right-4 bottom-[calc(1rem+env(safe-area-inset-bottom))] z-30 rounded-full shadow-lg sm:hidden"
      >
        <Plus className="size-6" />
        <span className="sr-only">Add transaction</span>
      </Button>
      <CreateTransactionDialog
        open={quickAddOpen}
        onOpenChange={setQuickAddOpen}
      />
    </SidebarProvider>
  );
}
