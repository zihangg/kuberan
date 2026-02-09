import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { ProfileResponse } from "@/types/api";

export const profileKeys = {
  all: ["profile"] as const,
};

export function useProfile() {
  return useQuery({
    queryKey: profileKeys.all,
    queryFn: async () => {
      const res = await apiClient.get<ProfileResponse>("/api/v1/profile");
      return res.user;
    },
  });
}
