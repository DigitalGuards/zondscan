import React from "react";
import axios from "axios";
import config from '../../../config';
import { headers } from "next/headers";
import CopyAddressButton from "../../components/CopyAddressButton";
import TanStackTable from "../../components/TanStackTable";
import BalanceDisplay from "./balance-display";
import ActivityDisplay from "./activity-display";
import type { AddressData } from "./types";

const decodeToHex = (input: string): string => {
    const decoded = Buffer.from(input, 'base64');
    return decoded.toString('hex');
};

const getData = async (url: string | URL): Promise<AddressData | null> => {
    try {
        const path = new URL(url).pathname;
        if (path.endsWith('favicon.ico')) return null;
        if (!path.startsWith('/address/')) {
            console.error(`Invalid path: ${path}`);
            return null;
        }

        console.log('Fetching address data:', path);
        const response = await axios.get(`${config.handlerUrl}/address/aggregate${path.replace('/address/', '/')}`);
        console.log('Raw API response:', JSON.stringify(response.data, null, 2));

        // Process transactions to ensure gas values are in hex format
        if (response.data.transactions_by_address) {
            response.data.transactions_by_address = response.data.transactions_by_address.map((tx: any) => ({
                ...tx,
                gasUsedStr: tx.gasUsedStr || (tx.gasUsed ? `0x${tx.gasUsed.toString(16)}` : '0x0'),
                gasPriceStr: tx.gasPriceStr || (tx.gasPrice ? `0x${tx.gasPrice.toString(16)}` : '0x0')
            }));
        }

        // Decode contract addresses if present
        if (response.data.contract_code) {
            response.data.contract_code = {
                ...response.data.contract_code,
                decodedCreatorAddress: `0x${decodeToHex(response.data.contract_code.contractCreatorAddress)}`,
                decodedContractAddress: `0x${decodeToHex(response.data.contract_code.contractAddress)}`,
                contractSize: Math.ceil(response.data.contract_code.contractCode.length * 3 / 4)
            };
        }

        console.log('Processed transactions:', JSON.stringify(response.data.transactions_by_address, null, 2));
        return response.data;
    } catch (error) {
        console.error(`Error fetching data:`, error);
        return null;
    }
};

interface PageProps {
    params: Promise<{ query: string }>;
}

