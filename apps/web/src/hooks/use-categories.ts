import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { Category, CategoryType } from "@/types/models";
import type { PageResponse, PaginationParams } from "@/types/api";

export const categoryKeys = {
  all: ["categories"] as const,
  lists: () => [...categoryKeys.all, "list"] as const,
  list: (params?: PaginationParams & { type?: CategoryType }) =>
    [...categoryKeys.lists(), params] as const,
};

export function useCategories(
  params?: PaginationParams & { type?: CategoryType }
) {
  return useQuery({
    queryKey: categoryKeys.list(params),
    queryFn: () =>
      apiClient.get<PageResponse<Category>>("/api/v1/categories", {
        ...params,
      }),
  });
}
