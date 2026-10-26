'use client';

import { isRecord } from '../../lib/guards';

import TimeDisplay from '../../components/TimeDisplay';

import React from 'react';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import axios from 'axios';
import config from '../../../config';
import { formatNumberWithCommas, truncateHash, formatSummaryCount, formatSummaryStake } from '../../lib/helpers';
import SearchBar from '../../components/SearchBar';
import StatusBadge from '../../components/StatusBadge';

// ── Types ────────────────────────────────────────────────────────────────────

interface EpochSlot {
  slot: number;
  status: string;
  timestamp?: string;
  proposerIndex?: string;
  proposerPubkey?: string;
  blockNumber?: string;
  transactions: number;
  gasUsed?: string;
  blockHash?: string;
}

interface EpochDetail {
  epoch: string;
  timestamp: number;
  status: string;
  validatorsCount: number | null;
  activeCount: number | null;
  pendingCount: number | null;
  exitedCount: number | null;
  slashedCount: number | null;
  totalStaked: string | null;
  startSlot: number;
  endSlot: number;
  blocks: EpochSlot[];
  finalizedEpoch: string;
  justifiedEpoch: string;
  headEpoch: string;
  proposedCount: number;
  missedCount: number;
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function parseHex(hex: string): number {
  if (!hex) return 0;
  if (hex.startsWith('0x')) return parseInt(hex, 16);
  return parseInt(hex, 10) || 0;
}

function formatGasUsed(hex: string): string {
  const val = parseHex(hex);
  return formatNumberWithCommas(val.toString());
}

export function EpochSlotRow({ slot }: { slot: EpochSlot }): JSX.Element {
  const isProposed = slot.status === 'proposed';
  const timestamp = isProposed ? parseHex(slot.timestamp || '0') : 0;
  const blockHref = isProposed && slot.blockNumber ? `/block/${slot.blockNumber}` : null;
  const proposerIndex = /^\d+$/.test(slot.proposerIndex || '') ? slot.proposerIndex : null;

  return (
    <tr className="border-b border-border last:border-b-0 hover:bg-surface transition-colors">
      <td className="px-4 py-2 tabular-nums">
        {blockHref ? (
          <Link
            href={blockHref}
            className="text-accent hover:text-accent-hover hover:underline font-medium"
          >
            {formatNumberWithCommas(slot.slot.toString())}
          </Link>
        ) : (
          <span className="text-text-muted">{formatNumberWithCommas(slot.slot.toString())}</span>
        )}
      </td>
      <td className="px-4 py-2 hidden lg:table-cell">
        {blockHref && slot.blockHash ? (
          <Link
            href={blockHref}
            className="text-text-secondary hover:text-accent hover:underline font-mono text-xs transition-colors"
            title={slot.blockHash}
          >
            {truncateHash(slot.blockHash, 10, 6)}
          </Link>
        ) : (
          <span className="text-text-muted">…</span>
        )}
      </td>
      <td className="px-4 py-2">
        <StatusBadge status={slot.status} />
      </td>
      <td className="px-4 py-2 text-text-secondary tabular-nums">
        {isProposed ? <TimeDisplay timestamp={timestamp} relative /> : '…'}
      </td>
      <td className="px-4 py-2 hidden sm:table-cell">
        {proposerIndex ? (
          <Link
            href={`/validators/${proposerIndex}`}
            className="text-text-secondary hover:text-accent hover:underline font-mono text-xs transition-colors"
          >
            {formatNumberWithCommas(proposerIndex)}
          </Link>
        ) : (
          <span className="text-text-muted">-</span>
        )}
      </td>
      <td className="px-4 py-2 text-text-secondary tabular-nums">
        {isProposed ? slot.transactions : '…'}
      </td>
      <td className="px-4 py-2 text-text-secondary tabular-nums hidden md:table-cell">
        {isProposed && slot.gasUsed ? formatGasUsed(slot.gasUsed) : '…'}
      </td>
    </tr>
  );
}

// ── Summary Row ──────────────────────────────────────────────────────────────

function SummaryRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center py-2.5 border-b border-border/70 last:border-b-0">
      <span className="text-text-muted text-[13px] w-44 flex-shrink-0 mb-0.5 sm:mb-0">
        {label}:
      </span>
      <span className="text-text-primary text-sm">{children}</span>
    </div>
  );
}

export function EpochValidatorSummary({ data }: { data: Pick<EpochDetail, 'validatorsCount' | 'activeCount' | 'pendingCount' | 'exitedCount' | 'slashedCount' | 'totalStaked'> }): JSX.Element {
  return <>
    <SummaryRow label="Validators">{formatSummaryCount(data.validatorsCount)}</SummaryRow>
    <SummaryRow label="Active">{formatSummaryCount(data.activeCount)}</SummaryRow>
    <SummaryRow label="Pending">{formatSummaryCount(data.pendingCount)}</SummaryRow>
    <SummaryRow label="Exited">{formatSummaryCount(data.exitedCount)}</SummaryRow>
    <SummaryRow label="Slashed">{formatSummaryCount(data.slashedCount)}</SummaryRow>
    <SummaryRow label="Total Staked">{formatSummaryStake(data.totalStaked)}</SummaryRow>
  </>;
}

