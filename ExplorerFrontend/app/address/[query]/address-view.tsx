'use client';

import React, { useState, useEffect } from 'react';
import CopyAddressButton from "../../components/CopyAddressButton";
import QRCodeButton from "../../components/QRCodeButton";
import TanStackTable from "../../components/TanStackTable";
import BalanceDisplay from "./balance-display";
import ActivityDisplay from "./activity-display";
import type { AddressData } from "./types";
import Link from "next/link";

interface AddressViewProps {
    addressData: AddressData;
    addressSegment: string;
}

const AddressDisplay = ({ address, type }: { address: string, type: string }) => {
    const [isMobile, setIsMobile] = useState(false);

    useEffect(() => {
        const checkScreenSize = () => {
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
    } else if (addressSegment.slice(0, 3) === "0x2") {
        addressType = "Dilithium Address";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 md:h-6 md:w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
        );
    } else if (addressSegment.startsWith("Z")) {
        addressType = "Zond Address";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 md:h-6 md:w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
        );
    } else {
        addressType = "Address";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 md:h-6 md:w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 9V7a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2m2 4h10a2 2 0 002-2v-6a2 2 0 00-2-2H9a2 2 0 00-2 2v6a2 2 0 002 2zm7-5a2 2 0 11-4 0 2 2 0 014 0z" />
            </svg>
        );
    }

    return (
        <div className="py-3 md:py-6 lg:py-8 px-3 md:px-6 lg:px-8 max-w-[900px] mx-auto">
            <div className="relative overflow-hidden rounded-xl md:rounded-2xl 
                        bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                        border border-[#3d3d3d] shadow-lg md:shadow-xl mb-4 md:mb-6 lg:mb-8">
                <div className="p-3 md:p-6 lg:p-8">
                    {/* Header */}
                    <div className="flex flex-col lg:flex-row lg:items-center justify-between mb-4 md:mb-6 lg:mb-8 pb-3 md:pb-4 lg:pb-6 border-b border-gray-700">
                        <div className="flex items-start lg:items-center mb-3 lg:mb-0">
                            <div className="hidden lg:block">{addressIcon}</div>
                            <div className="flex-1 lg:ml-4">
                                <div className="flex items-center">
                                    <div className="block lg:hidden mr-2">{addressIcon}</div>
                                    <div className="text-xs md:text-sm font-medium text-gray-400">{addressType}</div>
                                </div>
                                <div className="flex flex-col lg:flex-row lg:items-center mt-1 gap-2">
                                    <AddressDisplay address={addressSegment} type={addressType} />
                                    {addressSegment && (
                                        <div className="flex items-center gap-2 mb-2 lg:mb-0 lg:ml-4">
                                            <CopyAddressButton address={addressSegment} />
                                            <QRCodeButton address={addressSegment} />
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                        <div className="px-2 md:px-3 lg:px-4 py-1 md:py-1.5 lg:py-2 rounded-lg md:rounded-xl bg-[#3d3d3d] bg-opacity-20 self-start lg:self-center">
                            <span className="text-xs md:text-sm font-medium text-[#ffa729]">Rank #{rank}</span>
                        </div>
                    </div>

                    {/* Stats Grid */}
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 md:gap-4 lg:gap-6">
                        <BalanceDisplay balance={balance} />
                        <ActivityDisplay firstSeen={firstSeen} lastSeen={lastSeen} />
                    </div>

                    {/* Contract Information */}
                    {contractData && contractData.contractCode && (
                        <div className="mt-4 md:mt-6">
                            <div className="rounded-xl bg-[#2d2d2d] border border-[#3d3d3d] p-3 md:p-4 lg:p-6 space-y-3 md:space-y-4">
                                <h3 className="text-base md:text-lg font-semibold text-[#ffa729]">
                                    {contractData.isToken ? 'Token Contract' : 'Contract'} Information
                                </h3>
                                
                                <div className="space-y-3">
                                    {/* Creator Address */}
                                    <div>
                                        <div className="text-xs md:text-sm text-gray-400 mb-1">Creator Address</div>
                                        <div className="flex items-center space-x-2">
                                            <AddressDisplay 
                                                address={contractData.contractCreatorAddress || 'Unknown'} 
                                                type="Creator"
                                            />
                                            {contractData.contractCreatorAddress && (
                                                <CopyAddressButton address={contractData.contractCreatorAddress} />
                                            )}
                                        </div>
                                    </div>

                                    {/* Token Information */}
                                    {contractData.isToken && (
                                        <>
                                            {/* Token Name */}
                                            <div>
                                                <div className="text-xs md:text-sm text-gray-400 mb-1">Token Name</div>
                                                <div className="text-xs md:text-sm text-gray-300">
                                                    {contractData.tokenName || 'Unknown'}
                                                </div>
                                            </div>

                                            {/* Token Symbol */}
                                            <div>
                                                <div className="text-xs md:text-sm text-gray-400 mb-1">Token Symbol</div>
                                                <div className="text-xs md:text-sm text-gray-300">
                                                    {contractData.tokenSymbol || 'Unknown'}
                                                </div>
                                            </div>

                                            {/* Token Decimals */}
                                            <div>
                                                <div className="text-xs md:text-sm text-gray-400 mb-1">Token Decimals</div>
                                                <div className="text-xs md:text-sm text-gray-300">
                                                    {contractData.tokenDecimals || '0'}
                                                </div>
                                            </div>
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
                                            <Link href={`/tx/${contractData.creationTransaction}`} className="text-xs md:text-sm text-[#ffa729] hover:text-[#ffb952]">
                                                {contractData.creationTransaction}
                                            </Link>
                                            <CopyAddressButton address={contractData.creationTransaction} />
                                        </div>
                                    </div>

                                    {/* Contract Status */}
                                    <div>
                                        <div className="text-xs md:text-sm text-gray-400 mb-1">Status</div>
                                        <div className="text-xs md:text-sm text-gray-300">
                                            {contractData.status === "0x1" ? "Success" : "Failed"}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {/* Transactions Section */}
            <div className="space-y-3 md:space-y-4">
                <h2 className="text-base md:text-lg lg:text-xl font-semibold text-[#ffa729]">Transactions</h2>
                <div className="overflow-hidden rounded-xl border border-[#3d3d3d]">
                    {addressData.transactions_by_address && addressData.transactions_by_address.length > 0 ? (
                        <TanStackTable 
                            transactions={addressData.transactions_by_address} 
                            internalt={addressData.internal_transactions_by_address || []}
                        />
                    ) : (
                        <div className="p-4 md:p-6 text-center text-gray-400">
                            No transactions found for this address
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
