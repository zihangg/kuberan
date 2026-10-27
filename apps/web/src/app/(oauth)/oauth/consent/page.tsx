"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { apiClient, ApiClientError } from "@/lib/api-client";
import type {
  OAuthConsentAcceptRequest,
  OAuthConsentDetails,
  OAuthRedirectResponse,
} from "@/types/api";
import { isProtocolScope, isWriteScope, scopeLabel } from "@/lib/oauth-scopes";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

// Show the host of the client's redirect target — the anti-phishing signal.
function redirectHost(redirectUris?: string[]): string | null {
  const uri = redirectUris?.[0];
  if (!uri) return null;
  try {
    return new URL(uri).host;
  } catch {
    return uri;
  }
}

// Read scopes start ticked; write scopes only when the user approved them for
// this client before (plan 022: write access is always an explicit opt-in).
function initialSelection(details: OAuthConsentDetails): Set<string> {
  const previous = new Set(details.previously_approved ?? []);
  return new Set(
    (details.requested_scopes ?? []).filter(
      (scope) =>
        !isProtocolScope(scope) && (!isWriteScope(scope) || previous.has(scope))
    )
  );
}

function ScopeCheckbox({
  scope,
  checked,
  disabled,
  onChange,
}: {
  scope: string;
  checked: boolean;
  disabled: boolean;
  onChange: (scope: string, checked: boolean) => void;
}) {
  return (
    <label className="flex items-center gap-2 text-sm">
      <input
        type="checkbox"
        className="border-input size-4 rounded"
        checked={checked}
        onChange={(e) => onChange(scope, e.target.checked)}
        disabled={disabled}
      />
      <span className="text-muted-foreground">{scopeLabel(scope)}</span>
    </label>
  );
}

function OAuthConsent() {
  const searchParams = useSearchParams();
  const consentChallenge = searchParams.get("consent_challenge") ?? "";

  const [details, setDetails] = useState<OAuthConsentDetails | null>(null);
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [rememberClient, setRememberClient] = useState(true);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (!consentChallenge) {
      setError("Missing consent challenge. Start the connection from your MCP client.");
      setIsLoading(false);
      return;
    }

    let cancelled = false;
    (async () => {
      try {
        const res = await apiClient.get<OAuthConsentDetails>(
          "/api/v1/oauth/consent",
          { consent_challenge: consentChallenge }
        );
        if (cancelled) return;
        // Trusted client: Hydra accepted server-side, just follow the redirect.
        if (res.redirect_to) {
          window.location.href = res.redirect_to;
          return;
        }
        setDetails(res);
        setSelected(initialSelection(res));
      } catch (err) {
        if (cancelled) return;
        setError(
          err instanceof ApiClientError
            ? err.message
            : "An unexpected error occurred"
        );
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [consentChallenge]);

  async function handleAccept() {
    setError("");
    setIsSubmitting(true);
    try {
      const body: OAuthConsentAcceptRequest = {
        consent_challenge: consentChallenge,
        remember_client: rememberClient,
        granted_scopes: Array.from(selected),
      };
      const res = await apiClient.post<OAuthRedirectResponse>(
        "/api/v1/oauth/consent/accept",
        body
      );
      window.location.href = res.redirect_to;
    } catch (err) {
      setError(
        err instanceof ApiClientError
          ? err.message
          : "An unexpected error occurred"
      );
      setIsSubmitting(false);
    }
  }

  function toggleScope(scope: string, checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(scope);
      else next.delete(scope);
      return next;
    });
  }

  async function handleDeny() {
    setError("");
    setIsSubmitting(true);
    try {
      const res = await apiClient.post<OAuthRedirectResponse>(
        "/api/v1/oauth/consent/reject",
        { consent_challenge: consentChallenge }
      );
      window.location.href = res.redirect_to;
    } catch (err) {
      setError(
        err instanceof ApiClientError
          ? err.message
          : "An unexpected error occurred"
      );
      setIsSubmitting(false);
    }
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">Authorize access</CardTitle>
          <CardDescription>Loading request details...</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  if (error && !details) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">Authorize access</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        </CardContent>
      </Card>
    );
  }

  const clientName = details?.client?.client_name || details?.client?.client_id || "An application";
  const host = redirectHost(details?.redirect_uris);
  const scopes = details?.requested_scopes ?? [];
  const readScopes = scopes.filter((s) => !isProtocolScope(s) && !isWriteScope(s));
  const writeScopes = scopes.filter(isWriteScope);
  const protocolScopes = scopes.filter(isProtocolScope);
  const grantsWrite = writeScopes.some((s) => selected.has(s));

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-2xl">Authorize access</CardTitle>
        <CardDescription>
          <span className="text-foreground font-medium">{clientName}</span> wants
          to connect to your Kuberan account.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md p-3 text-sm">
            {error}
          </div>
        )}
        {host && (
          <p className="text-muted-foreground text-sm">
            Redirects to{" "}
            <span className="text-foreground font-mono">{host}</span>
          </p>
        )}
        <div className="flex flex-col gap-2">
          <p className="text-sm font-medium">This will allow it to:</p>
          {readScopes.map((scope) => (
            <ScopeCheckbox
              key={scope}
              scope={scope}
              checked={selected.has(scope)}
              disabled={isSubmitting}
              onChange={toggleScope}
            />
          ))}
          {protocolScopes.length > 0 && (
            <ul className="flex flex-col gap-1">
              {Array.from(new Set(protocolScopes.map(scopeLabel))).map((label) => (
                <li key={label} className="text-muted-foreground text-sm">
                  • {label}
                </li>
              ))}
            </ul>
          )}
        </div>
        {writeScopes.length > 0 && (
          <div className="flex flex-col gap-2 rounded-md border border-amber-500/40 p-3">
            <p className="text-sm font-medium">Allow changes</p>
            <p className="text-muted-foreground text-xs">
              Off unless you tick them. {clientName} will be able to change your
              financial records. You can disconnect it in Settings at any time.
            </p>
            {writeScopes.map((scope) => (
              <ScopeCheckbox
                key={scope}
                scope={scope}
                checked={selected.has(scope)}
                disabled={isSubmitting}
                onChange={toggleScope}
              />
            ))}
          </div>
        )}
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="border-input size-4 rounded"
            checked={rememberClient}
            onChange={(e) => setRememberClient(e.target.checked)}
            disabled={isSubmitting}
          />
          <span>Remember this application (skip this screen next time)</span>
        </label>
      </CardContent>
      <CardFooter className="flex flex-col gap-2 pt-2">
        <Button
          type="button"
          className="w-full"
          onClick={handleAccept}
          disabled={isSubmitting}
        >
          {isSubmitting
            ? "Authorizing..."
            : grantsWrite
              ? "Authorize, including changes"
              : "Authorize"}
        </Button>
        <Button
          type="button"
          variant="outline"
          className="w-full"
          onClick={handleDeny}
          disabled={isSubmitting}
        >
          Deny
        </Button>
      </CardFooter>
    </Card>
  );
}

export default function OAuthConsentPage() {
  return (
    <Suspense fallback={null}>
      <OAuthConsent />
    </Suspense>
  );
}