// ── Component ────────────────────────────────────────────────────────────────

export default function EpochDetailClient({ epochId }: { epochId: string }): JSX.Element {
  const { data, isPending: loading, error: queryError } = useQuery({
    queryKey: ['epoch-detail', epochId],
    queryFn: async ({ signal }): Promise<EpochDetail> => {
      const res = await axios.get<EpochDetail>(`${config.handlerUrl}/epoch/${epochId}`, { signal, timeout: 15000 });
      return res.data;
    },
    refetchInterval: 30000,
    refetchIntervalInBackground: false,
    retry: 2,
  });
  const errorData: unknown = axios.isAxiosError(queryError) ? queryError.response?.data : undefined;
  const error = queryError ? (isRecord(errorData) && typeof errorData.error === 'string' ? errorData.error : 'Failed to load epoch data') : null;
  const epochNum = parseInt(epochId);

  const headEpoch = data ? parseInt(data.headEpoch) : 0;

  return (
    <main className="page-content py-4 sm:py-6 lg:py-8" aria-labelledby="epoch-heading">
      {/* Header with navigation */}
      <div className="flex items-center justify-between mb-4">
        <h1 id="epoch-heading" className="flex items-center gap-2 section-title">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="h-5 w-5 sm:h-6 sm:w-6"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          Epoch {formatNumberWithCommas(epochId)}
        </h1>
        <div className="flex items-center gap-2">
          {epochNum > 0 ? (
            <Link
              href={`/epoch/${epochNum - 1}`}
              className="px-3 py-1.5 rounded-lg bg-surface border border-border text-text-secondary text-sm hover:border-accent transition-colors"
            >
              &larr; Prev
            </Link>
          ) : (
            <span className="px-3 py-1.5 rounded-lg bg-surface border border-border text-text-muted text-sm cursor-not-allowed">
              &larr; Prev
            </span>
          )}
          {!loading && epochNum < headEpoch ? (
            <Link
              href={`/epoch/${epochNum + 1}`}
              className="px-3 py-1.5 rounded-lg bg-surface border border-border text-text-secondary text-sm hover:border-accent transition-colors"
            >
              Next &rarr;
            </Link>
          ) : (
            <span className="px-3 py-1.5 rounded-lg bg-surface border border-border text-text-muted text-sm cursor-not-allowed">
              Next &rarr;
            </span>
          )}
        </div>
      </div>

      <div className="mb-6">
        <SearchBar />
      </div>

      {/* Loading */}
      {loading && (
        <div className="card-simple p-8 text-center">
          <div className="animate-pulse text-text-muted">Loading epoch data...</div>
        </div>
      )}

      {/* Error */}
      {error && !loading && (
        <div className="rounded-xl bg-red-900/20 border border-red-800 p-6 text-center">
          <p className="text-error font-semibold mb-1">Epoch not found</p>
          <p className="text-text-secondary text-sm">{error}</p>
        </div>
      )}

      {/* Content */}
      {data && (
        <>
          {/* Summary Panel */}
          <div className="card-simple overflow-hidden mb-6">
            <div className="px-4 py-3 border-b border-border">
              <h2 className="text-[15px] font-display font-semibold text-text-primary">
                Epoch Details
              </h2>
            </div>
            <div className="px-4 py-2">
              <SummaryRow label="Epoch">{formatNumberWithCommas(data.epoch)}</SummaryRow>
              <SummaryRow label="Status">
                <StatusBadge status={data.status} />
              </SummaryRow>
              <SummaryRow label="Time">
                {data.timestamp > 0 ? (
                  <span>
                    <TimeDisplay timestamp={data.timestamp} relative />
                    <span className="text-text-muted ml-2">
                      (<TimeDisplay timestamp={data.timestamp} />)
                    </span>
                  </span>
                ) : (
                  '-'
                )}
              </SummaryRow>
              <EpochValidatorSummary data={data} />
              <SummaryRow label="Slots">
                {data.proposedCount + data.missedCount} ({data.proposedCount} Proposed
                {data.missedCount > 0 ? `, ${data.missedCount} Missed` : ''})
              </SummaryRow>
            </div>
          </div>

          {/* Slots Table */}
          <div className="card-simple overflow-hidden">
            <div className="px-4 py-3 border-b border-border">
              <h2 className="text-[15px] font-display font-semibold text-text-primary">Slots</h2>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border">
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-text-muted uppercase tracking-wider">
                      Slot
                    </th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden lg:table-cell">
                      Block Hash
                    </th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-text-muted uppercase tracking-wider">
                      Status
                    </th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-text-muted uppercase tracking-wider">
                      Time
                    </th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden sm:table-cell">
                      Proposer
                    </th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-text-muted uppercase tracking-wider">
                      Txns
                    </th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden md:table-cell">
                      Gas Used
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {data.blocks.map((slot) => (
                    <EpochSlotRow key={slot.slot} slot={slot} />
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </main>
  );
}
