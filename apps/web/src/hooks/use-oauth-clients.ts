import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { ConnectedAppsResponse } from "@/types/api";

export const oauthClientKeys = {
  all: ["oauth-clients"] as const,
  list: () => [...oauthClientKeys.all, "list"] as const,
};

export function useConnectedApps() {
  return useQuery({
    queryKey: oauthClientKeys.list(),
    queryFn: async () => {
      const res = await apiClient.get<ConnectedAppsResponse>(
        "/api/v1/oauth/clients"
      );
      return res.clients;
    },
  });
}

export function useDisconnectApp() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (clientId: string) =>
      apiClient.del<void>(
        `/api/v1/oauth/clients/${encodeURIComponent(clientId)}`
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: oauthClientKeys.all });
    },
  });
}
