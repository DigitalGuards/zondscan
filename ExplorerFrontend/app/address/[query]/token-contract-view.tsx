'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import CopyAddressButton from "../../components/CopyAddressButton";
import QRCodeButton from "../../components/QRCodeButton";
import { formatAmount } from "../../lib/helpers";

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

interface TokenContractViewProps {
    address: string;
    contractData: {
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
    if (!address) return <span className="text-gray-500">Unknown</span>;

    const display = truncate ? `${address.slice(0, 10)}...${address.slice(-8)}` : address;
    return (
        <Link href={`/address/${address}`} className="text-accent hover:text-accent-hover font-mono text-xs md:text-sm">
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
    const [activeTab, setActiveTab] = useState<'overview' | 'holders' | 'transfers'>('overview');
    const [tokenInfo, setTokenInfo] = useState<TokenInfo | null>(null);
    const [holders, setHolders] = useState<TokenHolder[]>([]);
    const [transfers, setTransfers] = useState<TokenTransfer[]>([]);
    const [holdersTotal, setHoldersTotal] = useState(0);
    const [transfersTotal, setTransfersTotal] = useState(0);
    const [holdersPage, setHoldersPage] = useState(0);
    const [transfersPage, setTransfersPage] = useState(0);
    const [loading, setLoading] = useState(true);
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

    // Fetch holders when tab is active
    useEffect(() => {
        if (activeTab !== 'holders') return;

        const fetchHolders = async () => {
            setLoading(true);
            try {
                const res = await fetch(`${handlerUrl}/token/${address}/holders?page=${holdersPage}&limit=${limit}`);
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
    }, [address, handlerUrl, activeTab, holdersPage]);

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
    const symbol = tokenInfo?.symbol ?? contractData.symbol ?? 'TOKEN';
    const name = tokenInfo?.name ?? contractData.name ?? 'Unknown Token';
    const totalSupply = tokenInfo?.totalSupply ?? contractData.totalSupply ?? '0';

    const tabs = [
        { id: 'overview', label: 'Overview' },
        { id: 'holders', label: `Holders${tokenInfo ? ` (${tokenInfo.holderCount})` : ''}` },
        { id: 'transfers', label: `Transfers${tokenInfo ? ` (${tokenInfo.transferCount})` : ''}` },
    ];

    return (
        <div className="py-3 md:py-6 lg:py-8 px-3 md:px-6 lg:px-8 max-w-[1200px] mx-auto">
            {/* Token Header Card */}
            <div className="relative overflow-hidden rounded-xl md:rounded-2xl bg-card-gradient border border-border shadow-lg md:shadow-xl mb-4 md:mb-6">
                <div className="p-4 md:p-6 lg:p-8">
                    {/* Token Identity */}
                    <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6 pb-4 border-b border-gray-700">
                        <div className="flex items-center gap-4">
                            {/* Token Icon */}
                            <div className="w-12 h-12 md:w-16 md:h-16 rounded-full bg-gradient-to-br from-[#ffa729] to-[#ff6b00] flex items-center justify-center text-xl md:text-2xl font-bold text-white">
                                {symbol.charAt(0)}
                            </div>
                            <div>
                                <div className="flex items-center gap-2">
                                    <h1 className="text-xl md:text-2xl font-bold text-white">{name}</h1>
                                    <span className="px-2 py-0.5 rounded bg-[#ffa729]/20 text-[#ffa729] text-sm font-medium">
                                        {symbol}
                                    </span>
                                </div>
                                <div className="flex items-center gap-2 mt-1">
                                    <span className="text-xs md:text-sm text-gray-400 font-mono">{address}</span>
                                    <CopyAddressButton address={address} />
                                    <QRCodeButton address={address} />
                                </div>
                            </div>
                        </div>
                        <div className="px-3 py-1.5 rounded-lg bg-green-500/20 text-green-400 text-sm font-medium self-start">
                            QRC-20 Token
                        </div>
                    </div>

                    {/* Token Stats Grid */}
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                        <div className="bg-black/20 rounded-lg p-3 md:p-4">
                            <div className="text-xs md:text-sm text-gray-400 mb-1">Total Supply</div>
                            <div className="text-sm md:text-base font-semibold text-white truncate" title={formatTokenAmount(totalSupply, decimals)}>
                                {formatTokenAmount(totalSupply, decimals)} {symbol}
                            </div>
                        </div>
                        <div className="bg-black/20 rounded-lg p-3 md:p-4">
                            <div className="text-xs md:text-sm text-gray-400 mb-1">Holders</div>
                            <div className="text-sm md:text-base font-semibold text-white">
                                {tokenInfo?.holderCount?.toLocaleString() ?? '-'}
                            </div>
                        </div>
                        <div className="bg-black/20 rounded-lg p-3 md:p-4">
                            <div className="text-xs md:text-sm text-gray-400 mb-1">Transfers</div>
                            <div className="text-sm md:text-base font-semibold text-white">
                                {tokenInfo?.transferCount?.toLocaleString() ?? '-'}
                            </div>
                        </div>
                        <div className="bg-black/20 rounded-lg p-3 md:p-4">
                            <div className="text-xs md:text-sm text-gray-400 mb-1">Decimals</div>
                            <div className="text-sm md:text-base font-semibold text-white">
                                {decimals}
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            {/* Tabs */}
            <div className="flex border-b border-gray-700 mb-4">
                {tabs.map((tab) => (
                    <button
                        key={tab.id}
                        onClick={() => setActiveTab(tab.id as typeof activeTab)}
                        className={`px-4 py-3 text-sm font-medium transition-colors relative
                            ${activeTab === tab.id
                                ? 'text-[#ffa729]'
                                : 'text-gray-400 hover:text-gray-300'
                            }`}
                    >
                        {tab.label}
                        {activeTab === tab.id && (
                            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-[#ffa729]" />
                        )}
                    </button>
                ))}
            </div>

            {/* Tab Content */}
            <div className="bg-card-gradient rounded-xl border border-border overflow-hidden">
                {/* Overview Tab */}
                {activeTab === 'overview' && (
                    <div className="p-4 md:p-6 space-y-4">
                        <h3 className="text-lg font-semibold text-accent mb-4">Contract Details</h3>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div>
                                <div className="text-xs md:text-sm text-gray-400 mb-1">Creator</div>
                                <div className="flex items-center gap-2">
                                    <AddressDisplay address={tokenInfo?.creatorAddress || contractData.creatorAddress || ''} />
                                    {(tokenInfo?.creatorAddress || contractData.creatorAddress) && (
                                        <CopyAddressButton address={tokenInfo?.creatorAddress || contractData.creatorAddress || ''} />
                                    )}
                                </div>
                            </div>

                            <div>
                                <div className="text-xs md:text-sm text-gray-400 mb-1">Creation Transaction</div>
                                <div className="flex items-center gap-2">
                                    <Link
                                        href={`/tx/${tokenInfo?.creationTxHash || contractData.creationTransaction}`}
                                        className="text-accent hover:text-accent-hover font-mono text-xs md:text-sm truncate"
                                    >
                                        {tokenInfo?.creationTxHash || contractData.creationTransaction || 'Unknown'}
                                    </Link>
                                </div>
                            </div>

                            <div>
                                <div className="text-xs md:text-sm text-gray-400 mb-1">Creation Block</div>
                                <Link
                                    href={`/block/${tokenInfo?.creationBlock || ''}`}
                                    className="text-accent hover:text-accent-hover text-sm"
                                >
                                    {tokenInfo?.creationBlock ? parseInt(tokenInfo.creationBlock, 16).toLocaleString() : '-'}
                                </Link>
                            </div>

                            <div>
                                <div className="text-xs md:text-sm text-gray-400 mb-1">Contract Size</div>
                                <div className="text-sm text-gray-300">
                                    {contractData.contractCode
                                        ? `${Math.floor(contractData.contractCode.length * 0.75).toLocaleString()} bytes`
                                        : '-'
                                    }
                                </div>
                            </div>
                        </div>
                    </div>
                )}

                {/* Holders Tab */}
                {activeTab === 'holders' && (
                    <div>
                        {loading ? (
                            <div className="p-8 text-center text-gray-400">Loading holders...</div>
                        ) : holders.length === 0 ? (
                            <div className="p-8 text-center text-gray-400">No holders found</div>
                        ) : (
                            <>
                                <div className="overflow-x-auto">
                                    <table className="w-full">
                                        <thead className="bg-black/30">
                                            <tr>
                                                <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase">#</th>
                                                <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase">Address</th>
                                                <th className="px-4 py-3 text-right text-xs font-medium text-gray-400 uppercase">Balance</th>
                                                <th className="px-4 py-3 text-right text-xs font-medium text-gray-400 uppercase hidden md:table-cell">Share</th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-gray-700/50">
                                            {holders.map((holder, idx) => {
                                                const totalSupplyBigInt = totalSupply ? BigInt(totalSupply) : BigInt(0);
                                                const share = totalSupplyBigInt > 0n && holder.balance
                                                    ? ((BigInt(holder.balance) * BigInt(10000)) / totalSupplyBigInt)
                                                    : BigInt(0);
                                                const sharePercent = Number(share) / 100;

                                                return (
                                                    <tr key={holder.holderAddress} className="hover:bg-white/5">
                                                        <td className="px-4 py-3 text-sm text-gray-400">
                                                            {holdersPage * limit + idx + 1}
                                                        </td>
                                                        <td className="px-4 py-3">
                                                            <AddressDisplay address={holder.holderAddress} truncate />
                                                        </td>
                                                        <td className="px-4 py-3 text-right text-sm text-white font-mono">
                                                            {formatTokenAmount(holder.balance, decimals)} {symbol}
                                                        </td>
                                                        <td className="px-4 py-3 text-right text-sm text-gray-400 hidden md:table-cell">
                                                            {sharePercent.toFixed(2)}%
                                                        </td>
                                                    </tr>
                                                );
                                            })}
                                        </tbody>
                                    </table>
                                </div>

                                {/* Pagination */}
                                {holdersTotal > limit && (
                                    <div className="flex items-center justify-between px-4 py-3 border-t border-gray-700">
                                        <div className="text-sm text-gray-400">
                                            Showing {holdersPage * limit + 1} - {Math.min((holdersPage + 1) * limit, holdersTotal)} of {holdersTotal}
                                        </div>
                                        <div className="flex gap-2">
                                            <button
                                                onClick={() => setHoldersPage(p => Math.max(0, p - 1))}
                                                disabled={holdersPage === 0}
                                                className="px-3 py-1 rounded bg-gray-700 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-600"
                                            >
                                                Previous
                                            </button>
                                            <button
                                                onClick={() => setHoldersPage(p => p + 1)}
                                                disabled={(holdersPage + 1) * limit >= holdersTotal}
                                                className="px-3 py-1 rounded bg-gray-700 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-600"
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
                            <div className="p-8 text-center text-gray-400">Loading transfers...</div>
                        ) : transfers.length === 0 ? (
                            <div className="p-8 text-center text-gray-400">No transfers found</div>
                        ) : (
                            <>
                                <div className="overflow-x-auto">
                                    <table className="w-full">
                                        <thead className="bg-black/30">
                                            <tr>
                                                <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase">Tx Hash</th>
                                                <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase">From</th>
                                                <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase">To</th>
                                                <th className="px-4 py-3 text-right text-xs font-medium text-gray-400 uppercase">Amount</th>
                                                <th className="px-4 py-3 text-right text-xs font-medium text-gray-400 uppercase hidden md:table-cell">Time</th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-gray-700/50">
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
                                                    <td className="px-4 py-3 text-right text-sm text-white font-mono">
                                                        {formatTokenAmount(transfer.amount, transfer.tokenDecimals || decimals)} {symbol}
                                                    </td>
                                                    <td className="px-4 py-3 text-right text-xs text-gray-400 hidden md:table-cell">
                                                        {formatTimestamp(transfer.timestamp)}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>

                                {/* Pagination */}
                                {transfersTotal > limit && (
                                    <div className="flex items-center justify-between px-4 py-3 border-t border-gray-700">
                                        <div className="text-sm text-gray-400">
                                            Showing {transfersPage * limit + 1} - {Math.min((transfersPage + 1) * limit, transfersTotal)} of {transfersTotal}
                                        </div>
                                        <div className="flex gap-2">
                                            <button
                                                onClick={() => setTransfersPage(p => Math.max(0, p - 1))}
                                                disabled={transfersPage === 0}
                                                className="px-3 py-1 rounded bg-gray-700 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-600"
                                            >
                                                Previous
                                            </button>
                                            <button
                                                onClick={() => setTransfersPage(p => p + 1)}
                                                disabled={(transfersPage + 1) * limit >= transfersTotal}
                                                className="px-3 py-1 rounded bg-gray-700 text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-600"
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
