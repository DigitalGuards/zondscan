'use client';

import { useEffect, useMemo, useState } from 'react';
import axios from 'axios';
import Link from 'next/link';
import {
  createColumnHelper,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  useReactTable,
} from '@tanstack/react-table';
import Badge from '../../../components/Badge';
import DebouncedInput from '../../../components/DebouncedInput';
import ImageWithFallback from '../../../components/ImageWithFallback';
import config from '../../../../config';
import {
  PAGE_SIZE,
  Paginator,
  renderTableBody,
  renderTableHeader,
} from './_table-utils';

/**
 * NFT holdings panel (QRC-721 + QRC-1155). Lazy-fetches `/address/:addr/nfts`
 * on mount. The endpoint already joins tokenBalances with tokenMetadata so
 * each row carries the resolved name + image when off-chain metadata has
 * been fetched.
 */

interface NFTBalanceRow {
  contractAddress: string;
  tokenID: string;
  tokenStandard: string;
  balance: string;
  collectionName?: string;
  collectionSymbol?: string;
  name?: string;
  image?: string;
  externalURL?: string;
}

const NFT_STANDARDS = new Set(['ERC-721', 'ERC-1155']);
const columnHelper = createColumnHelper<NFTBalanceRow>();

interface NftsPanelProps {
  address: string;
  onLoaded?: (count: number) => void;
}

export default function NftsPanel({ address, onLoaded }: NftsPanelProps): JSX.Element {
  const [rows, setRows] = useState<NFTBalanceRow[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    let cancelled = false;
    setRows([]);
    setLoaded(false);
    setError(null);
    setFilter('');
    setLoading(true);
    (async () => {
      try {
        const res = await axios.get(`${config.handlerUrl}/address/${address}/nfts`);
        if (cancelled) return;
        const arr: NFTBalanceRow[] = Array.isArray(res.data?.nfts) ? res.data.nfts : [];
        const next = arr.filter((n) => NFT_STANDARDS.has(n.tokenStandard));
        setRows(next);
        setLoaded(true);
        onLoaded?.(next.length);
      } catch (err) {
        if (cancelled) return;
        console.error('Failed to load NFTs:', err);
        setError('Failed to load NFTs');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [address, onLoaded]);

  const columns = useMemo(
    () => [
      columnHelper.accessor(
        (row) =>
          `${row.collectionName ?? ''} ${row.collectionSymbol ?? ''} ${row.name ?? ''} ${row.contractAddress} #${row.tokenID}`,
        {
          id: 'NFT',
          header: 'NFT',
          cell: (info) => {
            const r = info.row.original;
            const tokenLabel = r.name || `#${r.tokenID}`;
            const collectionLabel =
              r.collectionName || r.collectionSymbol || r.contractAddress;
            return (
              <div className="flex items-center gap-3 min-w-0">
                {r.image ? (
                  // Off-chain metadata images live on arbitrary HTTP / IPFS
                  // gateways. ImageWithFallback swaps in the tokenID monogram
                  // on load failure; unoptimized inside the component skips
                  // next/image's domain allow-list.
                  <ImageWithFallback
                    src={r.image}
                    alt=""
                    width={40}
                    height={40}
                    className="w-10 h-10 rounded-md object-cover bg-[#1a1a1a]"
                    fallback={
                      <div className="w-10 h-10 rounded-md bg-[#1a1a1a] border border-[#3d3d3d] flex items-center justify-center text-xs font-mono text-gray-500">
                        {(r.tokenID || '?').slice(0, 3)}
                      </div>
                    }
                  />
                ) : (
                  <div className="w-10 h-10 rounded-md bg-[#1a1a1a] border border-[#3d3d3d] flex items-center justify-center text-xs font-mono text-gray-500">
                    {(r.tokenID || '?').slice(0, 3)}
                  </div>
                )}
                <div className="min-w-0">
                  <Link
                    href={`/address/${r.contractAddress}`}
                    className="text-sm text-[#ffa729] hover:text-[#ffb954] transition-colors font-medium break-all"
                    title={r.contractAddress}
                  >
                    {collectionLabel}
                  </Link>
                  <div className="text-xs text-gray-400 font-mono break-all">
                    {tokenLabel}
                    {r.name && r.tokenID && ` (#${r.tokenID})`}
                  </div>
                </div>
              </div>
            );
          },
        },
      ),
      columnHelper.accessor((row) => row.tokenStandard, {
        id: 'Standard',
        header: 'Standard',
        cell: (info) => {
          const r = info.row.original;
          const standardBadge = r.tokenStandard === 'ERC-1155' ? 'QRC-1155' : 'QRC-721';
          const qty = (() => {
            try {
              return BigInt(r.balance).toLocaleString('en-US');
            } catch {
              return r.balance;
            }
          })();
          return (
            <div className="flex items-center gap-2">
              <Badge variant="warning">{standardBadge}</Badge>
              {r.tokenStandard === 'ERC-1155' && (
                <span className="text-sm font-mono text-gray-200">x{qty}</span>
              )}
            </div>
          );
        },
      }),
    ],
    [],
  );

  const table = useReactTable({
    data: rows,
    columns,
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageIndex: 0, pageSize: PAGE_SIZE } },
  });

  if (loading && !loaded) {
    return <div className="p-8 text-center text-gray-400">Loading NFTs...</div>;
  }
  if (error) {
    return <div className="p-8 text-center text-red-400">{error}</div>;
  }
  if (loaded && rows.length === 0) {
    return <div className="p-8 text-center text-gray-400">No NFTs held by this address.</div>;
  }

  return (
    <div className="w-full">
      <div className="p-4 border-b border-[#3d3d3d]">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="text-sm text-gray-400">
            {rows.length} NFT{rows.length === 1 ? '' : 's'} held
          </div>
          <DebouncedInput
            value={filter}
            onChange={(v) => setFilter(String(v))}
            className="px-4 py-2 text-sm bg-[#1a1a1a] border border-[#3d3d3d] rounded-lg focus:outline-none focus:border-[#ffa729] text-white w-full md:w-auto"
            placeholder="Search by collection, tokenID, address"
          />
        </div>
      </div>

      {table.getRowModel().rows.length === 0 ? (
        <div className="p-6 text-center text-sm text-gray-400">No NFTs match your search.</div>
      ) : (
        <div className="overflow-x-auto">
          <table aria-label="NFT holdings table" className="w-full">
            <thead>{renderTableHeader(table)}</thead>
            <tbody>{renderTableBody(table)}</tbody>
          </table>
        </div>
      )}

      <Paginator
        pageIndex={table.getState().pagination.pageIndex}
        pageCount={table.getPageCount()}
        canPrev={table.getCanPreviousPage()}
        canNext={table.getCanNextPage()}
        goFirst={() => table.setPageIndex(0)}
        goPrev={() => table.previousPage()}
        goNext={() => table.nextPage()}
        goLast={() => table.setPageIndex(table.getPageCount() - 1)}
      />
    </div>
  );
}
