'use client';

import { useQuery } from '@tanstack/react-query';
import { fetchChainTiming } from './chainTiming';

export function useChainTiming() {
  return useQuery({
    queryKey: ['chain-timing'],
    queryFn: ({ signal }) => fetchChainTiming(signal),
    staleTime: 60_000,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
    retry: 2,
  });
}
