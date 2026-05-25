'use client';

import { useState, useEffect } from 'react';
import CopyButton from "../../components/CopyButton";
import QRCodeButton from "../../components/QRCodeButton";
import ContractTabs from "../../components/ContractTabs";
import VerifiedBadge from "../../components/VerifiedBadge";
import BalanceDisplay from "./balance-display";
import ActivityDisplay from "./activity-display";
import AddressTabs from "./address-tabs";
import type { AddressData } from "@/app/types";
import Link from "next/link";
import Breadcrumbs from "../../components/Breadcrumbs";

interface AddressViewProps {
    addressData: AddressData;
    addressSegment: string;
}

const AddressDisplay = ({ address }: { address: string }): JSX.Element => {
    const [isMobile, setIsMobile] = useState(false);

    useEffect(() => {
        const checkScreenSize = (): void => {
            setIsMobile(window.innerWidth < 768);
        };
        
        checkScreenSize();
        window.addEventListener('resize', checkScreenSize);
        return () => window.removeEventListener('resize', checkScreenSize);
    }, []);

    const displayAddress = isMobile ? `${address.slice(0, 8)}...${address.slice(-6)}` : address;

    return (
        <div className="text-sm lg:text-base font-mono text-gray-300 break-all lg:break-normal">
            {displayAddress}
        </div>
    );
};

