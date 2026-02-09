"use client";

import { LogOut } from "lucide-react";
import { useAuth } from "@/hooks/use-auth";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { Separator } from "@/components/ui/separator";

function getUserInitials(firstName: string, lastName: string): string {
  const first = firstName?.[0] ?? "";
  const last = lastName?.[0] ?? "";
  return (first + last).toUpperCase() || "U";
}

export function AppHeader() {
  const { user, logout } = useAuth();

  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="mr-2 h-4" />
      <div className="flex-1" />
      <DropdownMenu>
        <DropdownMenuTrigger className="flex items-center gap-2 rounded-md px-2 py-1 hover:bg-accent outline-none">
          <Avatar size="sm">
            <AvatarFallback>
              {user
                ? getUserInitials(user.first_name, user.last_name)
                : "U"}
            </AvatarFallback>
          </Avatar>
          <span className="text-sm font-medium hidden sm:inline">
            {user
              ? [user.first_name, user.last_name].filter(Boolean).join(" ") ||
                user.email
              : "User"}
          </span>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuLabel>
            {user?.email ?? "Account"}
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={logout}>
            <LogOut />
            Log out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>
  );
}
