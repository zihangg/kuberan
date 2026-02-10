import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import type { PortfolioSnapshot } from "@/types/models";
import type { PageResponse, PortfolioSnapshotFilters } from "@/types/api";

export const snapshotKeys = {
  all: ["snapshots"] as const,
  lists: () => [...snapshotKeys.all, "list"] as const,
  list: (params: PortfolioSnapshotFilters) =>
    [...snapshotKeys.lists(), params] as const,
};

export function usePortfolioSnapshots(params: PortfolioSnapshotFilters) {
  return useQuery({
    queryKey: snapshotKeys.list(params),
    queryFn: () =>
      apiClient.get<PageResponse<PortfolioSnapshot>>(
        "/api/v1/investments/snapshots",
        { ...params }
      ),
    enabled: params.from_date.length > 0 && params.to_date.length > 0,
  });
}
