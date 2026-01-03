'use client';

import React, { useEffect, useState } from "react";
import axios from "axios";
import config from '../../../config';
import AddressView from "./address-view";
import type { AddressData } from "./types";
import { decodeToHex, formatAddress } from '../../lib/helpers';

interface AddressClientProps {
    address: string;
}

export default function AddressClient({ address }: AddressClientProps): JSX.Element {
    const [addressData, setAddressData] = useState<AddressData | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const fetchData = async (): Promise<void> => {
            try {
                setIsLoading(true);
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
                    const rawCreatorAddress = response.data.contract_code.contractCreatorAddress ? 
                        `0x${decodeToHex(response.data.contract_code.contractCreatorAddress)}` : '0x0';
                        
                    response.data.contract_code = {
                        ...response.data.contract_code,
                        decodedCreatorAddress: formatAddress(rawCreatorAddress),
                        decodedContractAddress: response.data.contract_code.contractAddress ? 
                            `0x${decodeToHex(response.data.contract_code.contractAddress)}` : '0x0',
                        contractSize: response.data.contract_code.contractCode ? 
                            Math.ceil(response.data.contract_code.contractCode.length * 3 / 4) : 0
                    };
                }

                console.log('Processed transactions:', JSON.stringify(response.data.transactions_by_address, null, 2));
                setAddressData(response.data);
                setError(null);
            } catch (error) {
                console.error(`Error fetching data:`, error);
                setError("Failed to load address data");
                setAddressData(null);
            } finally {
                setIsLoading(false);
            }
        };

        fetchData();
    }, [address]);

    if (isLoading) {
        return <div className="text-center p-8">Loading address data...</div>;
    }

    if (error || !addressData) {
        return <div className="text-center p-8 text-red-400">{error || "Error loading address data"}</div>;
    }

    return <AddressView addressData={addressData} addressSegment={address} />;
} 