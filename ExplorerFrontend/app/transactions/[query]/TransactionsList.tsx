'use client';

import { useMemo } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { formatAmount, timeAgo, truncateHash } from '../../lib/helpers';
import SearchBar from '../../components/SearchBar';
import Pagination from '../../components/Pagination';
import CopyButton from '../../components/CopyButton';
import Badge from '../../components/Badge';
import EmptyState from '../../components/EmptyState';
import type { TransactionsListProps } from '@/app/types';

const ITEMS_PER_PAGE = 10;

export default function TransactionsList({
  initialData,
  currentPage,
}: TransactionsListProps): JSX.Element {
  const router = useRouter();
  // Render straight from props; the previous local mirror state served no
  // purpose (no setter is called) and the useEffect-resync tripped the new
  // set-state-in-effect rule. Memo for stable identity in case downstream
  // components ever .map over it with referential keys.
  const transactions = useMemo(() => initialData.txs, [initialData.txs]);
  const totalPages = Math.max(1, Math.ceil(initialData.total / ITEMS_PER_PAGE));

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
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Amount</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Time</th>
                  </tr>
                </thead>
                <tbody>
                  {transactions.map((tx) => {
                    const [formattedAmount, unit] = formatAmount(tx.Amount);
                    const isContractCall = parseFloat(String(tx.Amount)) === 0;

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
