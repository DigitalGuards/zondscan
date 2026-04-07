'use client';

import React from 'react';
import Link from 'next/link';
import axios from 'axios';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { formatNumberWithCommas, timeAgo, formatStaked } from '../../lib/helpers';
import SearchBar from '../../components/SearchBar';
import StatusBadge from '../../components/StatusBadge';
import Pagination from '../../components/Pagination';

interface EpochItem {
  epoch: string;
  timestamp: number;
  status: string;
  validatorsCount: number;
  activeCount: number;
  totalStaked: string;
}

interface EpochsData {
  epochs: EpochItem[];
  total: number;
  finalizedEpoch: string;
  justifiedEpoch: string;
}

const ITEMS_PER_PAGE = 15;

const fetchEpochs = async (page: string): Promise<EpochsData> => {
  const response = await axios.get<EpochsData>(`${config.handlerUrl}/epochs?page=${page}&limit=${ITEMS_PER_PAGE}`);
  return response.data;
};

interface EpochsClientProps {
  initialData: EpochsData;
  initialPage: string;
}

export default function EpochsClient({ initialData, initialPage }: EpochsClientProps): JSX.Element {
  const router = useRouter();
  const currentPage = parseInt(initialPage);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['epochs', initialPage],
    queryFn: () => fetchEpochs(initialPage),
    staleTime: 60000,
    gcTime: 5 * 60 * 1000,
    retry: 2,
    initialData,
  });

  const totalPages = data ? Math.max(1, Math.ceil(data.total / ITEMS_PER_PAGE)) : 1;

  const goToNextPage = (): void => {
    router.push(`/epochs/${Math.min(currentPage + 1, totalPages)}`);
  };

  const goToPreviousPage = (): void => {
    router.push(`/epochs/${Math.max(currentPage - 1, 1)}`);
  };

  if (isError) {
    return (
      <div className="p-4 sm:p-8 max-w-6xl mx-auto">
        <h1 className="text-xl sm:text-2xl font-bold mb-4 text-[#ffa729]">Epochs</h1>
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-xl">
          <p className="font-bold">Error:</p>
          <p className="text-sm">{error instanceof Error ? error.message : 'Failed to load epochs'}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      <h1 className="text-xl sm:text-2xl font-bold mb-4 text-[#ffa729]">Epochs</h1>

      <div className="mb-6">
        <SearchBar />
      </div>

      <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden mb-6">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[#2a2a2a]">
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Epoch</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Time</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Status</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Validators</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Active</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden md:table-cell">Total Staked</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: ITEMS_PER_PAGE }).map((_, i) => (
                  <tr key={i} className="border-b border-[#2a2a2a] last:border-b-0">
                    <td className="px-4 py-3"><div className="h-4 w-12 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3 hidden sm:table-cell"><div className="h-4 w-10 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3 hidden sm:table-cell"><div className="h-4 w-10 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3 hidden md:table-cell"><div className="h-4 w-24 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  </tr>
                ))
              ) : !data?.epochs?.length ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-gray-500">
                    No epoch data available yet. The explorer is still syncing.
                  </td>
                </tr>
              ) : (
                data.epochs.map((epoch) => (
                  <tr
                    key={epoch.epoch}
                    className="border-b border-[#2a2a2a] last:border-b-0 hover:bg-[#252525] transition-colors"
                  >
                    <td className="px-4 py-3">
                      <Link href={`/epoch/${epoch.epoch}`} className="text-[#ffa729] hover:text-[#ffb954] hover:underline font-medium tabular-nums">
                        {formatNumberWithCommas(epoch.epoch)}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-gray-400 tabular-nums">{timeAgo(epoch.timestamp)}</td>
                    <td className="px-4 py-3">
                      <StatusBadge status={epoch.status} />
                    </td>
                    <td className="px-4 py-3 text-gray-300 tabular-nums hidden sm:table-cell">
                      {formatNumberWithCommas(epoch.validatorsCount.toString())}
                    </td>
                    <td className="px-4 py-3 text-gray-300 tabular-nums hidden sm:table-cell">
                      {formatNumberWithCommas(epoch.activeCount.toString())}
                    </td>
                    <td className="px-4 py-3 text-gray-400 tabular-nums hidden md:table-cell">
                      {formatStaked(epoch.totalStaked)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      <Pagination
        currentPage={currentPage}
        totalPages={totalPages}
        onPrevious={goToPreviousPage}
        onNext={goToNextPage}
      />
    </div>
  );
}
