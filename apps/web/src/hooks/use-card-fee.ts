import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { CardAnnualFee, CardFeeSummary } from "@/types/models";
import type { AnnualFeeResponse, SetAnnualFeeRequest } from "@/types/api";
import { accountKeys } from "./use-accounts";
import { reminderKeys } from "./use-reminders";

export const cardFeeKeys = {
  all: ["card-fee"] as const,
  forAccount: (accountId: string) =>
    [...cardFeeKeys.all, "account", accountId] as const,
};

/** The annual fee and waiver progress of a credit card. */
export function useFeeWaiver(accountId: string) {
  return useQuery({
    queryKey: cardFeeKeys.forAccount(accountId),
    queryFn: () =>
      apiClient.get<CardFeeSummary>(`/api/v1/accounts/${accountId}/fee-waiver`),
    enabled: !!accountId,
  });
}

function useInvalidateCardFee(accountId: string) {
  const queryClient = useQueryClient();
  return () => {
    queryClient.invalidateQueries({ queryKey: accountKeys.all });
    queryClient.invalidateQueries({ queryKey: reminderKeys.forAccount(accountId) });
    queryClient.invalidateQueries({ queryKey: cardFeeKeys.forAccount(accountId) });
  };
}

/** Set or replace a card's annual fee and waiver condition. */
export function useSetAnnualFee(accountId: string) {
  const invalidate = useInvalidateCardFee(accountId);
  return useMutation({
    mutationFn: async (data: SetAnnualFeeRequest): Promise<CardAnnualFee> => {
      const res = await apiClient.put<AnnualFeeResponse>(
        `/api/v1/accounts/${accountId}/annual-fee`,
        data,
      );
      return res.annual_fee;
    },
    onSuccess: invalidate,
  });
}

/** Remove a card's annual fee. */
export function useDeleteAnnualFee(accountId: string) {
  const invalidate = useInvalidateCardFee(accountId);
  return useMutation({
    mutationFn: () => apiClient.del<void>(`/api/v1/accounts/${accountId}/annual-fee`),
    onSuccess: invalidate,
  });
}
