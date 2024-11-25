import React from "react";
import axios from "axios";
import config from '../../../config';
import { headers } from "next/headers";
import CopyAddressButton from "../../components/CopyAddressButton";
import TanStackTable from "../../components/TanStackTable";
import BalanceDisplay from "./balance-display";
import ActivityDisplay from "./activity-display";
import type { AddressData } from "./types";

const getData = async (url: string | URL): Promise<AddressData | null> => {
    try {
        const path = new URL(url).pathname;
        if (path.endsWith('favicon.ico')) return null;
        if (!path.startsWith('/address/')) {
            console.error(`Invalid path: ${path}`);
            return null;
        }
        const response = await axios.get(`${config.handlerUrl}/address/aggregate${path.replace('/address/', '/')}`);
        return response.data;
    } catch (error) {
        console.error(`Error fetching data`);
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
    } else if (addressData.response !== null) {
        addressType = "Contract";
        addressIcon = (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
            </svg>
        );
    }

    return (
        <div className="py-8">
            <div className="relative overflow-hidden rounded-2xl 
                        bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                        border border-[#3d3d3d] shadow-xl mb-8">
                <div className="p-8">
                    {/* Header */}
                    <div className="flex items-center justify-between mb-8 pb-6 border-b border-gray-700">
                        <div className="flex items-center">
                            {addressIcon}
                            <div className="ml-4">
                                <div className="text-sm font-medium text-gray-400">{addressType}</div>
                                <div className="flex items-center mt-1">
                                    <div className="text-lg font-mono text-gray-300">{addressSegment}</div>
                                    <div className="ml-2">
                                        <CopyAddressButton address={addressSegment} />
                                    </div>
                                </div>
                            </div>
                        </div>
                        <div className="px-4 py-2 rounded-xl bg-[#3d3d3d] bg-opacity-20">
                            <span className="text-sm font-medium text-[#ffa729]">Rank #{rank}</span>
                        </div>
                    </div>

                    {/* Content Grid */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                        <BalanceDisplay balance={balance} />
                        <ActivityDisplay firstSeen={firstSeen} lastSeen={lastSeen} />
                    </div>
                </div>
            </div>

            {/* Transactions Table */}
            <div className="relative overflow-hidden rounded-2xl 
                        bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                        border border-[#3d3d3d] shadow-xl p-8">
                <h2 className="text-xl font-bold text-[#ffa729] mb-6">Transactions</h2>
                <TanStackTable 
                    transactions={addressData.transactions_by_address} 
                    internalt={addressData.internal_transactions_by_address} 
                />
            </div>
        </div>
    );
}
