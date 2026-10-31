"use client";

import { useState } from "react";
import { toast } from "sonner";
import { RefreshCw, Unplug } from "lucide-react";
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

function appName(app: ConnectedApp): string {
  return app.client_name || app.client_id;
}

function ConnectedAppRow({
  app,
  onDisconnect,
}: {
  app: ConnectedApp;
  onDisconnect: (app: ConnectedApp) => void;
}) {
  const canWrite = app.scopes.some(isWriteScope);
  const details = app.legacy
    ? "Connected before per-app permissions; reconnect to choose what it can do"
    : [
        app.created_at && `Connected ${formatDateTime(app.created_at)}`,
        // last_used_at only moves when the app makes a change, not on reads.
        app.last_used_at && `Last change ${formatDateTime(app.last_used_at)}`,
      ]
        .filter(Boolean)
        .join(" · ");

  return (
    <div className="flex flex-col gap-3 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0 space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <p className="truncate font-medium">{appName(app)}</p>
          <Badge variant={canWrite ? "default" : "secondary"}>
            {canWrite ? "Can make changes" : "Read only"}
          </Badge>
        </div>
        {app.scopes.length > 0 && (
          <ul className="space-y-0.5">
            {app.scopes.map((scope) => (
              <li key={scope} className="text-sm text-muted-foreground">
                {scopeLabel(scope)}
              </li>
            ))}
          </ul>
        )}
        {details && <p className="text-xs text-muted-foreground">{details}</p>}
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

function ConnectedAppsList({
  onDisconnect,
}: {
  onDisconnect: (app: ConnectedApp) => void;
}) {
  const { data: apps, isLoading, isError, refetch, isRefetching } =
    useConnectedApps();

  if (isLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-4 w-64" />
      </div>
    );
  }

  // Never show "no apps" when the list could not be loaded: this is the page
  // people use to revoke access.
  if (isError) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p className="text-sm text-destructive">
          Couldn&apos;t load your connected apps.
        </p>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void refetch()}
          disabled={isRefetching}
        >
          <RefreshCw className="size-4" />
          Try again
        </Button>
      </div>
    );
  }

  if (!apps || apps.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">No apps are connected.</p>
    );
  }

  return (
    <div className="divide-y divide-border/60">
      {apps.map((app) => (
        <ConnectedAppRow
          key={app.client_id}
          app={app}
          onDisconnect={onDisconnect}
        />
      ))}
    </div>
  );
}

export function ConnectedAppsSettings() {
  const disconnect = useDisconnectApp();
  const [pending, setPending] = useState<ConnectedApp | null>(null);

  async function confirmDisconnect() {
    if (!pending) return;
    try {
      await disconnect.mutateAsync(pending.client_id);
      toast.success(`${appName(pending)} disconnected`);
      setPending(null);
    } catch (err) {
      // Keep the dialog open so the user can retry.
      toast.error("Failed to disconnect", {
        description:
          err instanceof ApiClientError ? err.message : "Please try again",
      });
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Connected apps</CardTitle>
        <CardDescription>
          Apps such as Claude that you have connected to Kuberan through MCP.
          Disconnecting stops changes immediately and read access within 15
          minutes.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <ConnectedAppsList onDisconnect={setPending} />
      </CardContent>

      <AlertDialog
        open={pending !== null}
        onOpenChange={(open) => {
          if (!open && !disconnect.isPending) setPending(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Disconnect {pending ? appName(pending) : ""}?
            </AlertDialogTitle>
            <AlertDialogDescription>
              It will no longer be able to read or change your data, and you
              will see the consent screen if you connect it again.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={disconnect.isPending}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={disconnect.isPending}
              onClick={(e) => {
                // Radix closes the dialog on click; keep it open until the
                // request settles so a failure can be retried.
                e.preventDefault();
                void confirmDisconnect();
              }}
            >
              {disconnect.isPending ? "Disconnecting..." : "Disconnect"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
