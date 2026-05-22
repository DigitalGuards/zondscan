'use client';

import { useEffect, useState } from 'react';
import axios from 'axios';
import Link from 'next/link';
import Badge from '../../components/Badge';
import config from '../../../config';
import { formatTokenAmount } from '../../lib/helpers';

/**
 * Holdings panel for an address page: ERC-20 token balances and per-tokenID
 * NFT (ERC-721 / ERC-1155) holdings. The data has been collected by the
 * syncer for a while now (tokenBalances collection, joined with contractCode
 * + tokenMetadata via the backend's /address/:addr/tokens and /nfts
 * endpoints), it just wasn't surfaced on the address page UI.
 *
 * Fetches both endpoints in parallel on mount. Each section renders
 * independently; a 200 with an empty array shows the empty state for that
 * section but doesn't block the other. Network failures are silent: the
 * section stays empty rather than throwing, matching the rest of the
 * address page's resilience pattern.
 */

interface TokenBalanceRow {
  contractAddress: string;
  balance: string;
  name?: string;
  symbol?: string;
  decimals?: number;
  tokenStandard?: string;
  tokenID?: string;
}

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

interface HoldingsDisplayProps {
  address: string;
}

const NFT_STANDARDS = new Set(['ERC-721', 'ERC-1155']);

export default function HoldingsDisplay({ address }: HoldingsDisplayProps): JSX.Element | null {
  const [tokens, setTokens] = useState<TokenBalanceRow[]>([]);
  const [nfts, setNfts] = useState<NFTBalanceRow[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [tokensRes, nftsRes] = await Promise.allSettled([
          axios.get(`${config.handlerUrl}/address/${address}/tokens`),
          axios.get(`${config.handlerUrl}/address/${address}/nfts`),
        ]);
        if (cancelled) return;
        if (tokensRes.status === 'fulfilled') {
          const arr: TokenBalanceRow[] = Array.isArray(tokensRes.value.data?.tokens)
            ? tokensRes.value.data.tokens
            : [];
          // The /address/:addr/tokens endpoint returns ERC-20 balances and
          // also legacy un-classified rows from before the tokenStandard tag
          // existed. Filter to ERC-20 (or untagged) so per-tokenID NFT rows
          // don't appear twice (they'll show in the NFTs section).
          setTokens(arr.filter((t) => !t.tokenStandard || t.tokenStandard === 'ERC-20'));
        }
        if (nftsRes.status === 'fulfilled') {
          const arr: NFTBalanceRow[] = Array.isArray(nftsRes.value.data?.nfts)
            ? nftsRes.value.data.nfts
            : [];
          setNfts(arr.filter((n) => NFT_STANDARDS.has(n.tokenStandard)));
        }
      } finally {
        if (!cancelled) setLoaded(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [address]);

  if (!loaded) return null;
  if (tokens.length === 0 && nfts.length === 0) return null;

  return (
    <div className="space-y-4 md:space-y-6">
      <h2 className="text-base md:text-lg lg:text-xl font-semibold text-accent">Holdings</h2>

      {tokens.length > 0 && (
        <div className="rounded-xl border border-border overflow-hidden">
          <div className="px-4 py-3 border-b border-border bg-[#1a1a1a]/40 flex items-center gap-2">
            <span className="text-sm font-medium text-gray-200">Tokens</span>
            <Badge variant="brand">{tokens.length} QRC-20</Badge>
          </div>
          <ul className="divide-y divide-border">
            {tokens.map((t) => {
              const formatted = formatTokenAmount(t.balance, t.decimals ?? 18);
              const label = t.name || t.symbol || t.contractAddress;
              return (
                <li key={t.contractAddress} className="px-4 py-3 flex flex-wrap items-center justify-between gap-3">
                  <Link
                    href={`/address/${t.contractAddress}`}
                    className="text-sm text-accent hover:text-accent-hover transition-colors font-medium"
                  >
                    {label}
                    {t.symbol && t.symbol !== t.name && (
                      <span className="text-gray-500 ml-2 font-mono text-xs">({t.symbol})</span>
                    )}
                  </Link>
                  <span className="text-sm font-mono text-gray-200 break-all">
                    {formatted}
                    {t.symbol && <span className="text-gray-500 ml-2">{t.symbol}</span>}
                  </span>
                </li>
              );
            })}
          </ul>
        </div>
      )}

      {nfts.length > 0 && (
        <div className="rounded-xl border border-border overflow-hidden">
          <div className="px-4 py-3 border-b border-border bg-[#1a1a1a]/40 flex items-center gap-2">
            <span className="text-sm font-medium text-gray-200">NFTs</span>
            <Badge variant="warning">{nfts.length} collection{nfts.length === 1 ? '' : 's'}</Badge>
          </div>
          <ul className="divide-y divide-border">
            {nfts.map((n) => {
              const standardBadge = n.tokenStandard === 'ERC-1155' ? 'QRC-1155' : 'QRC-721';
              const tokenLabel = n.name || `#${n.tokenID}`;
              const collectionLabel = n.collectionName || n.collectionSymbol || n.contractAddress;
              const qty = (() => {
                try {
                  return BigInt(n.balance).toLocaleString('en-US');
                } catch {
                  return n.balance;
                }
              })();
              return (
                <li
                  key={`${n.contractAddress}-${n.tokenID}`}
                  className="px-4 py-3 flex flex-wrap items-center justify-between gap-3"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    {n.image ? (
                      // Off-chain metadata images live on arbitrary HTTP / IPFS
                      // gateways, so plain <img> avoids next/image's domain-list
                      // requirement. Lazy load + fixed dimensions keep layout
                      // stable while many rows hydrate.
                      // eslint-disable-next-line @next/next/no-img-element
                      <img
                        src={n.image}
                        alt=""
                        loading="lazy"
                        className="w-10 h-10 rounded-md object-cover bg-[#1a1a1a]"
                      />
                    ) : (
                      <div className="w-10 h-10 rounded-md bg-[#1a1a1a] border border-border flex items-center justify-center text-xs font-mono text-gray-500">
                        {(n.tokenID || '?').slice(0, 3)}
                      </div>
                    )}
                    <div className="min-w-0">
                      <Link
                        href={`/address/${n.contractAddress}`}
                        className="text-sm text-accent hover:text-accent-hover transition-colors font-medium break-all"
                      >
                        {collectionLabel}
                      </Link>
                      <div className="text-xs text-gray-400 font-mono break-all">
                        {tokenLabel}
                        {n.name && n.tokenID && ` (#${n.tokenID})`}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant="warning">{standardBadge}</Badge>
                    {n.tokenStandard === 'ERC-1155' && (
                      <span className="text-sm font-mono text-gray-200">x{qty}</span>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </div>
  );
}
