import React from "react";
import axios from "axios";
import { Metadata } from 'next';
import config from '../../../config';
import AddressView from "./address-view";
import type { AddressData } from "./types";

const decodeToHex = (input: string): string => {
    if (!input) return '0x0';
    const decoded = Buffer.from(input, 'base64');
    return decoded.toString('hex');
};

const getData = async (address: string): Promise<AddressData | null> => {
    try {
        console.log('Fetching address data:', address);
        const response = await axios.get(`${config.handlerUrl}/address/aggregate/${address}`);
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
        if (response.data.contract_code && response.data.contract_code.contractCode) {
            response.data.contract_code = {
                ...response.data.contract_code,
                decodedCreatorAddress: response.data.contract_code.contractCreatorAddress ? 
                    `0x${decodeToHex(response.data.contract_code.contractCreatorAddress)}` : '0x0',
                decodedContractAddress: response.data.contract_code.contractAddress ? 
                    `0x${decodeToHex(response.data.contract_code.contractAddress)}` : '0x0',
                contractSize: response.data.contract_code.contractCode ? 
                    Math.ceil(response.data.contract_code.contractCode.length * 3 / 4) : 0
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

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
    const resolvedParams = await params;
    const address = resolvedParams.query;
    
    return {
        title: `Address ${address} | ZondScan`,
        description: `View details for Zond address ${address}. See balance, transactions, and other blockchain data.`,
        openGraph: {
            title: `Address ${address} | ZondScan`,
            description: `View details for Zond address ${address}. See balance, transactions, and other blockchain data.`,
            url: `https://zondscan.com/address/${address}`,
            siteName: 'ZondScan',
            type: 'website',
        },
    };
}

export default async function Page({ params }: PageProps) {
    const resolvedParams = await params;
    const addressData = await getData(resolvedParams.query);

    if (!addressData) {
        return <div>Error loading address data</div>;
    }

    return (
        <main>
            <h1 className="sr-only">Address {resolvedParams.query}</h1>
            <AddressView addressData={addressData} addressSegment={resolvedParams.query} />
        </main>
    );
}
