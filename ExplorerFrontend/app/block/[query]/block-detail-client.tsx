"use client";

import axios from 'axios';
import React, { useState, useEffect, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import config from '../../../config';
import Link from 'next/link';
import { formatAmount, truncateHash, timeAgo, formatPlanckAdaptive, formatNumberWithCommas, hexToNumber } from '../../lib/helpers';
import Breadcrumbs from '../../components/Breadcrumbs';
import DetailRow from '../../components/DetailRow';
import CopyButton from '../../components/CopyButton';
import EmptyState from '../../components/EmptyState';
import Badge from '../../components/Badge';

// Same threshold the tx page uses for confirmations: past this depth a
// block is effectively final and we stop polling to bound load.
const TERMINAL_CONFIRMATIONS = 24;

function BackToBlocksLink(): JSX.Element | null {
  const searchParams = useSearchParams();
  const fromBlocks = searchParams.get('from') === 'blocks';
  const returnPage = searchParams.get('page') || '1';

  if (!fromBlocks) return null;

  return (
    <Link
      href={`/blocks/${returnPage}`}
      className="inline-flex items-center text-text-secondary hover:text-accent mb-4 md:mb-6"
    >
      <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
      </svg>
      Back to Blocks
    </Link>
  );
}

type Block = {
  baseFeePerGas: string;
  gasLimit: string;
  gasUsed: string;
  hash: string;
  number: string;
  parentHash: string;
  receiptsRoot: string;
  stateRoot: string;
  timestamp: string;
  transactions: Array<{
    hash: string;
    from: string;
    to: string;
    value: string;
  }>;
  transactionsRoot: string;
  extraData: string;
  logsBloom: string;
  miner: string;
  size: string;
  prevRandao: string;
  withdrawals: any[];
  withdrawalsRoot: string;
};

interface BlockDetailClientProps {
  blockNumber: string;
}

const formatTimestampUTC = (timestamp: string | null | undefined): string => {
  if (!timestamp) return 'N/A';
  const ts = typeof timestamp === 'string' && timestamp.startsWith('0x')
    ? parseInt(timestamp, 16)
    : parseInt(timestamp);
  if (isNaN(ts)) return 'N/A';
  const date = new Date(ts * 1000);
  const month = (date.getUTCMonth() + 1).toString().padStart(2, '0');
  const day = date.getUTCDate().toString().padStart(2, '0');
  const year = date.getUTCFullYear();
  const hours = date.getUTCHours().toString().padStart(2, '0');
  const minutes = date.getUTCMinutes().toString().padStart(2, '0');
  const seconds = date.getUTCSeconds().toString().padStart(2, '0');
  return `${month}/${day}/${year}, ${hours}:${minutes}:${seconds} UTC`;
};

export default function BlockDetailClient({ blockNumber }: BlockDetailClientProps): JSX.Element {
  // Reject obviously-bad inputs before hitting the API: negative numbers,
  // non-integers, NaN, and absurdly long hex strings would all just earn
  // a 400 from the backend. Catching them here keeps the URL bar honest
  // and skips the network round-trip + console noise.
  const isValidBlockId = ((): boolean => {
    if (!blockNumber) return false;
    if (blockNumber.startsWith('0x')) return /^0x[0-9a-fA-F]{1,16}$/.test(blockNumber);
    const n = parseInt(blockNumber, 10);
    return Number.isFinite(n) && n >= 0 && String(n) === blockNumber;
  })();

  const [blockData, setBlockData] = useState<Block | null>(null);
  // Per-tx activity counts surfaced alongside the block payload by the
  // backend; an empty/absent map means no row had token-transfer or
  // internal-call activity and the columns render dashes.
  const [txActivity, setTxActivity] = useState<Record<string, { tokenTransfers: number; internalCalls: number }>>({});
  // Initialise loading/notFound from the validity check so the invalid-id
  // path doesn't need to write state inside a useEffect (set-state-in-effect
  // rule). React's "adjusting state on prop change" pattern below resyncs
  // when the user navigates between blocks.
  const [loading, setLoading] = useState<boolean>(isValidBlockId);
  const [error, setError] = useState<string | null>(null);
  const [notFound, setNotFound] = useState<boolean>(!isValidBlockId);
  const [prevBlock, setPrevBlock] = useState<string>(blockNumber);
  if (prevBlock !== blockNumber) {
    setPrevBlock(blockNumber);
    setBlockData(null);
    setError(null);
    setLoading(isValidBlockId);
    setNotFound(!isValidBlockId);
  }

  const blockNum = parseInt(blockNumber);

  // Live confirmations (= currentLatest - thisBlock). Polled via the
  // shared /latestblock endpoint (5s server cache). Stops at
  // TERMINAL_CONFIRMATIONS to bound load on long-lived tabs.
  const latestBlockQuery = useQuery<{ blockNumber: number }>({
    queryKey: ['latestblock'],
    queryFn: async () => {
      const r = await axios.get<{ blockNumber: number }>(`${config.handlerUrl}/latestblock`);
      return r.data;
    },
    enabled: Number.isFinite(blockNum) && blockNum >= 0,
    refetchInterval: (q) => {
      const latest = q.state.data?.blockNumber ?? 0;
      const depth = latest > 0 ? Math.max(0, latest - blockNum) : 0;
      return depth >= 0 && depth < TERMINAL_CONFIRMATIONS ? 5000 : false;
    },
    refetchIntervalInBackground: false,
  });
  const liveConfirmations = (() => {
    const latest = latestBlockQuery.data?.blockNumber;
    if (!Number.isFinite(blockNum) || !latest) return null;
    return Math.max(0, latest - blockNum);
  })();

  useEffect(() => {
    if (!isValidBlockId) return;
    const fetchBlock = async (): Promise<void> => {
      try {
        setLoading(true);
        const response = await axios.get(`${config.handlerUrl}/block/${blockNumber}`);
        const block = response.data?.block?.result;
        if (!block) throw new Error('Invalid block data received');

        setBlockData({
          baseFeePerGas: block.baseFeePerGas || '0x0',
          gasLimit: block.gasLimit || '0x0',
          gasUsed: block.gasUsed || '0x0',
          hash: block.hash || '',
          number: block.number || '0x0',
          parentHash: block.parentHash || '',
          receiptsRoot: block.receiptsRoot || '',
          stateRoot: block.stateRoot || '',
          timestamp: block.timestamp || '0x0',
          transactions: block.transactions || [],
          transactionsRoot: block.transactionsRoot || '',
          extraData: block.extraData || '',
          logsBloom: block.logsBloom || '',
          miner: block.miner || '',
          size: block.size || '0x0',
          prevRandao: block.prevRandao || '',
          withdrawals: block.withdrawals || [],
          withdrawalsRoot: block.withdrawalsRoot || '',
        });
        // txActivity is keyed by parent tx hash, payload {tokenTransfers,
        // internalCalls}. Treat absent as empty so the consumer doesn't
        // need to null-check on every cell.
        const activity = response.data?.txActivity;
        setTxActivity(activity && typeof activity === 'object' ? activity : {});
        setError(null);
        setNotFound(false);
      } catch (err) {
        // A 404 just means the block hasn't been synced yet (or doesn't
        // exist). Treat that as an informational state, not a console
        // error, Next.js dev mode promotes console.error to a red
        // overlay which makes a benign "future block" feel like a bug.
        if (axios.isAxiosError(err) && (err.response?.status === 404 || err.response?.status === 400)) {
          // 404 → block hasn't been synced; 400 → unparseable block id
          // (we'd usually have caught it in `isValidBlockId` above, but
          // the backend's parser may reject some edge cases we don't).
          setNotFound(true);
          setError(null);
        } else {
          console.error('Error fetching block:', err);
          setError('Failed to load block details');
          setNotFound(false);
        }
      } finally {
        setLoading(false);
      }
    };

    if (blockNumber) fetchBlock();
  }, [blockNumber, isValidBlockId]);

  if (loading) {
    return (
      <div className="flex justify-center items-center min-h-[400px]">
        <div className="animate-spin rounded-full h-10 w-10 border-t-2 border-b-2 border-accent"></div>
      </div>
    );
  }

  if (notFound) {
    return (
      <div className="py-4 sm:py-6 lg:py-8">
        <div className="card p-6">
          <p className="font-semibold text-accent mb-1">Block #{Number.isFinite(blockNum) ? blockNum.toLocaleString() : blockNumber} not found</p>
          <p className="text-sm text-text-secondary">
            This block hasn&apos;t been produced (or synced) yet. Check back in a few seconds, or
            {' '}<Link href="/blocks/1" className="text-accent hover:underline">browse recent blocks</Link>.
          </p>
        </div>
      </div>
    );
  }

  if (error || !blockData) {
    return (
      <div className="py-4 sm:py-6 lg:py-8">
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-xl">
          <p className="font-bold">Error:</p>
          <p className="text-sm">{error || 'Block not found'}</p>
        </div>
      </div>
    );
  }

  const tsNum = typeof blockData.timestamp === 'string' && blockData.timestamp.startsWith('0x')
    ? parseInt(blockData.timestamp, 16)
    : parseInt(blockData.timestamp);

  return (
    <main className="py-4 sm:py-6 lg:py-8" aria-labelledby="block-heading">
      <Breadcrumbs items={[
        { label: 'Blocks', href: '/blocks/1' },
        { label: `Block #${blockNumber}` },
      ]} />

      <Suspense>
        <BackToBlocksLink />
      </Suspense>

      {/* Block Details Card */}
      <section aria-labelledby="block-heading" className="card overflow-hidden mb-6">
        {/* Header */}
        <div className="flex items-center justify-between p-4 sm:p-6 border-b border-border">
          <div className="flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5} aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
            </svg>
            <h1 id="block-heading" className="section-title">Block #{formatNumberWithCommas(hexToNumber(blockData.number).toString())}</h1>
            {/* Block 0 is the chain's genesis block: flag it so the all-zero
                parent hash below reads as expected rather than broken. */}
            {blockNum === 0 && (
              <Badge variant="brand">Genesis</Badge>
            )}
            {/* Live depth: surfaces how settled this block is. "Latest"
                when we're looking at the head, otherwise "N
                confirmations". Hidden until the first /latestblock
                response arrives so we don't flash a stale value. */}
            {liveConfirmations !== null && (
              <Badge variant={liveConfirmations === 0 ? 'info' : liveConfirmations >= TERMINAL_CONFIRMATIONS ? 'success' : 'neutral'}>
                {liveConfirmations === 0
                  ? 'Latest'
                  : `${liveConfirmations.toLocaleString()} confirmation${liveConfirmations === 1 ? '' : 's'}`}
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-2">
            {blockNum > 0 && (
              <Link
                href={`/block/${blockNum - 1}`}
                className="px-3 py-1.5 rounded-lg bg-surface border border-border text-text-secondary text-sm hover:border-accent transition-colors"
              >
                &larr;
              </Link>
            )}
            <Link
              href={`/block/${blockNum + 1}`}
              className="px-3 py-1.5 rounded-lg bg-surface border border-border text-text-secondary text-sm hover:border-accent transition-colors"
            >
              &rarr;
            </Link>
          </div>
        </div>

        {/* Content */}
        <div className="p-4 sm:p-6">
          <DetailRow label="Block Hash" mono>
            <div className="flex items-start gap-2">
              <span>{blockData.hash}</span>
              <CopyButton value={blockData.hash} label="Copy hash" size="sm" />
            </div>
          </DetailRow>
          <DetailRow label="Parent Hash" mono>
            {/* Genesis has no parent: same guard as the prev-block arrow
                above, an unlinked all-zero hash beats a link to block -1. */}
            {blockNum > 0 ? (
              <Link
                href={`/block/${blockNum - 1}`}
                className="text-text-secondary hover:text-accent transition-colors break-all"
              >
                {blockData.parentHash}
              </Link>
            ) : (
              <span className="break-all">{blockData.parentHash}</span>
            )}
          </DetailRow>
          <DetailRow label="Timestamp">
            {timeAgo(tsNum)}
            <span className="text-text-muted ml-2">({formatTimestampUTC(blockData.timestamp)})</span>
          </DetailRow>
          <DetailRow label="Transactions">
            {blockData.transactions?.length ?? 0}
          </DetailRow>
          <DetailRow label="Gas Used">{formatNumberWithCommas(hexToNumber(blockData.gasUsed).toString())}</DetailRow>
          <DetailRow label="Gas Limit">{formatNumberWithCommas(hexToNumber(blockData.gasLimit).toString())}</DetailRow>
          <DetailRow label="Base Fee">
            {(() => {
              const [val, unit] = formatPlanckAdaptive(blockData.baseFeePerGas);
              return <>{val} {unit}</>;
            })()}
          </DetailRow>
          {blockData.prevRandao && (
            <DetailRow label="Prev Randao" mono>{blockData.prevRandao}</DetailRow>
          )}
          <DetailRow label="State Root" mono>{blockData.stateRoot}</DetailRow>
          <DetailRow label="Receipts Root" mono>{blockData.receiptsRoot}</DetailRow>
          {blockData.extraData && blockData.extraData !== '0x' && (
            <DetailRow label="Extra Data" mono>{blockData.extraData}</DetailRow>
          )}
        </div>
      </section>

      {/* Transactions Table */}
      <section aria-labelledby="block-txs-heading" className="card overflow-hidden">
        <div className="px-4 sm:px-6 py-4 border-b border-border">
          <h2 id="block-txs-heading" className="text-[15px] font-display font-semibold text-text-primary">
            Transactions ({blockData.transactions?.length ?? 0})
          </h2>
        </div>

        {blockData.transactions && blockData.transactions.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border/50">
                  <th className="text-left px-4 sm:px-6 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider">Hash</th>
                  <th className="text-left px-4 sm:px-6 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden sm:table-cell">From</th>
                  <th className="text-left px-4 sm:px-6 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden sm:table-cell">To</th>
                  <th className="text-left px-4 sm:px-6 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider">Value</th>
                  <th className="text-left px-4 sm:px-6 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden md:table-cell">Activity</th>
                </tr>
              </thead>
              <tbody>
                {blockData.transactions.map((tx) => {
                  const [amount, unit] = formatAmount(tx.value);
                  const activity = txActivity[tx.hash];
                  return (
                    <tr
                      key={tx.hash}
                      className="border-b border-border/30 last:border-b-0 hover:bg-surface-3 transition-colors"
                    >
                      <td className="px-4 sm:px-6 py-3">
                        <Link
                          href={`/tx/${tx.hash}`}
                          className="text-accent hover:text-accent-hover hover:underline font-mono text-xs"
                          title={tx.hash}
                        >
                          {truncateHash(tx.hash, 10, 6)}
                        </Link>
                      </td>
                      <td className="px-4 sm:px-6 py-3 hidden sm:table-cell">
                        <Link
                          href={`/address/${tx.from}`}
                          className="text-text-secondary hover:text-accent font-mono text-xs transition-colors"
                          title={tx.from}
                        >
                          {truncateHash(tx.from, 8, 6)}
                        </Link>
                      </td>
                      <td className="px-4 sm:px-6 py-3 hidden sm:table-cell">
                        <Link
                          href={`/address/${tx.to}`}
                          className="text-text-secondary hover:text-accent font-mono text-xs transition-colors"
                          title={tx.to}
                        >
                          {truncateHash(tx.to, 8, 6)}
                        </Link>
                      </td>
                      <td className="px-4 sm:px-6 py-3 text-text-secondary tabular-nums whitespace-nowrap">
                        {amount}
                        <span className="text-text-muted text-xs ml-1">{unit}</span>
                      </td>
                      <td className="px-4 sm:px-6 py-3 hidden md:table-cell whitespace-nowrap">
                        {activity ? (
                          <span className="text-xs font-mono text-text-secondary flex items-center gap-2">
                            {activity.tokenTransfers > 0 && (
                              <span className="text-accent" title="Token / NFT transfers emitted">
                                {activity.tokenTransfers} token{activity.tokenTransfers === 1 ? '' : 's'}
                              </span>
                            )}
                            {activity.internalCalls > 0 && (
                              <span className="text-text-secondary" title="Internal contract calls">
                                {activity.internalCalls} call{activity.internalCalls === 1 ? '' : 's'}
                              </span>
                            )}
                          </span>
                        ) : (
                          <span className="text-text-muted text-xs">-</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState title="No transactions in this block" description="This block was produced without any transactions." />
        )}
      </section>
    </main>
  );
}
