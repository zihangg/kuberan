import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { Category, CategoryType } from "@/types/models";
import type {
  PageResponse,
  PaginationParams,
  CategoryResponse,
  CreateCategoryRequest,
  UpdateCategoryRequest,
  DeleteResponse,
} from "@/types/api";

export const categoryKeys = {
  all: ["categories"] as const,
  lists: () => [...categoryKeys.all, "list"] as const,
  list: (params?: PaginationParams & { type?: CategoryType }) =>
    [...categoryKeys.lists(), params] as const,
  details: () => [...categoryKeys.all, "detail"] as const,
  detail: (id: string) => [...categoryKeys.details(), id] as const,
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

export function useCategory(id: string) {
  return useQuery({
    queryKey: categoryKeys.detail(id),
    queryFn: async () => {
      const res = await apiClient.get<CategoryResponse>(
        `/api/v1/categories/${id}`
      );
      return res.category;
    },
    enabled: !!id,
  });
}

export function useCreateCategory() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateCategoryRequest) => {
      const res = await apiClient.post<CategoryResponse>(
        "/api/v1/categories",
        data
      );
      return res.category;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoryKeys.lists() });
    },
  });
}

export function useUpdateCategory(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: UpdateCategoryRequest) => {
      const res = await apiClient.put<CategoryResponse>(
        `/api/v1/categories/${id}`,
        data
      );
      return res.category;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoryKeys.lists() });
      queryClient.invalidateQueries({ queryKey: categoryKeys.detail(id) });
    },
  });
}

export function useDeleteCategory() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const res = await apiClient.del<DeleteResponse>(
        `/api/v1/categories/${id}`
      );
      return res;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoryKeys.lists() });
    },
  });
}
