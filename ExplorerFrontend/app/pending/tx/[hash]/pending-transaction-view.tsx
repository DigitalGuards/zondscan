'use client';

import React, { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import axios from 'axios';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import type { PendingTransaction } from '@/app/types';
import config from '../../../../config';
import { formatAmount, formatGasPrice, decodeTokenTransferInput, formatTokenAmount, hexToBigInt } from '../../../lib/helpers';
import Badge from '../../../components/Badge';
import Breadcrumbs from '../../../components/Breadcrumbs';
import DetailRow from '../../../components/DetailRow';
import CopyButton from '../../../components/CopyButton';

interface PendingTransactionViewProps {
  pendingTx: PendingTransaction;
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

export default function PendingTransactionView({ pendingTx }: PendingTransactionViewProps): JSX.Element {
  const [formattedValue, unit] = formatAmount(pendingTx.value);
  const formattedGasPrice = formatGasPrice(pendingTx.gasPrice);
  const router = useRouter();

  const decodedTransfer = useMemo(() => {
    return decodeTokenTransferInput(pendingTx.input);
  }, [pendingTx.input]);

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
    enabled: statusQuery.data?.status === 'pending',
  });

  // ── Mined → navigate, dropped → flip local state ───────────────────────
  const [isDropped, setIsDropped] = useState(false);
  useEffect(() => {
    if (statusQuery.data?.status === 'mined') {
      router.replace(`/tx/${pendingTx.hash}`);
    } else if (statusQuery.data?.status === 'dropped') {
      setIsDropped(true);
    }
  }, [statusQuery.data?.status, pendingTx.hash, router]);

  // ── Elapsed counter (driven by local 1s tick, independent of polls) ────
  // Seed from createdAt so SSR and the first client render produce identical
  // markup (elapsed = 0); hydrate to real wall-clock time in the effect.
  const [nowSec, setNowSec] = useState(pendingTx.createdAt ?? 0);
  useEffect(() => {
    setNowSec(Math.floor(Date.now() / 1000));
    if (statusQuery.data?.status !== 'pending') return;
    const id = setInterval(() => setNowSec(Math.floor(Date.now() / 1000)), 1000);
    return () => clearInterval(id);
  }, [statusQuery.data?.status]);
  const elapsedSec = Math.max(0, nowSec - (pendingTx.createdAt ?? nowSec));

  // ── ETA countdown — seeded from each poll, ticks down every second ─────
  const [etaRemaining, setEtaRemaining] = useState<number | null>(null);
  useEffect(() => {
    if (typeof etaQuery.data?.etaSec === 'number') {
      setEtaRemaining(Math.max(0, Math.round(etaQuery.data.etaSec)));
    }
  }, [etaQuery.data?.etaSec]);
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
          <p className="text-gray-300">
            This transaction is no longer in the mempool. It may have been dropped or replaced.
            Please check if a transaction with a higher gas price was submitted with the same nonce.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      <Breadcrumbs items={[
        { label: 'Pending', href: '/pending/1' },
        { label: `${pendingTx.hash.slice(0, 10)}...${pendingTx.hash.slice(-6)}` },
      ]} />

      {/* Main Details Card */}
      <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
        {/* Header */}
        <div className="flex items-center justify-between p-4 sm:p-6 border-b border-[#3d3d3d]">
          <div className="flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6 text-[#ffa729]">
              <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
            </svg>
            <h1 className="text-xl sm:text-2xl font-bold text-[#ffa729]">Pending Transaction</h1>
            {isTokenTransfer && <Badge variant="brand">Token Transfer</Badge>}
          </div>
          <Badge variant="warning" size="md" dot>Pending</Badge>
        </div>

        {/* Live status strip */}
        <div className="px-4 sm:px-6 py-3 border-b border-[#3d3d3d] bg-[#1a1a1a]/40">
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
          <p className="text-[11px] text-gray-500">
            Auto-refreshing every 5s — this page will navigate to the confirmed transaction once it&apos;s included in a block.
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
              className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
            >
              {pendingTx.from}
            </Link>
          </DetailRow>
          <DetailRow label="To" mono>
            {pendingTx.to ? (
              <Link
                href={`/address/${pendingTx.to}`}
                className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
              >
                {pendingTx.to}
              </Link>
            ) : (
              <span className="text-gray-400">Contract Creation</span>
            )}
            {isTokenTransfer && <span className="text-xs text-gray-500 ml-2">(Contract)</span>}
          </DetailRow>
          <DetailRow label="Value">
            <span className="font-semibold text-[#ffa729]">{formattedValue}</span>
            <span className="text-gray-500 ml-1">{unit}</span>
          </DetailRow>
          <DetailRow label="Gas Price">{formattedGasPrice} Shor</DetailRow>
          <DetailRow label="Gas Limit">{pendingTx.gas}</DetailRow>
          <DetailRow label="Nonce">{pendingTx.nonce}</DetailRow>
        </div>
      </div>

      {/* Token Transfer Section */}
      {isTokenTransfer && decodedTransfer && (
        <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <div className="flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-[#ffa729]">
                <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
              </svg>
              <h2 className="text-[15px] font-semibold text-[#ffa729]">Token Transfer (Pending)</h2>
              <Badge variant="brand">QRC-20</Badge>
            </div>
          </div>
          <div className="p-4 sm:p-6">
            <DetailRow label="Method">
              <span className="font-mono text-sm">
                {decodedTransfer.methodName === 'transferFrom'
                  ? `${decodedTransfer.methodName}(address, address, uint256)`
                  : `${decodedTransfer.methodName}(address, uint256)`}
              </span>
            </DetailRow>
            <DetailRow label="Amount">
              <span className="font-semibold">{formatTokenAmount(decodedTransfer.amount, 18)}</span>
              <span className="text-gray-500 ml-2 text-xs">(raw: {decodedTransfer.amount})</span>
            </DetailRow>
            <DetailRow label="Recipient" mono>
              <Link
                href={`/address/${decodedTransfer.to}`}
                className="text-[#ffa729] hover:text-[#ffb84d] transition-colors break-all"
              >
                {decodedTransfer.to}
              </Link>
            </DetailRow>
            <p className="text-xs text-gray-500 mt-3">
              Token name and symbol will be available once the transaction is confirmed.
            </p>
          </div>
        </div>
      )}

      {/* Input Data Section */}
      {pendingTx.input && pendingTx.input !== '0x' && (
        <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden">
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <h2 className="text-[15px] font-semibold text-[#ffa729]">
              Input Data
              {isTokenTransfer && <span className="text-xs text-gray-500 ml-2">(decoded above)</span>}
            </h2>
          </div>
          <div className="p-4 sm:p-6">
            <p className="font-mono text-gray-300 break-all text-xs leading-relaxed">{pendingTx.input}</p>
          </div>
        </div>
      )}
    </div>
  );
}
