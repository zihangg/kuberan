import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { Transaction } from "@/types/models";
import type {
  PageResponse,
  TransactionResponse,
  DeleteResponse,
  TransactionFilters,
  CreateTransactionRequest,
  CreateTransferRequest,
} from "@/types/api";
import { accountKeys } from "./use-accounts";

export const transactionKeys = {
  all: ["transactions"] as const,
  lists: () => [...transactionKeys.all, "list"] as const,
  listByAccount: (accountId: number, filters?: TransactionFilters) =>
    [...transactionKeys.lists(), "account", accountId, filters] as const,
  details: () => [...transactionKeys.all, "detail"] as const,
  detail: (id: number) => [...transactionKeys.details(), id] as const,
};

export function useAccountTransactions(
  accountId: number,
  filters?: TransactionFilters
) {
  return useQuery({
    queryKey: transactionKeys.listByAccount(accountId, filters),
    queryFn: () =>
      apiClient.get<PageResponse<Transaction>>(
        `/api/v1/accounts/${accountId}/transactions`,
        { ...filters }
      ),
    enabled: accountId > 0,
  });
}

export function useTransaction(id: number) {
  return useQuery({
    queryKey: transactionKeys.detail(id),
    queryFn: async () => {
      const res = await apiClient.get<TransactionResponse>(
        `/api/v1/transactions/${id}`
      );
      return res.transaction;
    },
    enabled: id > 0,
  });
}

export function useCreateTransaction() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateTransactionRequest) => {
      const res = await apiClient.post<TransactionResponse>(
        "/api/v1/transactions",
        data
      );
      return res.transaction;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: transactionKeys.lists() });
      queryClient.invalidateQueries({
        queryKey: accountKeys.detail(variables.account_id),
      });
      queryClient.invalidateQueries({ queryKey: accountKeys.lists() });
    },
  });
}

export function useCreateTransfer() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateTransferRequest) => {
      const res = await apiClient.post<TransactionResponse>(
        "/api/v1/transactions/transfer",
        data
      );
      return res.transaction;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: transactionKeys.lists() });
      queryClient.invalidateQueries({
        queryKey: accountKeys.detail(variables.from_account_id),
      });
      queryClient.invalidateQueries({
        queryKey: accountKeys.detail(variables.to_account_id),
      });
      queryClient.invalidateQueries({ queryKey: accountKeys.lists() });
    },
  });
}

export function useDeleteTransaction() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: number) => {
      await apiClient.del<DeleteResponse>(`/api/v1/transactions/${id}`);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: transactionKeys.lists() });
      queryClient.invalidateQueries({ queryKey: accountKeys.all });
    },
  });
}
