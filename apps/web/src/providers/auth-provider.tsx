"use client";

import {
  createContext,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { ReactNode } from "react";
import { toast } from "sonner";
import type { User } from "@/types/models";
import type {
  AuthResponse,
  LoginRequest,
  RegisterRequest,
  ProfileResponse,
} from "@/types/api";
import { apiClient, ApiClientError } from "@/lib/api-client";
import {
  clearTokens,
  revokeSession,
  setAccessToken,
  setRefreshToken,
  getAccessToken,
  setAuthCookie,
  clearAuthCookie,
} from "@/lib/auth";

export interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  hideBalances: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => void;
  toggleHideBalances: () => Promise<void>;
  /** Account preselected for new transactions, or null when none is set. */
  defaultAccountId: string | null;
  /**
   * Merges server-confirmed profile fields into the session user. Only the
   * given fields change, so an in-flight optimistic update to another field
   * is not overwritten.
   */
  patchUser: (fields: Partial<User>) => void;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const isAuthenticated = user !== null;
  const hideBalances = user?.hide_balances ?? false;
  const defaultAccountId = user?.default_account_id ?? null;

  // Fetch user profile using stored tokens. Returns null only when the
  // session is really over; transient failures (offline, server restarting)
  // are retried and then rethrown so the stored session is kept.
  const fetchProfile = useCallback(async (): Promise<User | null> => {
    for (let attempt = 0; ; attempt++) {
      try {
        const res = await apiClient.get<ProfileResponse>("/api/v1/profile");
        return res.user;
      } catch (err) {
        const transient =
          !(err instanceof ApiClientError) || err.status === 0 || err.status >= 500;
        if (!transient) return null;
        if (attempt >= 2) throw err;
        await new Promise((resolve) => setTimeout(resolve, 1000 * 2 ** attempt));
      }
    }
  }, []);

  // On mount: try to restore session from stored tokens
  useEffect(() => {
    async function init() {
      const token = getAccessToken();
      if (!token) {
        setIsLoading(false);
        return;
      }

      try {
        const profile = await fetchProfile();
        if (profile) {
          setUser(profile);
          setAuthCookie();
        } else {
          clearTokens();
          clearAuthCookie();
        }
      } catch {
        // Server unreachable: keep the tokens so a reload can restore the
        // session instead of forcing a fresh login.
        toast.error("Could not reach the server");
      }
      setIsLoading(false);
    }

    init();
  }, [fetchProfile]);

  const login = useCallback(
    async (email: string, password: string) => {
      const data: LoginRequest = { email, password };
      const res = await apiClient.post<AuthResponse>(
        "/api/v1/auth/login",
        data
      );
      setAccessToken(res.access_token);
      setRefreshToken(res.refresh_token);
      setAuthCookie();
      setUser(res.user);
    },
    []
  );

  const register = useCallback(
    async (data: RegisterRequest) => {
      const res = await apiClient.post<AuthResponse>(
        "/api/v1/auth/register",
        data
      );
      setAccessToken(res.access_token);
      setRefreshToken(res.refresh_token);
      setAuthCookie();
      setUser(res.user);
    },
    []
  );

  const toggleHideBalances = useCallback(async () => {
    setUser((prev) => {
      if (!prev) return prev;
      const next = !prev.hide_balances;
      apiClient
        .patch<ProfileResponse>("/api/v1/profile", { hide_balances: next })
        .catch(() => {
          setUser((current) =>
            current ? { ...current, hide_balances: !next } : current
          );
          toast.error("Failed to update balance visibility setting");
        });
      return { ...prev, hide_balances: next };
    });
  }, []);

  const patchUser = useCallback((fields: Partial<User>) => {
    setUser((prev) => (prev ? { ...prev, ...fields } : prev));
  }, []);

  const logout = useCallback(() => {
    void revokeSession();
    clearTokens();
    clearAuthCookie();
    setUser(null);
    if (typeof window !== "undefined") {
      window.location.href = "/login";
    }
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      isLoading,
      isAuthenticated,
      hideBalances,
      login,
      register,
      logout,
      toggleHideBalances,
      defaultAccountId,
      patchUser,
    }),
    [
      user,
      isLoading,
      isAuthenticated,
      hideBalances,
      login,
      register,
      logout,
      toggleHideBalances,
      defaultAccountId,
      patchUser,
    ]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
