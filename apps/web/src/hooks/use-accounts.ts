import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { Account } from "@/types/models";
import type {
  PageResponse,
  PaginationParams,
  AccountResponse,
  CreateCashAccountRequest,
  CreateInvestmentAccountRequest,
  CreateCreditCardAccountRequest,
  UpdateAccountRequest,
} from "@/types/api";

export const accountKeys = {
  all: ["accounts"] as const,
  lists: () => [...accountKeys.all, "list"] as const,
  list: (params?: PaginationParams) =>
    [...accountKeys.lists(), params] as const,
  details: () => [...accountKeys.all, "detail"] as const,
  detail: (id: number) => [...accountKeys.details(), id] as const,
};

export function useAccounts(params?: PaginationParams) {
  return useQuery({
    queryKey: accountKeys.list(params),
    queryFn: () =>
      apiClient.get<PageResponse<Account>>("/api/v1/accounts", {
        ...params,
      }),
  });
}

export function useAccount(id: number) {
  return useQuery({
    queryKey: accountKeys.detail(id),
    queryFn: async () => {
      const res = await apiClient.get<AccountResponse>(`/api/v1/accounts/${id}`);
      return res.account;
    },
    enabled: id > 0,
  });
}

export function useCreateCashAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateCashAccountRequest) => {
      const res = await apiClient.post<AccountResponse>(
        "/api/v1/accounts/cash",
        data
      );
      return res.account;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: accountKeys.lists() });
    },
  });
}

export function useCreateInvestmentAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateInvestmentAccountRequest) => {
      const res = await apiClient.post<AccountResponse>(
        "/api/v1/accounts/investment",
        data
      );
      return res.account;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: accountKeys.lists() });
    },
  });
}

export function useCreateCreditCardAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateCreditCardAccountRequest) => {
      const res = await apiClient.post<AccountResponse>(
        "/api/v1/accounts/credit-card",
        data
      );
      return res.account;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: accountKeys.lists() });
    },
  });
}

export function useUpdateAccount(id: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: UpdateAccountRequest) => {
      const res = await apiClient.put<AccountResponse>(
        `/api/v1/accounts/${id}`,
        data
      );
      return res.account;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: accountKeys.lists() });
      queryClient.invalidateQueries({ queryKey: accountKeys.detail(id) });
    },
  });
}
