"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Unplug } from "lucide-react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useConnectedApps, useDisconnectApp } from "@/hooks/use-oauth-clients";
import { ApiClientError } from "@/lib/api-client";
import { formatDateTime } from "@/lib/format";
import { isWriteScope, scopeLabel } from "@/lib/oauth-scopes";
import type { ConnectedApp } from "@/types/api";

function ConnectedAppRow({
  app,
  onDisconnect,
}: {
  app: ConnectedApp;
  onDisconnect: (app: ConnectedApp) => void;
}) {
  const canWrite = app.scopes.some(isWriteScope);

  return (
    <div className="flex flex-col gap-3 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0 space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <p className="truncate font-medium">
            {app.client_name || app.client_id}
          </p>
          <Badge variant={canWrite ? "default" : "secondary"}>
            {canWrite ? "Can make changes" : "Read only"}
          </Badge>
        </div>
        <ul className="space-y-0.5">
          {app.scopes.map((scope) => (
            <li key={scope} className="text-sm text-muted-foreground">
              {scopeLabel(scope)}
            </li>
          ))}
        </ul>
        <p className="text-xs text-muted-foreground">
          Connected {formatDateTime(app.created_at)}
          {app.last_used_at
            ? ` · Last change ${formatDateTime(app.last_used_at)}`
            : ""}
        </p>
      </div>
      <Button
        variant="outline"
        size="sm"
        className="shrink-0"
        onClick={() => onDisconnect(app)}
      >
        <Unplug className="size-4" />
        Disconnect
      </Button>
    </div>
  );
}

export function ConnectedAppsSettings() {
  const { data: apps, isLoading } = useConnectedApps();
  const disconnect = useDisconnectApp();
  const [pending, setPending] = useState<ConnectedApp | null>(null);

  async function confirmDisconnect() {
    if (!pending) return;
    const name = pending.client_name || pending.client_id;
    try {
      await disconnect.mutateAsync(pending.client_id);
      toast.success(`${name} disconnected`);
    } catch (err) {
      toast.error("Failed to disconnect", {
        description:
          err instanceof ApiClientError ? err.message : "Please try again",
      });
    } finally {
      setPending(null);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Connected apps</CardTitle>
        <CardDescription>
          Apps such as Claude that you have connected to Kuberan through
          MCP. Disconnecting stops changes immediately and read access within
          15 minutes.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-5 w-40" />
            <Skeleton className="h-4 w-64" />
          </div>
        ) : !apps || apps.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No apps are connected.
          </p>
        ) : (
          <div className="divide-y divide-border/60">
            {apps.map((app) => (
              <ConnectedAppRow
                key={app.client_id}
                app={app}
                onDisconnect={setPending}
              />
            ))}
          </div>
        )}
      </CardContent>

      <AlertDialog
        open={pending !== null}
        onOpenChange={(open) => !open && setPending(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Disconnect {pending?.client_name || pending?.client_id}?
            </AlertDialogTitle>
            <AlertDialogDescription>
              It will no longer be able to read or change your data, and you
              will see the consent screen if you connect it again.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => void confirmDisconnect()}
              disabled={disconnect.isPending}
            >
              Disconnect
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
