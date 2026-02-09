"use client";

import {
  createContext,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { ReactNode } from "react";
import type { User } from "@/types/models";
import type {
  AuthResponse,
  LoginRequest,
  RegisterRequest,
  ProfileResponse,
} from "@/types/api";
import { apiClient } from "@/lib/api-client";
import {
  clearTokens,
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
  login: (email: string, password: string) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const isAuthenticated = user !== null;

  // Fetch user profile using stored tokens
  const fetchProfile = useCallback(async (): Promise<User | null> => {
    try {
      const res = await apiClient.get<ProfileResponse>("/api/v1/profile");
      return res.user;
    } catch {
      return null;
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

      const profile = await fetchProfile();
      if (profile) {
        setUser(profile);
        setAuthCookie();
      } else {
        clearTokens();
        clearAuthCookie();
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

  const logout = useCallback(() => {
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
      login,
      register,
      logout,
    }),
    [user, isLoading, isAuthenticated, login, register, logout]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