export default function AddressView({ addressData, addressSegment }: AddressViewProps): JSX.Element {
    const { balance } = addressData.address;
    const { rank } = addressData;

    let firstSeen = 0;
    let lastSeen = 0;
    if (addressData.transactions_by_address && Array.isArray(addressData.transactions_by_address) && addressData.transactions_by_address.length > 0) {
        const timestamps = addressData.transactions_by_address.map(tx => tx.TimeStamp);
        firstSeen = Math.min(...timestamps);
        lastSeen = Math.max(...timestamps);
    }

    let addressType = "";
    let addressIcon = null;
    const contractData = addressData.contract_code;

    if (contractData && contractData.contractCode) {
        addressType = "Contract";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 md:h-6 md:w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
            </svg>
        );
    } else if (addressSegment.startsWith("Q2")) {
        addressType = "Dilithium Address";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 md:h-6 md:w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
        );
    } else if (addressSegment.startsWith("Q")) {
        addressType = "QRL Address";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 md:h-6 md:w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
        );
    } else {
        addressType = "Address";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 md:h-6 md:w-6 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 9V7a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2m2 4h10a2 2 0 002-2v-6a2 2 0 00-2-2H9a2 2 0 00-2 2v6a2 2 0 002 2zm7-5a2 2 0 11-4 0 2 2 0 014 0z" />
            </svg>
        );
    }

    // Visually-hidden h1 anchors the main landmark for assistive tech;
    // the visible heading is the address text itself (rendered below
    // via AddressDisplay), which doesn't carry semantic heading weight.
    const addressHeadingId = 'address-page-heading';
    // addressIcon is rendered both desktop + mobile; tag both so the
    // duplicate doesn't get announced as two graphics.
    const decorativeIcon = addressIcon ? (
        <span aria-hidden="true">{addressIcon}</span>
    ) : null;

    return (
        <main className="detail-content" aria-labelledby={addressHeadingId}>
            <h1 id={addressHeadingId} className="sr-only">
                {addressType || 'Address'} {addressSegment}
            </h1>
            <Breadcrumbs items={[
                { label: 'Address' },
                { label: `${addressSegment.slice(0, 10)}...${addressSegment.slice(-6)}` },
            ]} />
            <section
                aria-labelledby={addressHeadingId}
                className="relative overflow-hidden rounded-xl md:rounded-2xl
                        bg-card-gradient
                        border border-border shadow-lg md:shadow-xl mb-4 md:mb-6 lg:mb-8"
            >
                <div className="p-3 md:p-6 lg:p-8">
                    {/* Header */}
                    <div className="flex flex-col lg:flex-row lg:items-center justify-between mb-4 md:mb-6 lg:mb-8 pb-3 md:pb-4 lg:pb-6 border-b border-gray-700">
                        <div className="flex items-start lg:items-center mb-3 lg:mb-0">
                            <div className="hidden lg:block">{decorativeIcon}</div>
                            <div className="flex-1 lg:ml-4">
                                <div className="flex items-center">
                                    <div className="block lg:hidden mr-2">{decorativeIcon}</div>
                                    <div className="text-xs md:text-sm font-medium text-gray-400">{addressType}</div>
                                </div>
                                <div className="flex flex-col lg:flex-row lg:items-center mt-1 gap-2">
                                    <AddressDisplay address={addressSegment} />
                                    {addressSegment && (
                                        <div className="flex items-center gap-2 mb-2 lg:mb-0 lg:ml-4">
                                            <CopyButton value={addressSegment} label="Copy address" />
                                            <QRCodeButton address={addressSegment} />
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                        <div className="px-2 md:px-3 lg:px-4 py-1 md:py-1.5 lg:py-2 rounded-lg md:rounded-xl bg-border bg-opacity-20 self-start lg:self-center">
                            <span className="text-xs md:text-sm font-medium text-accent">Rank #{rank}</span>
                        </div>
                    </div>

                    {/* Stats Grid */}
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 md:gap-4 lg:gap-6">
                        <BalanceDisplay balance={balance} />
                        <ActivityDisplay firstSeen={firstSeen} lastSeen={lastSeen} />
                    </div>

                    {/* Contract Information */}
                    {contractData && contractData.contractCode && (
                        <section aria-labelledby="contract-info-heading" className="mt-4 md:mt-6">
                            <div className="card-simple p-3 md:p-4 lg:p-6 space-y-3 md:space-y-4">
                                <div className="flex items-center gap-2 flex-wrap">
                                    <h3 id="contract-info-heading" className="text-base md:text-lg font-semibold text-accent">
                                        {contractData.tokenStandard === 'ERC-721'
                                            ? 'NFT Collection'
                                            : contractData.tokenStandard === 'ERC-1155'
                                              ? 'Multi-Token Collection'
                                              : contractData.isToken
                                                ? 'Token Contract'
                                                : 'Contract'} Information
                                    </h3>
                                    {contractData.verified && <VerifiedBadge />}
                                </div>
                                
                                <div className="space-y-3">
                                    {/* Creator Address */}
                                    <div>
                                        <div className="text-xs md:text-sm text-gray-400 mb-1">Creator Address</div>
                                        <div className="flex items-center space-x-2">
                                            <AddressDisplay
                                                address={contractData.creatorAddress || 'Unknown'}
                                            />
                                            {contractData.creatorAddress && (
                                                <CopyButton value={contractData.creatorAddress} label="Copy address" />
                                            )}
                                        </div>
                                    </div>

                                    {/* Token / NFT Information */}
                                    {contractData.isToken && (
                                        <>
                                            {/* Standard label, surfaces the QRC-X branding
                                                (DB value stays ERC-X). */}
                                            {contractData.tokenStandard && (
                                                <div>
                                                    <div className="text-xs md:text-sm text-gray-400 mb-1">Token Standard</div>
                                                    <div className="text-xs md:text-sm text-gray-300">
                                                        {contractData.tokenStandard.replace(/^ERC-/, 'QRC-')}
                                                    </div>
                                                </div>
                                            )}

                                            {/* Name */}
                                            <div>
                                                <div className="text-xs md:text-sm text-gray-400 mb-1">
                                                    {contractData.tokenStandard === 'ERC-20' || !contractData.tokenStandard ? 'Token Name' : 'Collection Name'}
                                                </div>
                                                <div className="text-xs md:text-sm text-gray-300">
                                                    {contractData.name || 'Unknown'}
                                                </div>
                                            </div>

                                            {/* Symbol */}
                                            <div>
                                                <div className="text-xs md:text-sm text-gray-400 mb-1">
                                                    {contractData.tokenStandard === 'ERC-20' || !contractData.tokenStandard ? 'Token Symbol' : 'Collection Symbol'}
                                                </div>
                                                <div className="text-xs md:text-sm text-gray-300">
                                                    {contractData.symbol || 'Unknown'}
                                                </div>
                                            </div>

                                            {/* Decimals, ERC-20 only */}
                                            {(contractData.tokenStandard === 'ERC-20' || !contractData.tokenStandard) && (
                                                <div>
                                                    <div className="text-xs md:text-sm text-gray-400 mb-1">Token Decimals</div>
                                                    <div className="text-xs md:text-sm text-gray-300">
                                                        {contractData.decimals || '0'}
                                                    </div>
                                                </div>
                                            )}
                                        </>
                                    )}

                                    {/* Contract Size */}
                                    <div>
                                        <div className="text-xs md:text-sm text-gray-400 mb-1">Contract Size</div>
                                        <div className="text-xs md:text-sm text-gray-300">
                                            {/* Base64 string is 4/3 the size of the binary data */}
                                            {Math.floor(contractData.contractCode.length * 0.75)} bytes
                                        </div>
                                    </div>

                                    {/* Creation Transaction */}
                                    <div>
                                        <div className="text-xs md:text-sm text-gray-400 mb-1">Creation Transaction</div>
                                        <div className="flex items-center space-x-2">
                                            <Link href={`/tx/${contractData.creationTransaction}`} className="text-xs md:text-sm text-accent hover:text-accent-hover">
                                                {contractData.creationTransaction}
                                            </Link>
                                            <CopyButton value={contractData.creationTransaction} label="Copy hash" />
                                        </div>
                                    </div>

                                    {/* Contract Status */}
                                    <div>
                                        <div className="text-xs md:text-sm text-gray-400 mb-1">Status</div>
                                        <div className="text-xs md:text-sm text-gray-300">
                                            {contractData.status === "0x1" ? "Success" : "Failed"}
                                        </div>
                                    </div>

                                    {/* Contract Code (verified source / ABI / bytecode) +
                                        Read / Write tabs. Replaces the old standalone bytecode
                                        block. ContractTabs handles both the unverified state
                                        (bytecode + Verify & Publish CTA) and the verified state. */}
                                    <ContractTabs contractData={contractData} />
                                </div>
                            </div>
                        </section>
                    )}
                </div>
            </section>

            {/* Unified activity tab bar. Replaces the old stacked
                HoldingsDisplay + Activity sections. Each panel inside
                lazy-loads its own data on first activation, persists the
                active tab in the URL (?tab=...), and keeps its pagination /
                filter state across tab switches. */}
            <AddressTabs
                // Force full remount on address change so the lazy-mount
                // accumulator + child fetch caches start fresh per holder.
                key={addressSegment}
                address={addressSegment}
                transactions={addressData.transactions_by_address || []}
                transactionsCount={
                    typeof addressData.transactions_count === 'number'
                        ? addressData.transactions_count
                        : (addressData.transactions_by_address?.length ?? 0)
                }
                internalt={addressData.internal_transactions_by_address || []}
            />
        </main>
    );
}
