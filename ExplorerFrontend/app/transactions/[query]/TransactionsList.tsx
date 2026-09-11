'use client';
import InterfaceText from '../../components/InterfaceText';

import TimeDisplay from '../../components/TimeDisplay';
import AddressText from '../../components/AddressText';

import { useMemo } from 'react';
import axios from 'axios';
import { useQuery } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { truncateHash } from '../../lib/helpers';
import TransactionAmount from '../../components/TransactionAmount';
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
    <div className="py-4 sm:py-6 lg:py-8">
      <h1 className="section-title mb-4"><InterfaceText text="Transactions" /></h1>

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
          <div className="card-simple overflow-hidden mb-6">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border">
                    <th className="text-left px-2 2xl:px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider"><InterfaceText text="Hash" /></th>
                    <th className="text-left px-2 2xl:px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden sm:[display:table-cell]">Type</th>
                    <th className="text-left px-2 2xl:px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden xl:[display:table-cell]"><InterfaceText text="From" /></th>
                    <th className="text-left px-2 2xl:px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden xl:[display:table-cell]"><InterfaceText text="To" /></th>
                    <th className="text-left px-2 2xl:px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden md:[display:table-cell]"><InterfaceText text="Block" /></th>
                    <th className="text-right px-2 2xl:px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider"><InterfaceText text="Amount" /></th>
                    <th className="text-left px-2 2xl:px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider"><InterfaceText text="Time" /></th>
                  </tr>
                </thead>
                <tbody>
                  {transactions.map((tx) => {
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
                        className="border-b border-border last:border-b-0 hover:bg-surface transition-colors"
                      >
                        <td className="px-2 2xl:px-4 py-3">
                          <div className="flex items-center gap-1.5">
                            <Link
                              href={`/tx/${tx.TxHash}?from=transactions&page=${currentPage}`}
                              className="text-accent hover:text-accent-hover hover:underline font-mono text-xs"
                              title={tx.TxHash}
                            >
                              <span className="sm:hidden">{truncateHash(tx.TxHash, 6, 4)}</span>
                              <span className="hidden sm:inline">{truncateHash(tx.TxHash, 10, 6)}</span>
                            </Link>
                            <span className="hidden sm:inline-flex">
                              <CopyButton value={tx.TxHash} label="Copy hash" size="sm" stopPropagation />
                            </span>
                          </div>
                        </td>
                        <td className="px-2 2xl:px-4 py-3 hidden sm:[display:table-cell]">
                          {isContractCall ? (
                            <Badge variant="neutral">Contract Call</Badge>
                          ) : (
                            <Badge variant="brand">Transfer</Badge>
                          )}
                        </td>
                        <td className="px-2 2xl:px-4 py-3 hidden xl:[display:table-cell]">
                          {fromAddr ? (
                            <Link
                              href={`/address/${fromAddr}`}
                              className="text-text-secondary hover:text-accent font-mono text-xs transition-colors"
                              title={fromAddr}
                            >
                              <AddressText address={fromAddr} />
                            </Link>
                          ) : (
                            <span className="text-text-muted text-xs">-</span>
                          )}
                        </td>
                        <td className="px-2 2xl:px-4 py-3 hidden xl:[display:table-cell]">
                          {toAddr ? (
                            <Link
                              href={`/address/${toAddr}`}
                              className="text-text-secondary hover:text-accent font-mono text-xs transition-colors"
                              title={toAddr}
                            >
                              <AddressText address={toAddr} />
                            </Link>
                          ) : (
                            <span className="text-text-muted text-xs">-</span>
                          )}
                        </td>
                        <td className="px-2 2xl:px-4 py-3 hidden md:[display:table-cell]">
                          {blockNum !== null ? (
                            <Link
                              href={`/block/${blockNum}`}
                              className="text-accent hover:text-accent-hover hover:underline tabular-nums text-xs"
                            >
                              {blockNum.toLocaleString()}
                            </Link>
                          ) : (
                            <span className="text-text-muted text-xs">-</span>
                          )}
                        </td>
                        <td className="px-2 2xl:px-4 text-right py-3 text-text-secondary tabular-nums whitespace-nowrap">
                          <TransactionAmount amount={tx.Amount} />
                        </td>
                        <td className="px-2 2xl:px-4 py-3 text-text-secondary tabular-nums whitespace-nowrap">
                          <TimeDisplay timestamp={tx.TimeStamp} relative />
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
