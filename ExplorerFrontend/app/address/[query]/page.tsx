import type { Metadata } from 'next';
import AddressView from "./address-view";
import { sharedMetadata } from '../../lib/seo/metaData';
import type { AddressData } from "./types";
import { decodeToHex, formatAddress } from '../../lib/helpers';

interface PageProps {
    params: Promise<{ query: string }>;
    searchParams?: Promise<Record<string, string | string[]>>;
}

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
    const resolvedParams = await params;
    const address = resolvedParams.query;
    const canonicalUrl = `https://zondscan.com/address`;

    return {
        ...sharedMetadata,
        title: `Address ${address} | ZondScan`,
        description: `View details for Zond address ${address}. See balance, transactions, and other blockchain data.`,
        alternates: {
          ...sharedMetadata.alternates,
          canonical: canonicalUrl,
        },
        openGraph: {
          ...sharedMetadata.openGraph,
          title: `Address ${address} | ZondScan`,
          description: `View details for Zond address ${address}. See balance, transactions, and other blockchain data.`,
          url: canonicalUrl,
          siteName: 'ZondScan',
          type: 'website',
        },
        twitter: {
          ...sharedMetadata.twitter,
          title: `Address ${address} | ZondScan`,
          description: `View details for Zond address ${address}. See balance, transactions, and other blockchain data.`,
        },
      };
    }

async function fetchAddressData(address: string): Promise<AddressData | null> {
    try {
        const handlerUrl = process.env.HANDLER_URL || 'http://127.0.0.1:8080';
        const response = await fetch(`${handlerUrl}/address/aggregate/${address}`, {
            cache: 'no-store'
        });

        if (!response.ok) {
            throw new Error('Failed to fetch address data');
        }

        const data = await response.json();

        // Process transactions to ensure gas values are in hex format
        if (data.transactions_by_address) {
            data.transactions_by_address = data.transactions_by_address.map((tx: any) => ({
                ...tx,
                gasUsedStr: tx.gasUsedStr || (tx.gasUsed ? `0x${tx.gasUsed.toString(16)}` : '0x0'),
                gasPriceStr: tx.gasPriceStr || (tx.gasPrice ? `0x${tx.gasPrice.toString(16)}` : '0x0')
            }));
        }

        // Decode contract addresses if present
        if (data.contract_code && data.contract_code.contractCode) {
            const rawCreatorAddress = data.contract_code.contractCreatorAddress ?
                `0x${decodeToHex(data.contract_code.contractCreatorAddress)}` : '0x0';

            data.contract_code = {
                ...data.contract_code,
                decodedCreatorAddress: formatAddress(rawCreatorAddress),
                decodedContractAddress: data.contract_code.contractAddress ?
                    `0x${decodeToHex(data.contract_code.contractAddress)}` : '0x0',
                contractSize: data.contract_code.contractCode ?
                    Math.ceil(data.contract_code.contractCode.length * 3 / 4) : 0
            };
        }

        return data;
    } catch (error) {
        console.error('Error fetching address data:', error);
        return null;
    }
}

export default async function Page({ params }: PageProps): Promise<JSX.Element> {
    const resolvedParams = await params;
    const address = resolvedParams.query;
    const addressData = await fetchAddressData(address);

    if (!addressData) {
        return (
            <main>
                <div className="text-center p-8 text-red-400">Failed to load address data</div>
            </main>
        );
    }

    return (
        <main>
            <h1 className="sr-only">Address {address}</h1>
            <AddressView addressData={addressData} addressSegment={address} />
        </main>
    );
}
