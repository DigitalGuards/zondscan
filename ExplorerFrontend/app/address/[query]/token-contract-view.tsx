'use client';

import { useState, useEffect } from 'react';
import Image from 'next/image';
import ImageWithFallback from '../../components/ImageWithFallback';
import Link from 'next/link';
import CopyButton from "../../components/CopyButton";
import QRCodeButton from "../../components/QRCodeButton";
import ContractTabs from "../../components/ContractTabs";
import TabPillBar from "../../components/TabPillBar";
import VerifiedBadge from "../../components/VerifiedBadge";
import type { ContractData } from "../../types/address";
import { compactTokenIDLabel, formatAmount } from "../../lib/helpers";
import Breadcrumbs from "../../components/Breadcrumbs";

interface TokenInfo {
    contractAddress: string;
    name: string;
    symbol: string;
    decimals: number;
    totalSupply: string;
    holderCount: number;
    transferCount: number;
    creatorAddress: string;
    creationTxHash: string;
    creationBlock: string;
}

interface TokenHolder {
    contractAddress: string;
    holderAddress: string;
    balance: string;
    blockNumber: string;
    updatedAt: string;
    // Phase 2: per-id storage for NFT collections. Always absent for ERC-20.
    tokenID?: string;
    tokenStandard?: 'ERC-20' | 'ERC-721' | 'ERC-1155' | string;
}

interface TokenIDSummary {
    tokenID: string;
    holderCount: number;
    tokenStandard?: string;
    blockNumber?: string;
    updatedAt?: string;
    // Phase 3b: off-chain per-token metadata pulled from contractURI /
    // tokenURI / uri(id). Absent on tokens whose metadata fetcher pass
    // hasn't completed or whose contract doesn't implement the getter.
    name?: string;
    description?: string;
    image?: string;
}

interface TokenTransfer {
    contractAddress: string;
    from: string;
    to: string;
    amount: string;
    blockNumber: string;
    txHash: string;
    timestamp: string;
    tokenSymbol: string;
    tokenDecimals: number;
    tokenName: string;
    transferType: string;
}

interface CreationTxData {
    BlockNumber: string;
    BlockTimestamp: string;
    From: string;
    TxHash: string;
    GasUsed: string;
    GasPrice: string;
    Value: string;
}

interface TokenContractViewProps {
    address: string;
    // Accept the shared ContractData shape so verification fields flow
    // through to the Code/Read/Write tabs alongside the existing
    // creator/symbol/decimals metadata.
    contractData: Partial<ContractData> & {
        creatorAddress?: string;
        creationTransaction?: string;
        contractCode?: string;
        isToken?: boolean;
        name?: string;
        symbol?: string;
        decimals?: number;
        totalSupply?: string;
        status?: string;
    };
    handlerUrl: string;
}

const AddressDisplay = ({ address, truncate = false }: { address: string; truncate?: boolean }) => {
    if (!address) return <span className="text-text-muted">Unknown</span>;

    const display = truncate ? `${address.slice(0, 10)}...${address.slice(-8)}` : address;
    return (
        <Link href={`/address/${address}`} className={`text-accent hover:text-accent-hover font-mono text-xs md:text-sm${truncate ? '' : ' break-all'}`}>
            {display}
        </Link>
    );
};

const formatTokenAmount = (amount: string, decimals: number): string => {
    if (!amount) return '0';

    // Handle hex amounts
    let value = amount;
    if (amount.startsWith('0x')) {
        try {
            value = BigInt(amount).toString();
        } catch {
            return '0';
        }
    }

    // Format with decimals
    const len = value.length;
    if (len <= decimals) {
        const zeros = '0'.repeat(decimals - len);
        return `0.${zeros}${value}`.replace(/\.?0+$/, '') || '0';
    }

    const intPart = value.slice(0, len - decimals);
    const decPart = value.slice(len - decimals);
    const formatted = decPart ? `${intPart}.${decPart}`.replace(/\.?0+$/, '') : intPart;

    // Add thousand separators
    const parts = formatted.split('.');
    parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    return parts.join('.');
};

// Only render contract-supplied metadata URLs that use an http(s) scheme.
// metadataExternalURL is attacker-controlled (it comes from the on-chain
// contractURI JSON), so blocking javascript:/data: and other schemes here
// prevents the anchor from becoming an XSS vector.
const isHttpUrl = (u?: string): boolean => !!u && /^https?:\/\//i.test(u);

const formatTimestamp = (timestamp: string): string => {
    if (!timestamp) return 'Unknown';

    let ts = timestamp;
    if (timestamp.startsWith('0x')) {
        ts = parseInt(timestamp, 16).toString();
    }

    const date = new Date(parseInt(ts) * 1000);
    return date.toUTCString();
};

