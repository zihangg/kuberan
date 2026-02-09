import { tokenStore } from "./api-client";

// Re-export token utilities for convenience
export const {
  getAccessToken,
  setAccessToken,
  getRefreshToken,
  setRefreshToken,
  clearTokens,
} = tokenStore;

// JWT claims (only the fields we care about)
interface JwtPayload {
  exp?: number; // expiry as unix timestamp (seconds)
  sub?: string; // subject (user id)
}

/**
 * Parse JWT payload without verifying signature.
 * Returns null if the token is malformed.
 */
export function parseJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = JSON.parse(atob(parts[1])) as JwtPayload;
    return payload;
  } catch {
    return null;
  }
}

/**
 * Check if a JWT token is expired (or will expire within bufferMs).
 * Returns true if expired/invalid, false if still valid.
 */
export function isTokenExpired(token: string, bufferMs = 30_000): boolean {
  const payload = parseJwtPayload(token);
  if (!payload?.exp) return true;
  return Date.now() >= payload.exp * 1000 - bufferMs;
}

/**
 * Check if the user has a non-expired access token stored.
 */
export function isAuthenticated(): boolean {
  const token = getAccessToken();
  if (!token) return false;
  return !isTokenExpired(token);
}

// Cookie name used by middleware for fast route protection
export const AUTH_COOKIE_NAME = "kuberan_auth";

/**
 * Set a simple auth flag cookie (not the actual token) for middleware route protection.
 * The cookie is not httpOnly so JS can manage it, but contains no sensitive data.
 */
export function setAuthCookie(): void {
  if (typeof document === "undefined") return;
  document.cookie = `${AUTH_COOKIE_NAME}=1; path=/; max-age=${7 * 24 * 60 * 60}; SameSite=Lax`;
}

/**
 * Remove the auth flag cookie on logout.
 */
export function clearAuthCookie(): void {
  if (typeof document === "undefined") return;
  document.cookie = `${AUTH_COOKIE_NAME}=; path=/; max-age=0`;
}