export default async function Address({ params }: PageProps): Promise<JSX.Element> {
    const headersList = await headers();
    const header_url = headersList.get('x-url') || "";
    const addressData = await getData(header_url);

    if (!addressData) {
        return <div>Error loading address data</div>;
    }

    const { balance } = addressData.address;
    const { rank } = addressData;

    let firstSeen = 0;
    let lastSeen = 0;
    if (addressData.transactions_by_address && Array.isArray(addressData.transactions_by_address) && addressData.transactions_by_address.length > 0) {
        const timestamps = addressData.transactions_by_address.map(tx => tx.TimeStamp);
        firstSeen = Math.min(...timestamps);
        lastSeen = Math.max(...timestamps);
    }

    const addressSegmentParts = new URL(header_url).pathname.split('/');
    const addressSegment = addressSegmentParts.length > 0 ? addressSegmentParts.pop()! : "";

    let addressType = "";
    let addressIcon = null;
    if (addressSegment.slice(0, 3) === "0x2") {
        addressType = "Dilithium Address";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
        );
    } else if (addressSegment.slice(0, 3) === "0x1") {
        addressType = "XMSS Address";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
        );
    } else if (addressData.contract_code && addressData.contract_code.contractCode) {
        addressType = "Contract";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
            </svg>
        );
    } else {
        addressType = "Address";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 9V7a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2m2 4h10a2 2 0 002-2v-6a2 2 0 00-2-2H9a2 2 0 00-2 2v6a2 2 0 002 2zm7-5a2 2 0 11-4 0 2 2 0 014 0z" />
            </svg>
        );
    }

    return (
        <div className="py-4 md:py-8 px-4 md:px-8">
            <div className="relative overflow-hidden rounded-2xl 
                        bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                        border border-[#3d3d3d] shadow-xl mb-6 md:mb-8">
                <div className="p-4 md:p-8">
                    {/* Header */}
                    <div className="flex flex-col md:flex-row md:items-center justify-between mb-6 md:mb-8 pb-4 md:pb-6 border-b border-gray-700">
                        <div className="flex items-start md:items-center mb-4 md:mb-0">
                            <div className="hidden md:block">{addressIcon}</div>
                            <div className="flex-1 md:ml-4">
                                <div className="flex items-center">
                                    <div className="block md:hidden mr-2">{addressIcon}</div>
                                    <div className="text-sm font-medium text-gray-400">{addressType}</div>
                                </div>
                                <div className="flex flex-col md:flex-row md:items-center mt-1">
                                    <div className="text-sm md:text-base font-mono text-gray-300 break-all md:break-normal">
                                        {addressSegment}
                                    </div>
                                    {addressSegment && (
                                        <div className="mt-2 md:mt-0 md:ml-2">
                                            <CopyAddressButton address={addressSegment} />
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                        <div className="px-3 py-1 md:px-4 md:py-2 rounded-xl bg-[#3d3d3d] bg-opacity-20 self-start md:self-center">
                            <span className="text-sm font-medium text-[#ffa729]">Rank #{rank}</span>
                        </div>
                    </div>

                    {/* Stats Grid */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 md:gap-6">
                        <BalanceDisplay balance={balance} />
                        <ActivityDisplay firstSeen={firstSeen} lastSeen={lastSeen} />
                    </div>

                    {/* Contract Information */}
                    {addressData.contract_code && addressData.contract_code.contractCode && (
                        <div className="mt-6">
                            <div className="rounded-xl bg-[#2d2d2d] border border-[#3d3d3d] p-4 md:p-6 space-y-4">
                                <h3 className="text-lg font-semibold text-[#ffa729]">
                                    {addressData.contract_code.isToken ? 'Token Contract' : 'Contract'} Information
                                </h3>
                                
                                <div className="space-y-3">
                                    {/* Creator Address */}
                                    <div>
                                        <div className="text-sm text-gray-400 mb-1">Creator Address</div>
                                        <div className="flex items-center space-x-2">
                                            <span className="text-sm font-mono text-gray-300 break-all">
                                                {addressData.contract_code.decodedCreatorAddress || 'Unknown'}
                                            </span>
                                            {addressData.contract_code.decodedCreatorAddress && (
                                                <CopyAddressButton address={addressData.contract_code.decodedCreatorAddress} />
                                            )}
                                        </div>
                                    </div>

                                    {/* Token Information */}
                                    {addressData.contract_code.isToken && (
                                        <>
                                            {/* Token Name */}
                                            <div>
                                                <div className="text-sm text-gray-400 mb-1">Token Name</div>
                                                <div className="text-sm text-gray-300">
                                                    {addressData.contract_code.tokenName || 'Unknown'}
                                                </div>
                                            </div>

                                            {/* Token Symbol */}
                                            <div>
                                                <div className="text-sm text-gray-400 mb-1">Token Symbol</div>
                                                <div className="text-sm text-gray-300">
                                                    {addressData.contract_code.tokenSymbol || 'Unknown'}
                                                </div>
                                            </div>

                                            {/* Token Decimals */}
                                            <div>
                                                <div className="text-sm text-gray-400 mb-1">Token Decimals</div>
                                                <div className="text-sm text-gray-300">
                                                    {addressData.contract_code.tokenDecimals || '0'}
                                                </div>
                                            </div>
                                        </>
                                    )}

                                    {/* Contract Size */}
                                    <div>
                                        <div className="text-sm text-gray-400 mb-1">Contract Size</div>
                                        <div className="text-sm text-gray-300">
                                            {addressData.contract_code.contractSize || 0} bytes
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {/* Transactions Section */}
            <div className="space-y-4">
                <h2 className="text-lg md:text-xl font-semibold text-[#ffa729]">Transactions</h2>
                <div className="overflow-hidden rounded-xl border border-[#3d3d3d]">
                    {addressData.transactions_by_address && addressData.transactions_by_address.length > 0 ? (
                        <TanStackTable 
                            transactions={addressData.transactions_by_address} 
                            internalt={addressData.internal_transactions_by_address || []}
                        />
                    ) : (
                        <div className="p-6 text-center text-gray-400">
                            No transactions found for this address
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
