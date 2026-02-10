import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { Security, SecurityPrice } from "@/types/models";
import type {
  PageResponse,
  PaginationParams,
  SecurityResponse,
  SecurityFilters,
} from "@/types/api";

export const securityKeys = {
  all: ["securities"] as const,
  lists: () => [...securityKeys.all, "list"] as const,
  list: (params?: SecurityFilters) =>
    [...securityKeys.lists(), params] as const,
  details: () => [...securityKeys.all, "detail"] as const,
  detail: (id: number) => [...securityKeys.details(), id] as const,
  prices: (id: number, from: string, to: string) =>
    [...securityKeys.all, "prices", id, from, to] as const,
};

export function useSecurities(params?: SecurityFilters) {
  return useQuery({
    queryKey: securityKeys.list(params),
    queryFn: () =>
      apiClient.get<PageResponse<Security>>("/api/v1/securities", {
        ...params,
      }),
  });
}

export function useSecurity(id: number) {
  return useQuery({
    queryKey: securityKeys.detail(id),
    queryFn: async () => {
      const res = await apiClient.get<SecurityResponse>(
        `/api/v1/securities/${id}`
      );
      return res.security;
    },
    enabled: id > 0,
  });
}

export function useSecurityPriceHistory(
  id: number,
  from: string,
  to: string,
  params?: PaginationParams
) {
  return useQuery({
    queryKey: securityKeys.prices(id, from, to),
    queryFn: () =>
      apiClient.get<PageResponse<SecurityPrice>>(
        `/api/v1/securities/${id}/prices`,
        { from_date: from, to_date: to, ...params }
      ),
    enabled: id > 0 && from.length > 0 && to.length > 0,
  });
}
