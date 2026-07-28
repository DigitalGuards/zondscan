'use client';

import React, { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import axios from 'axios';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import type { PendingTransaction, ContractMeta } from '@/app/types';
import config from '../../../../config';
import { formatAmount, formatGasPrice, decodeTokenTransferInput, decodeContractCall, formatTokenAmount, hexToBigInt, type DecodedTokenTransfer } from '../../../lib/helpers';
import Badge from '../../../components/Badge';
import Breadcrumbs from '../../../components/Breadcrumbs';
import DetailRow from '../../../components/DetailRow';
import CopyButton from '../../../components/CopyButton';

interface PendingTransactionViewProps {
  pendingTx: PendingTransaction;
  /**
   * Verified-contract metadata for the tx's target, forwarded by the
   * server-side fetch. When `verified` and `abi` are present the view
   * can decode the calldata via the same ABI-driven decoder used on
   * the confirmed tx page (parity with the iter 3 work).
   */
  targetContract?: ContractMeta;
}

type LiveStatus = 'pending' | 'mined' | 'dropped';

interface EtaResponse {
  etaSec: number;
  avgBlockTimeSec: number;
  avgGasUsedHex: string;
  pendingCount: number;
  gasAheadHex: string;
  medianGasPriceHex: string;
  yourGasPriceHex: string;
}

function formatElapsed(seconds: number): string {
  if (seconds < 0) seconds = 0;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${s.toString().padStart(2, '0')}`;
}

// QRC badge label + variant per decoded standard. Kept symmetric with
// tx page's qrcBadgeText so a tx looks consistent before vs after mining.
function badgeForDecoded(dt: DecodedTokenTransfer): { text: string; variant: 'brand' | 'warning' | 'info' } {
  if (dt.methodName === 'setApprovalForAll') {
    return { text: dt.standard === 'ERC-1155' ? 'QRC-1155 Approval' : 'NFT Approval', variant: 'info' };
  }
  if (dt.standard === 'ERC-721') return { text: 'QRC-721', variant: 'warning' };
  if (dt.standard === 'ERC-1155') return { text: 'QRC-1155', variant: 'warning' };
  return { text: 'QRC-20', variant: 'brand' };
}

function headerLabelFor(dt: DecodedTokenTransfer): string {
  if (dt.methodName === 'setApprovalForAll') return 'Token Approval (Pending)';
  if (dt.methodName === 'safeBatchTransferFrom') return 'Multi-Token Batch Transfer (Pending)';
  if (dt.standard === 'ERC-721') return 'NFT Transfer (Pending)';
  if (dt.standard === 'ERC-1155') return 'Multi-Token Transfer (Pending)';
  return 'Token Transfer (Pending)';
}

function methodSignatureFor(dt: DecodedTokenTransfer): string {
  switch (dt.methodName) {
    case 'transfer':              return 'transfer(address, uint256)';
    case 'transferFrom':          return 'transferFrom(address, address, uint256)';
    case 'safeTransferFrom':      return dt.standard === 'ERC-1155'
      ? 'safeTransferFrom(address, address, uint256, uint256, bytes)'
      : 'safeTransferFrom(address, address, uint256)';
    case 'safeBatchTransferFrom': return 'safeBatchTransferFrom(address, address, uint256[], uint256[], bytes)';
    case 'setApprovalForAll':     return 'setApprovalForAll(address, bool)';
  }
}

export default function PendingTransactionView({ pendingTx, targetContract }: PendingTransactionViewProps): JSX.Element {
  const [formattedValue, unit] = formatAmount(pendingTx.value);
  const formattedGasPrice = formatGasPrice(pendingTx.gasPrice);
  const router = useRouter();

  const decodedTransfer = useMemo(() => {
    return decodeTokenTransferInput(pendingTx.input);
  }, [pendingTx.input]);

  // ABI fallback: if the calldata didn't match a known token selector,
  // try the recipient's verified ABI. Same machinery as the confirmed
  // tx page's Input Data card.
  const decodedCall = useMemo(() => {
    if (decodedTransfer) return null;
    return decodeContractCall(pendingTx.input, targetContract?.abi);
  }, [decodedTransfer, pendingTx.input, targetContract?.abi]);

  const isTokenTransfer = decodedTransfer !== null;

  // ── Status poll ─────────────────────────────────────────────────────────
  // /pending-transaction returns 404 once the tx is tombstoned. On 404 we
  // probe /tx to distinguish "mined → redirect" from "dropped → notice".
  const statusQuery = useQuery<{ status: LiveStatus }>({
    queryKey: ['pending-tx-status', pendingTx.hash],
    queryFn: async () => {
      try {
        const res = await axios.get(`${config.handlerUrl}/pending-transaction/${pendingTx.hash}`);
        if (res.data?.transaction?.status === 'pending') {
          return { status: 'pending' };
        }
        return { status: 'mined' };
      } catch (err: unknown) {
        if (axios.isAxiosError(err) && err.response?.status === 404) {
          try {
            const tx = await axios.get(`${config.handlerUrl}/tx/${pendingTx.hash}`);
            if (tx.data?.response) return { status: 'mined' };
          } catch {
            /* fallthrough */
          }
          return { status: 'dropped' };
        }
        return { status: 'pending' };
      }
    },
    initialData: { status: 'pending' },
    refetchInterval: (query) => (query.state.data?.status === 'pending' ? 5000 : false),
  });

  // ── ETA poll ────────────────────────────────────────────────────────────
  const etaQuery = useQuery<EtaResponse | null>({
    queryKey: ['pending-tx-eta', pendingTx.hash],
    queryFn: async () => {
      try {
        const res = await axios.get<EtaResponse>(`${config.handlerUrl}/pending-tx-eta/${pendingTx.hash}`);
        return res.data;
      } catch {
        return null;
      }
    },
    refetchInterval: 5000,
    refetchIntervalInBackground: false,
    enabled: statusQuery.data?.status === 'pending',
  });

  // ── Mined → navigate, dropped → flip local state ───────────────────────
  // Derive isDropped from the polled status directly. router.replace is
  // a side effect that *must* stay in a useEffect (no setState involved
  // there, so the rule doesn't fire).
  const isDropped = statusQuery.data?.status === 'dropped';
  useEffect(() => {
    if (statusQuery.data?.status === 'mined') {
      router.replace(`/tx/${pendingTx.hash}`);
    }
  }, [statusQuery.data?.status, pendingTx.hash, router]);

  // ── Elapsed counter (driven by local 1s tick, independent of polls) ────
  // Seed from createdAt so SSR and the first client render produce identical
  // markup (elapsed = 0); hydrate to real wall-clock time after mount via
  // a deferred setState (outside the synchronous effect body) so we don't
  // trip set-state-in-effect.
  const [nowSec, setNowSec] = useState(pendingTx.createdAt ?? 0);
  useEffect(() => {
    const seed = setTimeout(() => setNowSec(Math.floor(Date.now() / 1000)), 0);
    if (statusQuery.data?.status !== 'pending') {
      return () => clearTimeout(seed);
    }
    const id = setInterval(() => setNowSec(Math.floor(Date.now() / 1000)), 1000);
    return () => {
      clearTimeout(seed);
      clearInterval(id);
    };
  }, [statusQuery.data?.status]);
  const elapsedSec = Math.max(0, nowSec - (pendingTx.createdAt ?? nowSec));

  // ── ETA countdown, seeded from each poll, ticks down every second ─────
  // Adjusting-state-on-prop-change: detect a new poll value mid-render and
  // resync without writing state inside an effect (set-state-in-effect).
  const [etaRemaining, setEtaRemaining] = useState<number | null>(null);
  const [prevEtaSec, setPrevEtaSec] = useState<number | undefined>(undefined);
  if (etaQuery.data?.etaSec !== prevEtaSec) {
    setPrevEtaSec(etaQuery.data?.etaSec);
    if (typeof etaQuery.data?.etaSec === 'number') {
      setEtaRemaining(Math.max(0, Math.round(etaQuery.data.etaSec)));
    }
  }
  const hasEta = etaRemaining !== null;
  useEffect(() => {
    if (!hasEta) return;
    const id = setInterval(() => setEtaRemaining((v) => (v === null ? null : Math.max(0, v - 1))), 1000);
    return () => clearInterval(id);
  }, [hasEta]);

  // ── Gas-vs-median comparison ───────────────────────────────────────────
  const gasCompare = useMemo(() => {
    if (!etaQuery.data?.medianGasPriceHex) return null;
    const yours = hexToBigInt(pendingTx.gasPrice);
    const median = hexToBigInt(etaQuery.data.medianGasPriceHex);
    if (median === BigInt(0)) return null;
    if (yours > median) return { variant: 'success' as const, label: 'above median' };
    if (yours < median) return { variant: 'warning' as const, label: 'below median' };
    return { variant: 'info' as const, label: 'at median' };
  }, [etaQuery.data?.medianGasPriceHex, pendingTx.gasPrice]);

  if (isDropped) {
    return (
      <div className="container mx-auto px-4">
        <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
          <h2 className="text-red-500 font-semibold mb-2">Transaction Not Found</h2>
          <p className="text-text-secondary">
            This transaction is no longer in the mempool. It may have been dropped or replaced.
            Please check if a transaction with a higher gas price was submitted with the same nonce.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="py-4 sm:py-6 lg:py-8">
      <Breadcrumbs items={[
        { label: 'Pending', href: '/pending/1' },
        { label: `${pendingTx.hash.slice(0, 10)}...${pendingTx.hash.slice(-6)}` },
      ]} />

      {/* Main Details Card */}
      <div className="card overflow-hidden mb-6">
        {/* Header */}
        <div className="flex items-center justify-between p-4 sm:p-6 border-b border-border">
          <div className="flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6 text-accent">
              <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
            </svg>
            <h1 className="section-title">Pending Transaction</h1>
            {decodedTransfer && (() => {
              const b = badgeForDecoded(decodedTransfer);
              return <Badge variant={b.variant}>{b.text}</Badge>;
            })()}
            {!decodedTransfer && decodedCall && (
              <Badge variant="info">{decodedCall.name}</Badge>
            )}
          </div>
          <Badge variant="warning" size="md" dot>Pending</Badge>
        </div>

        {/* Live status strip */}
        <div className="px-4 sm:px-6 py-3 border-b border-border bg-background/40">
          <div className="flex flex-wrap items-center gap-2 mb-1">
            <Badge variant="warning">Pending for {formatElapsed(elapsedSec)}</Badge>
            {etaRemaining !== null && (
              <Badge variant="info">
                {etaRemaining > 0 ? `Next inclusion ~${formatElapsed(etaRemaining)}` : 'Any moment now…'}
              </Badge>
            )}
            {typeof etaQuery.data?.pendingCount === 'number' && (
              <Badge variant="neutral">Mempool: {etaQuery.data.pendingCount}</Badge>
            )}
            {gasCompare && (
              <Badge variant={gasCompare.variant}>Gas: {gasCompare.label}</Badge>
            )}
          </div>
          <p className="text-[11px] text-text-muted">
            Auto-refreshing every 5s, this page will navigate to the confirmed transaction once it&apos;s included in a block.
          </p>
        </div>

        {/* Content */}
        <div className="p-4 sm:p-6">
          <DetailRow label="Transaction Hash" mono>
            <div className="flex items-start gap-2">
              <span>{pendingTx.hash}</span>
              <CopyButton value={pendingTx.hash} label="Copy hash" size="sm" />
            </div>
          </DetailRow>
          <DetailRow label="Status">
            <Badge variant="warning" dot>pending</Badge>
          </DetailRow>
          <DetailRow label="From" mono>
            <Link
              href={`/address/${pendingTx.from}`}
              className="text-text-primary hover:text-accent transition-colors break-all"
            >
              {pendingTx.from}
            </Link>
          </DetailRow>
          <DetailRow label="To" mono>
            {pendingTx.to ? (
              <Link
                href={`/address/${pendingTx.to}`}
                className="text-text-primary hover:text-accent transition-colors break-all"
              >
                {pendingTx.to}
              </Link>
            ) : (
              <span className="text-text-secondary">Contract Creation</span>
            )}
            {isTokenTransfer && <span className="text-xs text-text-muted ml-2">(Contract)</span>}
          </DetailRow>
          <DetailRow label="Value">
            <span className="font-semibold text-accent">{formattedValue}</span>
            <span className="text-text-muted ml-1">{unit}</span>
          </DetailRow>
          <DetailRow label="Gas Price">{formattedGasPrice} Shor</DetailRow>
          <DetailRow label="Gas Limit">{pendingTx.gas}</DetailRow>
          <DetailRow label="Nonce">{pendingTx.nonce}</DetailRow>
        </div>
      </div>

      {/* Token call section. Layout branches on the decoded shape: ERC-20
          shows recipient + raw amount (decimals unknown until confirmed);
          ERC-721 / ERC-1155 surface tokenID(s) + value(s); setApprovalForAll
          shows operator + on/off. Token name/symbol fill in once mined. */}
      {decodedTransfer && (() => {
        const b = badgeForDecoded(decodedTransfer);
        const isApproval = decodedTransfer.methodName === 'setApprovalForAll';
        const isBatch = decodedTransfer.methodName === 'safeBatchTransferFrom';
        return (
          <div className="card overflow-hidden mb-6">
            <div className="px-4 sm:px-6 py-4 border-b border-border">
              <div className="flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-accent">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
                </svg>
                <h2 className="text-[15px] font-display font-semibold text-text-primary">{headerLabelFor(decodedTransfer)}</h2>
                <Badge variant={b.variant}>{b.text}</Badge>
              </div>
            </div>
            <div className="p-4 sm:p-6">
              <DetailRow label="Method">
                <span className="font-mono text-sm">{methodSignatureFor(decodedTransfer)}</span>
              </DetailRow>

              {/* setApprovalForAll(operator, approved) */}
              {isApproval && (
                <>
                  <DetailRow label="Operator" mono>
                    <Link
                      href={`/address/${decodedTransfer.operator}`}
                      className="text-accent hover:text-accent-hover transition-colors break-all"
                    >
                      {decodedTransfer.operator}
                    </Link>
                  </DetailRow>
                  <DetailRow label="Approved">
                    <Badge variant={decodedTransfer.approved ? 'success' : 'error'}>
                      {decodedTransfer.approved ? 'true (granted)' : 'false (revoked)'}
                    </Badge>
                  </DetailRow>
                </>
              )}

              {/* Transfer-like calls: From (when present) → To */}
              {!isApproval && decodedTransfer.from && (
                <DetailRow label="From" mono>
                  <Link
                    href={`/address/${decodedTransfer.from}`}
                    className="text-text-primary hover:text-accent transition-colors break-all"
                  >
                    {decodedTransfer.from}
                  </Link>
                </DetailRow>
              )}
              {!isApproval && decodedTransfer.to && (
                <DetailRow label={decodedTransfer.from ? 'To' : 'Recipient'} mono>
                  <Link
                    href={`/address/${decodedTransfer.to}`}
                    className="text-accent hover:text-accent-hover transition-colors break-all"
                  >
                    {decodedTransfer.to}
                  </Link>
                </DetailRow>
              )}

              {/* ERC-20: raw amount (decimals unknown pre-confirmation) */}
              {decodedTransfer.standard === 'ERC-20' && decodedTransfer.amount && (
                <DetailRow label="Amount">
                  <span className="font-semibold">{formatTokenAmount(decodedTransfer.amount, 18)}</span>
                  <span className="text-text-muted ml-2 text-xs">(raw: {decodedTransfer.amount}, assumes 18 decimals)</span>
                </DetailRow>
              )}

              {/* ERC-721 / ERC-1155 single */}
              {!isBatch && decodedTransfer.tokenID && (
                <DetailRow label="Token ID" mono>
                  <span className="font-semibold text-text-primary">#{decodedTransfer.tokenID}</span>
                </DetailRow>
              )}
              {decodedTransfer.standard === 'ERC-1155' && !isBatch && decodedTransfer.value && (
                <DetailRow label="Quantity">
                  <span className="font-semibold text-text-primary">
                    {(() => { try { return BigInt(decodedTransfer.value).toLocaleString('en-US'); } catch { return decodedTransfer.value; } })()}
                  </span>
                </DetailRow>
              )}

              {/* ERC-1155 batch: list each (id, qty) row */}
              {isBatch && decodedTransfer.ids && decodedTransfer.values && (
                <DetailRow label="Batch">
                  <div className="flex flex-col gap-1 mt-1">
                    {decodedTransfer.ids.map((id, i) => (
                      <div key={`${id}-${i}`} className="font-mono text-xs text-text-primary">
                        <span className="text-accent">#{id}</span>
                        <span className="text-text-muted"> x </span>
                        <span>{(() => { try { return BigInt(decodedTransfer.values![i]).toLocaleString('en-US'); } catch { return decodedTransfer.values![i]; } })()}</span>
                      </div>
                    ))}
                  </div>
                </DetailRow>
              )}

              <p className="text-xs text-text-muted mt-3">
                {isApproval
                  ? 'Approval will take effect once the transaction is confirmed.'
                  : 'Token name and symbol will be available once the transaction is confirmed.'}
              </p>
            </div>
          </div>
        );
      })()}

      {/* ABI-decoded contract call (verified target, falls through from
          the well-known-token-selector path above). Same shape as the
          confirmed tx page's Input Data decoder section. */}
      {!decodedTransfer && decodedCall && (
        <div className="card overflow-hidden mb-6">
          <div className="px-4 sm:px-6 py-4 border-b border-border">
            <div className="flex items-center gap-2">
              <h2 className="text-[15px] font-display font-semibold text-text-primary">Contract Call (Pending)</h2>
              <Badge variant="info">{decodedCall.name}</Badge>
              {targetContract?.contractName && (
                <span className="text-xs text-text-muted font-mono">via {targetContract.contractName}</span>
              )}
            </div>
          </div>
          <div className="p-4 sm:p-6">
            <p className="text-xs text-text-muted font-mono mb-2">{decodedCall.signature}</p>
            <div className="space-y-1">
              {decodedCall.args.map((arg) => (
                <div key={arg.label} className="text-xs flex flex-wrap items-start gap-2">
                  <span className="text-text-secondary font-mono min-w-[80px]">{arg.label}:</span>
                  {arg.type === 'address' && arg.value ? (
                    <Link href={`/address/${arg.value}`} className="text-text-primary hover:text-accent transition-colors break-all font-mono">
                      {arg.value}
                    </Link>
                  ) : arg.type === 'bool' ? (
                    <Badge variant={arg.value === 'true' ? 'success' : 'error'}>{arg.value}</Badge>
                  ) : arg.type === 'uint256' ? (
                    <span className="text-text-primary font-mono break-all">
                      {(() => { try { return BigInt(arg.value || '0').toLocaleString('en-US'); } catch { return arg.value || ''; } })()}
                    </span>
                  ) : arg.type === 'uint256[]' ? (
                    <span className="text-text-primary font-mono break-all">
                      {arg.values && arg.values.length > 0
                        ? arg.values.map((v) => { try { return BigInt(v).toLocaleString('en-US'); } catch { return v; } }).join(', ')
                        : '[]'}
                    </span>
                  ) : arg.type === 'string' ? (
                    <span className="text-text-primary break-all">{arg.value}</span>
                  ) : (
                    <span className="text-text-primary font-mono break-all">{arg.value}</span>
                  )}
                </div>
              ))}
            </div>
            <p className="text-xs text-text-muted mt-3">
              Decoded from the target contract&apos;s verified ABI. Final on-chain state lands once the transaction is confirmed.
            </p>
          </div>
        </div>
      )}

      {/* Input Data Section */}
      {pendingTx.input && pendingTx.input !== '0x' && (
        <div className="card overflow-hidden">
          <div className="px-4 sm:px-6 py-4 border-b border-border">
            <h2 className="text-[15px] font-display font-semibold text-text-primary">
              Input Data
              {(isTokenTransfer || decodedCall) && <span className="text-xs text-text-muted ml-2">(decoded above)</span>}
            </h2>
          </div>
          <div className="p-4 sm:p-6">
            <p className="font-mono text-text-secondary break-all text-xs leading-relaxed">{pendingTx.input}</p>
          </div>
        </div>
      )}
    </div>
  );
}
