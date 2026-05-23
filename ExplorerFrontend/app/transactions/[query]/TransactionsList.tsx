'use client';

import { useMemo } from 'react';
import axios from 'axios';
import { useQuery } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { formatAmount, timeAgo, truncateHash } from '../../lib/helpers';
import SearchBar from '../../components/SearchBar';
import Pagination from '../../components/Pagination';
import CopyButton from '../../components/CopyButton';
import Badge from '../../components/Badge';
import EmptyState from '../../components/EmptyState';
import config from '../../../config';
import type { TransactionsListProps, TransactionsResponse } from '@/app/types';

const ITEMS_PER_PAGE = 10;

export default function TransactionsList({
  initialData,
  currentPage,
}: TransactionsListProps): JSX.Element {
  const router = useRouter();
  // Page-1 polls the network-wide feed on roughly block time so newly
  // mined txs appear without a manual reload. Later pages are historical
  // and stay static (no rug-pull mid-scroll). Mirrors the iter 33 pattern
  // on /blocks.
  const isHead = currentPage === 1;
  const { data } = useQuery<TransactionsResponse>({
    queryKey: ['txs', currentPage],
    queryFn: async () => {
      const res = await axios.get(`${config.handlerUrl}/txs?page=${currentPage}`);
      return res.data;
    },
    initialData,
    staleTime: isHead ? 0 : 60000,
    gcTime: 5 * 60 * 1000,
    retry: 2,
    refetchInterval: isHead ? 15000 : false,
    refetchIntervalInBackground: false,
  });
  const transactions = useMemo(() => data?.txs ?? initialData.txs, [data?.txs, initialData.txs]);
  const totalPages = Math.max(1, Math.ceil((data?.total ?? initialData.total) / ITEMS_PER_PAGE));

  const goToNextPage = (): void => {
    router.push(`/transactions/${Math.min(currentPage + 1, totalPages)}`);
  };

  const goToPreviousPage = (): void => {
    router.push(`/transactions/${Math.max(currentPage - 1, 1)}`);
  };

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      <h1 className="text-xl sm:text-2xl font-bold mb-4 text-[#ffa729]">Transactions</h1>

      <div className="mb-6">
        <SearchBar />
      </div>

      {transactions.length === 0 ? (
        <EmptyState
          title="No transactions found"
          description="There are no transactions to display on this page."
          actionLabel="View latest transactions"
          actionHref="/transactions/1"
        />
      ) : (
        <>
          <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden mb-6">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-[#2a2a2a]">
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Hash</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Type</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden lg:table-cell">From</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden lg:table-cell">To</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden md:table-cell">Block</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Amount</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Time</th>
                  </tr>
                </thead>
                <tbody>
                  {transactions.map((tx) => {
                    const [formattedAmount, unit] = formatAmount(tx.Amount);
                    const isContractCall = parseFloat(String(tx.Amount)) === 0;
                    // The API capitalises field names (BlockNumber/From/To)
                    // while the index signature on Transaction allows both
                    // cases. Parse the block number defensively for hex/decimal.
                    const blockRaw = (tx.BlockNumber ?? tx.blockNumber) as string | number | undefined;
                    const blockNum = (() => {
                      if (blockRaw === undefined || blockRaw === null) return null;
                      if (typeof blockRaw === 'number') return blockRaw;
                      if (typeof blockRaw === 'string' && blockRaw.startsWith('0x')) {
                        const n = parseInt(blockRaw, 16);
                        return Number.isFinite(n) ? n : null;
                      }
                      const n = parseInt(String(blockRaw), 10);
                      return Number.isFinite(n) ? n : null;
                    })();
                    const fromAddr = (tx.From ?? tx.from) as string | undefined;
                    const toAddr = (tx.To ?? tx.to) as string | undefined;

                    return (
                      <tr
                        key={tx.TxHash}
                        className="border-b border-[#2a2a2a] last:border-b-0 hover:bg-[#252525] transition-colors"
                      >
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-1.5">
                            <Link
                              href={`/tx/${tx.TxHash}?from=transactions&page=${currentPage}`}
                              className="text-[#ffa729] hover:text-[#ffb954] hover:underline font-mono text-xs"
                              title={tx.TxHash}
                            >
                              {truncateHash(tx.TxHash, 10, 6)}
                            </Link>
                            <CopyButton value={tx.TxHash} label="Copy hash" size="sm" stopPropagation />
                          </div>
                        </td>
                        <td className="px-4 py-3 hidden sm:table-cell">
                          {isContractCall ? (
                            <Badge variant="neutral">Contract Call</Badge>
                          ) : (
                            <Badge variant="brand">Transfer</Badge>
                          )}
                        </td>
                        <td className="px-4 py-3 hidden lg:table-cell">
                          {fromAddr ? (
                            <Link
                              href={`/address/${fromAddr}`}
                              className="text-gray-400 hover:text-[#ffa729] font-mono text-xs transition-colors"
                              title={fromAddr}
                            >
                              {truncateHash(fromAddr, 8, 6)}
                            </Link>
                          ) : (
                            <span className="text-gray-600 text-xs">-</span>
                          )}
                        </td>
                        <td className="px-4 py-3 hidden lg:table-cell">
                          {toAddr ? (
                            <Link
                              href={`/address/${toAddr}`}
                              className="text-gray-400 hover:text-[#ffa729] font-mono text-xs transition-colors"
                              title={toAddr}
                            >
                              {truncateHash(toAddr, 8, 6)}
                            </Link>
                          ) : (
                            <span className="text-gray-600 text-xs">-</span>
                          )}
                        </td>
                        <td className="px-4 py-3 hidden md:table-cell">
                          {blockNum !== null ? (
                            <Link
                              href={`/block/${blockNum}`}
                              className="text-[#ffa729] hover:text-[#ffb954] hover:underline tabular-nums text-xs"
                            >
                              {blockNum.toLocaleString()}
                            </Link>
                          ) : (
                            <span className="text-gray-600 text-xs">-</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-gray-300 tabular-nums whitespace-nowrap">
                          {formattedAmount}
                          <span className="text-gray-500 text-xs ml-1">{unit}</span>
                        </td>
                        <td className="px-4 py-3 text-gray-400 tabular-nums">
                          {timeAgo(tx.TimeStamp)}
                        </td>
                      </tr>
                    );
                  })}
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
        </>
      )}
    </div>
  );
}
