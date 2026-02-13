import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { Budget } from "@/types/models";
import type {
  PageResponse,
  BudgetResponse,
  BudgetProgressResponse,
  BudgetFilters,
  CreateBudgetRequest,
  UpdateBudgetRequest,
  DeleteResponse,
} from "@/types/api";

export const budgetKeys = {
  all: ["budgets"] as const,
  lists: () => [...budgetKeys.all, "list"] as const,
  list: (filters?: BudgetFilters) => [...budgetKeys.lists(), filters] as const,
  details: () => [...budgetKeys.all, "detail"] as const,
  detail: (id: string) => [...budgetKeys.details(), id] as const,
  progress: (id: string) =>
    [...budgetKeys.all, "progress", id] as const,
};

export function useBudgets(filters?: BudgetFilters) {
  return useQuery({
    queryKey: budgetKeys.list(filters),
    queryFn: () =>
      apiClient.get<PageResponse<Budget>>("/api/v1/budgets", {
        ...filters,
      }),
  });
}

export function useBudget(id: string) {
  return useQuery({
    queryKey: budgetKeys.detail(id),
    queryFn: async () => {
      const res = await apiClient.get<BudgetResponse>(
        `/api/v1/budgets/${id}`
      );
      return res.budget;
    },
    enabled: !!id,
  });
}

export function useBudgetProgress(id: string) {
  return useQuery({
    queryKey: budgetKeys.progress(id),
    queryFn: async () => {
      const res = await apiClient.get<BudgetProgressResponse>(
        `/api/v1/budgets/${id}/progress`
      );
      return res.progress;
    },
    enabled: !!id,
  });
}

export function useCreateBudget() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateBudgetRequest) => {
      const res = await apiClient.post<BudgetResponse>(
        "/api/v1/budgets",
        data
      );
      return res.budget;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.lists() });
    },
  });
}

export function useUpdateBudget(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: UpdateBudgetRequest) => {
      const res = await apiClient.put<BudgetResponse>(
        `/api/v1/budgets/${id}`,
        data
      );
      return res.budget;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.lists() });
      queryClient.invalidateQueries({ queryKey: budgetKeys.detail(id) });
    },
  });
}

export function useDeleteBudget() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const res = await apiClient.del<DeleteResponse>(
        `/api/v1/budgets/${id}`
      );
      return res;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: budgetKeys.lists() });
    },
  });
}