export default function TokenContractView({ address, contractData, handlerUrl }: TokenContractViewProps) {
    const [activeTab, setActiveTab] = useState<'overview' | 'holders' | 'transfers' | 'tokens'>('overview');
    const [tokenInfo, setTokenInfo] = useState<TokenInfo | null>(null);
    const [holders, setHolders] = useState<TokenHolder[]>([]);
    const [transfers, setTransfers] = useState<TokenTransfer[]>([]);
    const [holdersTotal, setHoldersTotal] = useState(0);
    const [transfersTotal, setTransfersTotal] = useState(0);
    const [holdersPage, setHoldersPage] = useState(0);
    const [transfersPage, setTransfersPage] = useState(0);
    const [loading, setLoading] = useState(true);
    const [creationTx, setCreationTx] = useState<CreationTxData | null>(null);
    // Phase 2: NFT-specific state. holderTokenIDFilter narrows /holders to
    // a single tokenID; tokensList drives the new "Tokens" tab listing.
    const [holderTokenIDFilter, setHolderTokenIDFilter] = useState<string>('');
    const [holderTokenIDInput, setHolderTokenIDInput] = useState<string>('');
    const [tokensList, setTokensList] = useState<TokenIDSummary[]>([]);
    const [tokensTotal, setTokensTotal] = useState(0);
    const [tokensPage, setTokensPage] = useState(0);
    const limit = 25;

    // Fetch token info
    useEffect(() => {
        const fetchTokenInfo = async () => {
            try {
                const res = await fetch(`${handlerUrl}/token/${address}/info`);
                if (res.ok) {
                    const data = await res.json();
                    setTokenInfo(data);
                }
            } catch (error) {
                console.error('Failed to fetch token info:', error);
            }
        };
        fetchTokenInfo();
    }, [address, handlerUrl]);

    // Fetch creation transaction details
    useEffect(() => {
        const creationTxHash = tokenInfo?.creationTxHash || contractData.creationTransaction;
        if (!creationTxHash) return;

        const fetchCreationTx = async (): Promise<void> => {
            try {
                const res = await fetch(`${handlerUrl}/tx/${creationTxHash}`);
                if (res.ok) {
                    const data = await res.json();
                    if (data.response) {
                        setCreationTx(data.response);
                    }
                }
            } catch (error) {
                console.error('Failed to fetch creation tx:', error);
            }
        };
        fetchCreationTx();
    }, [tokenInfo?.creationTxHash, contractData.creationTransaction, handlerUrl]);

    // Fetch holders when tab is active. Phase 2: append ?tokenID= when the
    // filter is set so the backend returns rows for that specific id only.
    useEffect(() => {
        if (activeTab !== 'holders') return;

        const fetchHolders = async () => {
            setLoading(true);
            try {
                const qs = new URLSearchParams({
                    page: String(holdersPage),
                    limit: String(limit),
                });
                if (holderTokenIDFilter) qs.set('tokenID', holderTokenIDFilter);
                const res = await fetch(`${handlerUrl}/token/${address}/holders?${qs.toString()}`);
                if (res.ok) {
                    const data = await res.json();
                    setHolders(data.holders || []);
                    setHoldersTotal(data.totalHolders || 0);
                }
            } catch (error) {
                console.error('Failed to fetch holders:', error);
            }
            setLoading(false);
        };
        fetchHolders();
    }, [address, handlerUrl, activeTab, holdersPage, holderTokenIDFilter]);

    // Fetch tokens (distinct tokenID list) when the tokens tab is active.
    useEffect(() => {
        if (activeTab !== 'tokens') return;

        const fetchTokens = async () => {
            setLoading(true);
            try {
                const res = await fetch(`${handlerUrl}/token/${address}/tokens?page=${tokensPage}&limit=${limit}`);
                if (res.ok) {
                    const data = await res.json();
                    setTokensList(data.tokens || []);
                    setTokensTotal(data.totalTokenIDs || 0);
                }
            } catch (error) {
                console.error('Failed to fetch tokens:', error);
            }
            setLoading(false);
        };
        fetchTokens();
    }, [address, handlerUrl, activeTab, tokensPage]);

    // Fetch transfers when tab is active
    useEffect(() => {
        if (activeTab !== 'transfers') return;

        const fetchTransfers = async () => {
            setLoading(true);
            try {
                const res = await fetch(`${handlerUrl}/token/${address}/transfers?page=${transfersPage}&limit=${limit}`);
                if (res.ok) {
                    const data = await res.json();
                    setTransfers(data.transfers || []);
                    setTransfersTotal(data.totalTransfers || 0);
                }
            } catch (error) {
                console.error('Failed to fetch transfers:', error);
            }
            setLoading(false);
        };
        fetchTransfers();
    }, [address, handlerUrl, activeTab, transfersPage]);

    const decimals = tokenInfo?.decimals ?? contractData.decimals ?? 18;
    const rawSymbol = tokenInfo?.symbol ?? contractData.symbol ?? '';
    const rawName = tokenInfo?.name ?? contractData.name ?? '';
    // Phase 3a: prefer the off-chain metadata-name over the on-chain
    // name() because most NFT collections leave name() empty and put the
    // human-readable title in the contractURI JSON instead. The on-chain
    // symbol still wins for the badge (collection JSONs don't carry one).
    const metaName = contractData.metadataName?.trim() || '';
    const metaImage = contractData.metadataImage?.trim() || '';
    const metaDescription = contractData.metadataDescription?.trim() || '';
    const metaExternalURL = contractData.metadataExternalURL?.trim() || '';
    // ERC-1155 collections often omit name()/symbol(), so fall back to a
    // truncated address rather than rendering "Unknown Token" / "TOKEN".
    const addrShort = `${address.slice(0, 10)}...${address.slice(-6)}`;
    const symbol = rawSymbol || addrShort;
    const name = metaName || rawName || addrShort;
    const totalSupply = tokenInfo?.totalSupply ?? contractData.totalSupply ?? '0';
    const creatorAddress = creationTx?.From || tokenInfo?.creatorAddress || contractData.creatorAddress || '';
    const creationTxHash = tokenInfo?.creationTxHash || contractData.creationTransaction || '';
    const tokenStandard = contractData.tokenStandard;
    const isNFT = tokenStandard === 'ERC-721' || tokenStandard === 'ERC-1155';
    const badgeLabel =
        tokenStandard === 'ERC-721'
            ? 'QRC-721 NFT'
            : tokenStandard === 'ERC-1155'
              ? 'QRC-1155 Multi-Token'
              : 'QRC-20 Token';
    const badgeClasses = isNFT
        ? 'bg-purple-500/20 text-purple-300'
        : 'bg-success/20 text-success';

    const tabs: { id: typeof activeTab; label: string }[] = [
        { id: 'overview', label: 'Overview' },
        { id: 'holders', label: `Holders${tokenInfo ? ` (${tokenInfo.holderCount})` : ''}` },
        { id: 'transfers', label: `Transfers${tokenInfo ? ` (${tokenInfo.transferCount})` : ''}` },
    ];
    // Phase 2: only NFT collections have a meaningful per-id list.
    if (isNFT) {
        tabs.push({ id: 'tokens', label: 'Tokens' });
    }

    // Three-step breadcrumb: Home > Contracts > <standard tab> > <this contract>.
    // The middle step deep-links back to the /contracts page with the
    // correct tab pre-selected so a back-step lands the user where they
    // came from.
    const tabHref =
        tokenStandard === 'ERC-721'  ? '/contracts?tab=erc721'
      : tokenStandard === 'ERC-1155' ? '/contracts?tab=erc1155'
      : tokenStandard === 'ERC-20'   ? '/contracts?tab=erc20'
      :                                '/contracts';
    const tabLabel =
        tokenStandard === 'ERC-721'  ? 'NFTs'
      : tokenStandard === 'ERC-1155' ? 'Multi-Token'
      : tokenStandard === 'ERC-20'   ? 'Tokens'
      :                                'All';

    return (
        <div className="detail-content">
            <Breadcrumbs items={[
                { label: 'Contracts', href: '/contracts' },
                { label: tabLabel, href: tabHref },
                { label: `${symbol || address.slice(0, 10) + '...' + address.slice(-6)}` },
            ]} />
            {/* Token Header Card */}
            <div className="relative overflow-hidden rounded-xl md:card mb-4 md:mb-6">
                <div className="p-4 md:p-6 lg:p-8">
                    {/* Token Identity */}
                    <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6 pb-4 border-b border-border">
                        <div className="flex items-center gap-4">
                            {/* Token Icon. Phase 3a: render the off-chain
                                metadata image when present, fall back to the
                                first-character monogram. Fixed 64px square so
                                the layout doesn't shift while the next/image
                                loader resolves. */}
                            {metaImage ? (
                                <div className="relative w-12 h-12 md:w-16 md:h-16 rounded-full overflow-hidden border border-border bg-black/30">
                                    <ImageWithFallback
                                        src={metaImage}
                                        alt={name}
                                        fill
                                        sizes="64px"
                                        className="object-cover"
                                        fallback={
                                            <div className="absolute inset-0 bg-gradient-to-br from-accent to-accent-dark flex items-center justify-center text-xl md:text-2xl font-bold text-background">
                                                {symbol.charAt(0)}
                                            </div>
                                        }
                                    />
                                </div>
                            ) : (
                                <div className="w-12 h-12 md:w-16 md:h-16 rounded-full bg-gradient-to-br from-accent to-accent-dark flex items-center justify-center text-xl md:text-2xl font-bold text-background">
                                    {symbol.charAt(0)}
                                </div>
                            )}
                            <div className="min-w-0">
                                <div className="flex items-center gap-2 flex-wrap">
                                    <h1 className="text-xl md:text-2xl font-bold text-text-primary">{name}</h1>
                                    {rawSymbol && (
                                        <span className="px-2 py-0.5 rounded bg-accent/20 text-accent text-sm font-medium">
                                            {rawSymbol}
                                        </span>
                                    )}
                                    {isHttpUrl(metaExternalURL) && (
                                        <a
                                            href={metaExternalURL}
                                            target="_blank"
                                            rel="noreferrer noopener nofollow"
                                            className="text-xs text-text-secondary hover:text-accent underline"
                                            title="External URL from contract metadata"
                                        >
                                            site ↗
                                        </a>
                                    )}
                                </div>
                                {metaDescription && (
                                    <p className="text-xs md:text-sm text-text-secondary mt-1 line-clamp-2 max-w-prose">
                                        {metaDescription}
                                    </p>
                                )}
                                <div className="flex items-center gap-2 mt-1 min-w-0">
                                    <span className="text-xs md:text-sm text-text-secondary font-mono break-all">{address}</span>
                                    <CopyButton value={address} label="Copy address" />
                                    <QRCodeButton address={address} />
                                </div>
                            </div>
                        </div>
                        <div className={`px-3 py-1.5 rounded-lg text-sm font-medium self-start ${badgeClasses}`}>
                            {badgeLabel}
                        </div>
                    </div>

                    {/* Token Stats Grid. Decimals hidden for NFT collections (no
                        fractional units); for ERC-1155 totalSupply is the
                        operator's reported aggregate and may not be meaningful
                        per-id (Phase 2 will surface per-id supply on the holders
                        endpoint). */}
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                        <div className="bg-black/20 rounded-lg p-3 md:p-4">
                            <div className="text-xs md:text-sm text-text-secondary mb-1">
                                {tokenStandard === 'ERC-721' ? 'Total Items' : 'Total Supply'}
                            </div>
                            <div
                                className="text-sm md:text-base font-semibold text-text-primary truncate"
                                title={isNFT ? totalSupply : formatTokenAmount(totalSupply, decimals)}
                            >
                                {isNFT
                                    ? (totalSupply !== '0' ? `${totalSupply}${rawSymbol ? ' ' + rawSymbol : ''}` : '-')
                                    : `${formatTokenAmount(totalSupply, decimals)}${rawSymbol ? ' ' + rawSymbol : ''}`}
                            </div>
                        </div>
                        <div className="bg-black/20 rounded-lg p-3 md:p-4">
                            <div className="text-xs md:text-sm text-text-secondary mb-1">Holders</div>
                            <div className="text-sm md:text-base font-semibold text-text-primary">
                                {tokenInfo?.holderCount?.toLocaleString() ?? '-'}
                            </div>
                        </div>
                        <div className="bg-black/20 rounded-lg p-3 md:p-4">
                            <div className="text-xs md:text-sm text-text-secondary mb-1">Transfers</div>
                            <div className="text-sm md:text-base font-semibold text-text-primary">
                                {tokenInfo?.transferCount?.toLocaleString() ?? '-'}
                            </div>
                        </div>
                        <div className="bg-black/20 rounded-lg p-3 md:p-4">
                            <div className="text-xs md:text-sm text-text-secondary mb-1">
                                {isNFT ? 'Standard' : 'Decimals'}
                            </div>
                            <div className="text-sm md:text-base font-semibold text-text-primary">
                                {isNFT ? (tokenStandard?.replace(/^ERC-/, 'QRC-') ?? '-') : decimals}
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            {/* Tabs */}
            <div className="mb-4">
                <TabPillBar
                    ariaLabel="Token contract sections"
                    activeKey={activeTab}
                    onSelect={setActiveTab}
                    tabs={tabs.map((tab) => ({ key: tab.id, label: tab.label }))}
                />
            </div>

            {/* Tab Content */}
            <div className="card overflow-hidden">
                {/* Overview Tab */}
                {activeTab === 'overview' && (
                    <div className="p-4 md:p-6 space-y-6">
                        <div>
                            <h3 className="font-display text-lg font-semibold text-text-primary mb-4">Contract Details</h3>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div>
                                    <div className="text-xs md:text-sm text-text-secondary mb-1">Creator</div>
                                    <div className="flex items-center gap-2">
                                        <AddressDisplay address={creatorAddress} />
                                        {creatorAddress && (
                                            <CopyButton value={creatorAddress} label="Copy address" />
                                        )}
                                    </div>
                                </div>

                                <div>
                                    <div className="text-xs md:text-sm text-text-secondary mb-1">Contract Size</div>
                                    <div className="text-sm text-text-secondary">
                                        {contractData.contractCode
                                            ? `${Math.floor(contractData.contractCode.length * 0.75).toLocaleString()} bytes`
                                            : '-'
                                        }
                                    </div>
                                </div>
                            </div>
                        </div>

                        {/* Contract Code (verified source / ABI / bytecode) + Read / Write tabs.
                            Replaces the standalone bytecode block on token pages too, same
                            behaviour as the address page, just nested inside the Overview tab. */}
                        {contractData.contractCode && (
                            <div>
                                <div className="flex items-center gap-2 flex-wrap mb-4">
                                    <h3 className="font-display text-lg font-semibold text-text-primary">Contract</h3>
                                    {contractData.verified && <VerifiedBadge />}
                                </div>
                                <ContractTabs
                                    // Forward the whole contractData and only override
                                    // the few required-string fields that ContractData
                                    // wants non-optional. Avoids hand-listing every
                                    // optional verification field, adding a new field
                                    // to ContractData just flows through.
                                    contractData={{
                                        ...contractData,
                                        creatorAddress: creatorAddress,
                                        address: address,
                                        contractCode: contractData.contractCode,
                                        creationTransaction: contractData.creationTransaction ?? '',
                                        isToken: contractData.isToken ?? true,
                                        status: contractData.status ?? '',
                                        decimals: contractData.decimals ?? 0,
                                        name: contractData.name ?? '',
                                        symbol: contractData.symbol ?? '',
                                        updatedAt: contractData.updatedAt ?? '',
                                        verified: contractData.verified ?? false,
                                    }}
                                />
                            </div>
                        )}

                        {/* Creation Transaction Details */}
                        <div>
                            <h3 className="font-display text-lg font-semibold text-text-primary mb-4">Creation Transaction</h3>
                            <div className="bg-black/20 rounded-lg p-4 space-y-3">
                                <div className="flex flex-col md:flex-row md:items-center justify-between gap-2">
                                    <div className="text-xs md:text-sm text-text-secondary">Transaction Hash</div>
                                    <div className="flex items-center gap-2 min-w-0">
                                        <Link
                                            href={`/tx/${creationTxHash}`}
                                            className="text-accent hover:text-accent-hover font-mono text-xs md:text-sm break-all"
                                        >
                                            {creationTxHash || 'Unknown'}
                                        </Link>
                                        {creationTxHash && (
                                            <CopyButton value={creationTxHash} label="Copy transaction hash" />
                                        )}
                                    </div>
                                </div>

                                <div className="border-t border-border" />

                                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                    <div className="flex justify-between md:flex-col">
                                        <div className="text-xs md:text-sm text-text-secondary">Block</div>
                                        <Link
                                            href={`/block/${creationTx?.BlockNumber || tokenInfo?.creationBlock || ''}`}
                                            className="text-accent hover:text-accent-hover text-sm"
                                        >
                                            {creationTx?.BlockNumber
                                                ? parseInt(creationTx.BlockNumber, 16).toLocaleString()
                                                : tokenInfo?.creationBlock
                                                    ? parseInt(tokenInfo.creationBlock, 16).toLocaleString()
                                                    : '-'}
                                        </Link>
                                    </div>

                                    <div className="flex justify-between md:flex-col">
                                        <div className="text-xs md:text-sm text-text-secondary">Timestamp</div>
                                        <div className="text-sm text-text-secondary">
                                            {creationTx?.BlockTimestamp
                                                ? formatTimestamp(creationTx.BlockTimestamp)
                                                : '-'}
                                        </div>
                                    </div>

                                    <div className="flex justify-between md:flex-col">
                                        <div className="text-xs md:text-sm text-text-secondary">Gas Used</div>
                                        <div className="text-sm text-text-secondary">
                                            {creationTx?.GasUsed
                                                ? parseInt(creationTx.GasUsed, 16).toLocaleString()
                                                : '-'}
                                        </div>
                                    </div>

                                    <div className="flex justify-between md:flex-col">
                                        <div className="text-xs md:text-sm text-text-secondary">Gas Price</div>
                                        <div className="text-sm text-text-secondary">
                                            {creationTx?.GasPrice
                                                ? `${(parseInt(creationTx.GasPrice, 16) / 1e9).toFixed(2)} Shor`
                                                : '-'}
                                        </div>
                                    </div>

                                    <div className="flex justify-between md:flex-col">
                                        <div className="text-xs md:text-sm text-text-secondary">Transaction Fee</div>
                                        <div className="text-sm text-text-secondary">
                                            {(() => {
                                                if (!creationTx?.GasUsed || !creationTx?.GasPrice) return '-';
                                                const [formattedFee] = formatAmount(`0x${(BigInt(creationTx.GasUsed) * BigInt(creationTx.GasPrice)).toString(16)}`);
                                                return `${formattedFee} QRL`;
                                            })()}
                                        </div>
                                    </div>

                                    <div className="flex justify-between md:flex-col">
                                        <div className="text-xs md:text-sm text-text-secondary">Value</div>
                                        <div className="text-sm text-text-secondary">
                                            {(() => {
                                                if (!creationTx?.Value) return '0 QRL';
                                                const [formattedValue] = formatAmount(creationTx.Value);
                                                return `${formattedValue} QRL`;
                                            })()}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                )}

                {/* Holders Tab */}
                {activeTab === 'holders' && (
                    <div>
                        {/* Phase 2: per-tokenID filter for NFT collections. The input
                            debounces locally and only sets the actual filter on submit,
                            so each keystroke doesn't trigger a network request. */}
                        {isNFT && (
                            <div className="flex flex-wrap items-center gap-2 px-4 py-3 border-b border-border bg-black/20">
                                <label htmlFor="tokenIDFilter" className="text-xs text-text-secondary">
                                    Filter by tokenID:
                                </label>
                                <input
                                    id="tokenIDFilter"
                                    type="text"
                                    inputMode="numeric"
                                    placeholder="e.g. 1"
                                    value={holderTokenIDInput}
                                    onChange={(e) => setHolderTokenIDInput(e.target.value.replace(/[^0-9]/g, ''))}
                                    onKeyDown={(e) => {
                                        if (e.key === 'Enter') {
                                            setHoldersPage(0);
                                            setHolderTokenIDFilter(holderTokenIDInput.trim());
                                        }
                                    }}
                                    className="px-2 py-1 rounded bg-black/40 border border-border text-sm text-text-primary font-mono w-32"
                                />
                                <button
                                    onClick={() => {
                                        setHoldersPage(0);
                                        setHolderTokenIDFilter(holderTokenIDInput.trim());
                                    }}
                                    className="px-3 py-1 rounded bg-accent/20 text-accent text-sm hover:bg-accent/30"
                                >
                                    Apply
                                </button>
                                {holderTokenIDFilter && (
                                    <button
                                        onClick={() => {
                                            setHolderTokenIDInput('');
                                            setHolderTokenIDFilter('');
                                            setHoldersPage(0);
                                        }}
                                        className="px-3 py-1 rounded bg-surface-2 text-sm hover:bg-surface-3"
                                    >
                                        Clear
                                    </button>
                                )}
                                {holderTokenIDFilter && (
                                    <span className="text-xs text-text-secondary">
                                        Showing holders of tokenID <span className="font-mono text-text-primary">{holderTokenIDFilter}</span>
                                    </span>
                                )}
                            </div>
                        )}

                        {loading ? (
                            <div className="p-8 text-center text-text-secondary">Loading holders...</div>
                        ) : holders.length === 0 ? (
                            <div className="p-8 text-center text-text-secondary">No holders found</div>
                        ) : (
                            <>
                                <div className="overflow-x-auto">
                                    <table aria-label="Token holders" className="w-full">
                                        <thead className="bg-black/30">
                                            <tr>
                                                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-secondary uppercase">#</th>
                                                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-secondary uppercase">Address</th>
                                                {isNFT && holderTokenIDFilter === '' && (
                                                    <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-secondary uppercase hidden md:table-cell">
                                                        {tokenStandard === 'ERC-721' ? 'NFTs owned' : 'Total quantity'}
                                                    </th>
                                                )}
                                                {isNFT && holderTokenIDFilter !== '' && (
                                                    <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-secondary uppercase">Token ID</th>
                                                )}
                                                <th scope="col" className="px-4 py-3 text-right text-xs font-medium text-text-secondary uppercase">Balance</th>
                                                {!isNFT && (
                                                    <th scope="col" className="px-4 py-3 text-right text-xs font-medium text-text-secondary uppercase hidden md:table-cell">Share</th>
                                                )}
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-border">
                                            {holders.map((holder, idx) => {
                                                const totalSupplyBigInt = totalSupply ? BigInt(totalSupply) : BigInt(0);
                                                let sharePercent = 0;
                                                try {
                                                    const share = totalSupplyBigInt > BigInt(0) && holder.balance
                                                        ? ((BigInt(holder.balance) * BigInt(10000)) / totalSupplyBigInt)
                                                        : BigInt(0);
                                                    sharePercent = Number(share) / 100;
                                                } catch {
                                                    sharePercent = 0;
                                                }

                                                const rowKey = holder.tokenID
                                                    ? `${holder.holderAddress}-${holder.tokenID}`
                                                    : holder.holderAddress;

                                                // ERC-721 balance column is the per-id "1"; show
                                                // a count instead in the aggregated view (which is
                                                // the sum coming from the backend).
                                                const balanceCell = isNFT
                                                    ? holder.balance
                                                    : `${formatTokenAmount(holder.balance, decimals)}${rawSymbol ? ' ' + rawSymbol : ''}`;

                                                return (
                                                    <tr key={rowKey} className="hover:bg-white/5">
                                                        <td className="px-4 py-3 text-sm text-text-secondary">
                                                            {holdersPage * limit + idx + 1}
                                                        </td>
                                                        <td className="px-4 py-3">
                                                            <AddressDisplay address={holder.holderAddress} truncate />
                                                        </td>
                                                        {isNFT && holderTokenIDFilter === '' && (
                                                            <td className="px-4 py-3 text-left text-sm text-text-secondary font-mono hidden md:table-cell">
                                                                {holder.balance}
                                                            </td>
                                                        )}
                                                        {isNFT && holderTokenIDFilter !== '' && (
                                                            <td className="px-4 py-3 text-left text-sm text-text-primary font-mono">
                                                                #{holder.tokenID ?? holderTokenIDFilter}
                                                            </td>
                                                        )}
                                                        <td className="px-4 py-3 text-right text-sm text-text-primary font-mono">
                                                            {balanceCell}
                                                        </td>
                                                        {!isNFT && (
                                                            <td className="px-4 py-3 text-right text-sm text-text-secondary hidden md:table-cell">
                                                                {sharePercent.toFixed(2)}%
                                                            </td>
                                                        )}
                                                    </tr>
                                                );
                                            })}
                                        </tbody>
                                    </table>
                                </div>

                                {/* Pagination */}
                                {holdersTotal > limit && (
                                    <div className="flex items-center justify-between px-4 py-3 border-t border-border">
                                        <div className="text-sm text-text-secondary">
                                            Showing {holdersPage * limit + 1} - {Math.min((holdersPage + 1) * limit, holdersTotal)} of {holdersTotal}
                                        </div>
                                        <div className="flex gap-2">
                                            <button
                                                aria-label="Go to previous page"
                                                onClick={() => setHoldersPage(p => Math.max(0, p - 1))}
                                                disabled={holdersPage === 0}
                                                className="px-3 py-1 rounded bg-surface-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-surface-3"
                                            >
                                                Previous
                                            </button>
                                            <button
                                                aria-label="Go to next page"
                                                onClick={() => setHoldersPage(p => p + 1)}
                                                disabled={(holdersPage + 1) * limit >= holdersTotal}
                                                className="px-3 py-1 rounded bg-surface-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-surface-3"
                                            >
                                                Next
                                            </button>
                                        </div>
                                    </div>
                                )}
                            </>
                        )}
                    </div>
                )}

                {/* Tokens Tab. Phase 2: lists every distinct tokenID minted
                    on this NFT contract, with the holder count for each id.
                    Clicking a row jumps to the holders tab filtered to that id. */}
                {activeTab === 'tokens' && (
                    <div>
                        {loading ? (
                            <div className="p-8 text-center text-text-secondary">Loading tokens...</div>
                        ) : tokensList.length === 0 ? (
                            <div className="p-8 text-center text-text-secondary">No tokens have been minted yet</div>
                        ) : (
                            <>
                                <div className="overflow-x-auto">
                                    <table aria-label="Token IDs" className="w-full">
                                        <thead className="bg-black/30">
                                            <tr>
                                                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-secondary uppercase w-16"></th>
                                                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-secondary uppercase">Token</th>
                                                <th scope="col" className="px-4 py-3 text-right text-xs font-medium text-text-secondary uppercase">Holders</th>
                                                <th scope="col" className="px-4 py-3 text-right text-xs font-medium text-text-secondary uppercase hidden md:table-cell">Standard</th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-border">
                                            {tokensList.map((t) => {
                                                // Phase 3b: render the off-chain image if the fetcher has
                                                // populated it; fall back to a #N monogram tile. The "Token"
                                                // column shows the metadata name when present, otherwise
                                                // just "#<id>" — keeps unfetched / no-metadata cases clean.
                                                const tokenLabel = t.name?.trim() || `#${t.tokenID}`;
                                                const subLabel = t.name?.trim() ? `#${t.tokenID}` : null;
                                                return (
                                                    <tr
                                                        key={t.tokenID}
                                                        className="hover:bg-white/5 cursor-pointer"
                                                        onClick={() => {
                                                            setHolderTokenIDInput(t.tokenID);
                                                            setHolderTokenIDFilter(t.tokenID);
                                                            setHoldersPage(0);
                                                            setActiveTab('holders');
                                                        }}
                                                    >
                                                        <td className="px-4 py-3">
                                                            {t.image ? (
                                                                <div className="relative w-10 h-10 rounded-md overflow-hidden border border-border bg-black/30">
                                                                    <ImageWithFallback
                                                                        src={t.image}
                                                                        alt={tokenLabel}
                                                                        fill
                                                                        sizes="40px"
                                                                        className="object-cover"
                                                                        fallback={
                                                                            <div className="absolute inset-0 bg-surface-3 flex items-center justify-center text-xs font-mono text-text-secondary">
                                                                                {compactTokenIDLabel(t.tokenID)}
                                                                            </div>
                                                                        }
                                                                    />
                                                                </div>
                                                            ) : (
                                                                <div
                                                                    className="w-10 h-10 rounded-md bg-surface-3 flex items-center justify-center text-xs font-mono text-text-secondary"
                                                                    title={`#${t.tokenID}`}
                                                                >
                                                                    #{compactTokenIDLabel(t.tokenID)}
                                                                </div>
                                                            )}
                                                        </td>
                                                        <td className="px-4 py-3 text-sm">
                                                            <div className="text-text-primary">{tokenLabel}</div>
                                                            {subLabel && <div className="text-xs text-text-muted font-mono">{subLabel}</div>}
                                                        </td>
                                                        <td className="px-4 py-3 text-right text-sm text-text-primary">{t.holderCount}</td>
                                                        <td className="px-4 py-3 text-right text-xs text-text-secondary hidden md:table-cell">
                                                            {t.tokenStandard ? t.tokenStandard.replace(/^ERC-/, 'QRC-') : '-'}
                                                        </td>
                                                    </tr>
                                                );
                                            })}
                                        </tbody>
                                    </table>
                                </div>

                                {/* Pagination */}
                                {tokensTotal > limit && (
                                    <div className="flex items-center justify-between px-4 py-3 border-t border-border">
                                        <div className="text-sm text-text-secondary">
                                            Showing {tokensPage * limit + 1} - {Math.min((tokensPage + 1) * limit, tokensTotal)} of {tokensTotal}
                                        </div>
                                        <div className="flex gap-2">
                                            <button
                                                aria-label="Go to previous page"
                                                onClick={() => setTokensPage(p => Math.max(0, p - 1))}
                                                disabled={tokensPage === 0}
                                                className="px-3 py-1 rounded bg-surface-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-surface-3"
                                            >
                                                Previous
                                            </button>
                                            <button
                                                aria-label="Go to next page"
                                                onClick={() => setTokensPage(p => p + 1)}
                                                disabled={(tokensPage + 1) * limit >= tokensTotal}
                                                className="px-3 py-1 rounded bg-surface-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-surface-3"
                                            >
                                                Next
                                            </button>
                                        </div>
                                    </div>
                                )}
                            </>
                        )}
                    </div>
                )}

                {/* Transfers Tab */}
                {activeTab === 'transfers' && (
                    <div>
                        {loading ? (
                            <div className="p-8 text-center text-text-secondary">Loading transfers...</div>
                        ) : transfers.length === 0 ? (
                            <div className="p-8 text-center text-text-secondary">No transfers found</div>
                        ) : (
                            <>
                                <div className="overflow-x-auto">
                                    <table aria-label="Token transfers" className="w-full">
                                        <thead className="bg-black/30">
                                            <tr>
                                                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-secondary uppercase">Tx Hash</th>
                                                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-secondary uppercase">From</th>
                                                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-secondary uppercase">To</th>
                                                <th scope="col" className="px-4 py-3 text-right text-xs font-medium text-text-secondary uppercase">Amount</th>
                                                <th scope="col" className="px-4 py-3 text-right text-xs font-medium text-text-secondary uppercase hidden md:table-cell">Time</th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-border">
                                            {transfers.map((transfer) => (
                                                <tr key={`${transfer.txHash}-${transfer.from}-${transfer.to}`} className="hover:bg-white/5">
                                                    <td className="px-4 py-3">
                                                        <Link
                                                            href={`/tx/${transfer.txHash}`}
                                                            className="text-accent hover:text-accent-hover font-mono text-xs"
                                                        >
                                                            {transfer.txHash.slice(0, 10)}...{transfer.txHash.slice(-8)}
                                                        </Link>
                                                    </td>
                                                    <td className="px-4 py-3">
                                                        <AddressDisplay address={transfer.from} truncate />
                                                    </td>
                                                    <td className="px-4 py-3">
                                                        <AddressDisplay address={transfer.to} truncate />
                                                    </td>
                                                    <td className="px-4 py-3 text-right text-sm text-text-primary font-mono">
                                                        {formatTokenAmount(transfer.amount, transfer.tokenDecimals || decimals)}{rawSymbol ? ' ' + rawSymbol : ''}
                                                    </td>
                                                    <td className="px-4 py-3 text-right text-xs text-text-secondary hidden md:table-cell">
                                                        {formatTimestamp(transfer.timestamp)}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>

                                {/* Pagination */}
                                {transfersTotal > limit && (
                                    <div className="flex items-center justify-between px-4 py-3 border-t border-border">
                                        <div className="text-sm text-text-secondary">
                                            Showing {transfersPage * limit + 1} - {Math.min((transfersPage + 1) * limit, transfersTotal)} of {transfersTotal}
                                        </div>
                                        <div className="flex gap-2">
                                            <button
                                                aria-label="Go to previous page"
                                                onClick={() => setTransfersPage(p => Math.max(0, p - 1))}
                                                disabled={transfersPage === 0}
                                                className="px-3 py-1 rounded bg-surface-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-surface-3"
                                            >
                                                Previous
                                            </button>
                                            <button
                                                aria-label="Go to next page"
                                                onClick={() => setTransfersPage(p => p + 1)}
                                                disabled={(transfersPage + 1) * limit >= transfersTotal}
                                                className="px-3 py-1 rounded bg-surface-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-surface-3"
                                            >
                                                Next
                                            </button>
                                        </div>
                                    </div>
                                )}
                            </>
                        )}
                    </div>
                )}
            </div>
        </div>
    );
}
