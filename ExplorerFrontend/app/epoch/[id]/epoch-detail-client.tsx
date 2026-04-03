'use client';

import React, { useState, useEffect } from 'react';
import Link from 'next/link';
import axios from 'axios';
import config from '../../../config';
import { formatNumberWithCommas, truncateHash, formatAddress, formatTimestamp } from '../../lib/helpers';
import SearchBar from '../../components/SearchBar';

// ── Types ────────────────────────────────────────────────────────────────────

interface EpochSlot {
  slot: number;
  status: string;
  timestamp?: string;
  proposer?: string;
  transactions: number;
  gasUsed?: string;
  blockHash?: string;
}

interface EpochDetail {
  epoch: string;
  timestamp: number;
  status: string;
  validatorsCount: number;
  activeCount: number;
  pendingCount: number;
  exitedCount: number;
  slashedCount: number;
  totalStaked: string;
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

function timeAgo(unixSeconds: number): string {
  const now = Math.floor(Date.now() / 1000);
  const diff = now - unixSeconds;
  if (diff < 0) return 'just now';
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

function formatStaked(gwei: string): string {
  try {
    const val = BigInt(gwei || '0');
    const qrl = val / BigInt(1_000_000_000);
    return formatNumberWithCommas(qrl.toString()) + ' QRL';
  } catch {
    return '0 QRL';
  }
}

function formatGasUsed(hex: string): string {
  const val = parseHex(hex);
  return formatNumberWithCommas(val.toString());
}

// ── Status Badge ─────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    finalized: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/20',
    justified: 'bg-blue-500/15 text-blue-400 border-blue-500/20',
    pending: 'bg-yellow-500/15 text-yellow-400 border-yellow-500/20',
    proposed: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/20',
    missed: 'bg-red-500/15 text-red-400 border-red-500/20',
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium border ${styles[status] || styles.pending}`}>
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  );
}

// ── Summary Row ──────────────────────────────────────────────────────────────

function SummaryRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center py-2.5 border-b border-[#2a2a2a] last:border-b-0">
      <span className="text-gray-500 text-sm w-40 flex-shrink-0 mb-0.5 sm:mb-0">{label}:</span>
      <span className="text-gray-200 text-sm">{children}</span>
    </div>
  );
}

// ── Component ────────────────────────────────────────────────────────────────

export default function EpochDetailClient({ epochId }: { epochId: string }): JSX.Element {
  const [data, setData] = useState<EpochDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const epochNum = parseInt(epochId);

  useEffect(() => {
    async function fetchData() {
      try {
        setLoading(true);
        setError(null);
        const res = await axios.get(`${config.handlerUrl}/epoch/${epochId}`);
        setData(res.data);
      } catch (err: any) {
        setError(err.response?.data?.error || 'Failed to load epoch data');
      } finally {
        setLoading(false);
      }
    }
    fetchData();
  }, [epochId]);

  const headEpoch = data ? parseInt(data.headEpoch) : 0;

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      {/* Header with navigation */}
      <div className="flex items-center justify-between mb-4">
        <h1 className="flex items-center gap-2 text-xl sm:text-2xl font-bold text-[#ffa729]">
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 sm:h-6 sm:w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          Epoch {formatNumberWithCommas(epochId)}
        </h1>
        <div className="flex items-center gap-2">
          {epochNum > 0 ? (
            <Link
              href={`/epoch/${epochNum - 1}`}
              className="px-3 py-1.5 rounded-lg bg-[#1e1e1e] border border-[#2a2a2a] text-gray-300 text-sm hover:border-[#ffa729] transition-colors"
            >
              &larr; Prev
            </Link>
          ) : (
            <span className="px-3 py-1.5 rounded-lg bg-[#1e1e1e] border border-[#2a2a2a] text-gray-600 text-sm cursor-not-allowed">
              &larr; Prev
            </span>
          )}
          {!loading && epochNum < headEpoch ? (
            <Link
              href={`/epoch/${epochNum + 1}`}
              className="px-3 py-1.5 rounded-lg bg-[#1e1e1e] border border-[#2a2a2a] text-gray-300 text-sm hover:border-[#ffa729] transition-colors"
            >
              Next &rarr;
            </Link>
          ) : (
            <span className="px-3 py-1.5 rounded-lg bg-[#1e1e1e] border border-[#2a2a2a] text-gray-600 text-sm cursor-not-allowed">
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
        <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] p-8 text-center">
          <div className="animate-pulse text-gray-500">Loading epoch data...</div>
        </div>
      )}

      {/* Error */}
      {error && !loading && (
        <div className="rounded-xl bg-red-900/20 border border-red-800 p-6 text-center">
          <p className="text-red-400 font-semibold mb-1">Epoch not found</p>
          <p className="text-gray-400 text-sm">{error}</p>
        </div>
      )}

      {/* Content */}
      {data && !loading && (
        <>
          {/* Summary Panel */}
          <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden mb-6">
            <div className="px-4 py-3 border-b border-[#2a2a2a]">
              <h2 className="text-[15px] font-semibold text-[#ffa729]">Epoch Details</h2>
            </div>
            <div className="px-4 py-2">
              <SummaryRow label="Epoch">{formatNumberWithCommas(data.epoch)}</SummaryRow>
              <SummaryRow label="Status"><StatusBadge status={data.status} /></SummaryRow>
              <SummaryRow label="Time">
                {data.timestamp > 0 ? (
                  <span>
                    {timeAgo(data.timestamp)}
                    <span className="text-gray-500 ml-2">({formatTimestamp(data.timestamp)})</span>
                  </span>
                ) : '—'}
              </SummaryRow>
              <SummaryRow label="Validators">{formatNumberWithCommas(data.validatorsCount.toString())}</SummaryRow>
              <SummaryRow label="Active">{formatNumberWithCommas(data.activeCount.toString())}</SummaryRow>
              {data.pendingCount > 0 && <SummaryRow label="Pending">{data.pendingCount}</SummaryRow>}
              {data.exitedCount > 0 && <SummaryRow label="Exited">{data.exitedCount}</SummaryRow>}
              {data.slashedCount > 0 && <SummaryRow label="Slashed">{data.slashedCount}</SummaryRow>}
              <SummaryRow label="Total Staked">{formatStaked(data.totalStaked)}</SummaryRow>
              <SummaryRow label="Slots">
                {data.proposedCount + data.missedCount} ({data.proposedCount} Proposed{data.missedCount > 0 ? `, ${data.missedCount} Missed` : ''})
              </SummaryRow>
            </div>
          </div>

          {/* Slots Table */}
          <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden">
            <div className="px-4 py-3 border-b border-[#2a2a2a]">
              <h2 className="text-[15px] font-semibold text-[#ffa729]">Slots</h2>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-[#2a2a2a]">
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Slot</th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Status</th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Time</th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Proposer</th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Txns</th>
                    <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden md:table-cell">Gas Used</th>
                  </tr>
                </thead>
                <tbody>
                  {data.blocks.map((slot) => {
                    const isProposed = slot.status === 'proposed';
                    const timestamp = isProposed ? parseHex(slot.timestamp || '0') : 0;
                    const proposer = isProposed ? formatAddress(slot.proposer || '') : '';

                    return (
                      <tr
                        key={slot.slot}
                        className="border-b border-[#2a2a2a] last:border-b-0 hover:bg-[#252525] transition-colors"
                      >
                        <td className="px-4 py-2 tabular-nums">
                          {isProposed ? (
                            <Link href={`/block/${slot.slot}`} className="text-[#ffa729] hover:text-[#ffb954] hover:underline font-medium">
                              {formatNumberWithCommas(slot.slot.toString())}
                            </Link>
                          ) : (
                            <span className="text-gray-600">{formatNumberWithCommas(slot.slot.toString())}</span>
                          )}
                        </td>
                        <td className="px-4 py-2">
                          <StatusBadge status={slot.status} />
                        </td>
                        <td className="px-4 py-2 text-gray-400 tabular-nums">
                          {isProposed ? timeAgo(timestamp) : '—'}
                        </td>
                        <td className="px-4 py-2 hidden sm:table-cell">
                          {isProposed && proposer ? (
                            <Link href={`/address/${proposer}`} className="text-gray-400 hover:text-[#ffa729] hover:underline font-mono text-xs transition-colors">
                              {truncateHash(proposer, 8, 6)}
                            </Link>
                          ) : (
                            <span className="text-gray-600">—</span>
                          )}
                        </td>
                        <td className="px-4 py-2 text-gray-300 tabular-nums">
                          {isProposed ? slot.transactions : '—'}
                        </td>
                        <td className="px-4 py-2 text-gray-400 tabular-nums hidden md:table-cell">
                          {isProposed && slot.gasUsed ? formatGasUsed(slot.gasUsed) : '—'}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
