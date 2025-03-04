import React from "react";
import { Metadata } from 'next';
import AddressClient from "./address-client";
import { decodeToHex, formatAddress } from '../../lib/helpers';

interface PageProps {
    params: { query: string };
}

export async function generateMetadata({ params }: { params: { query: string } }): Promise<Metadata> {
    const address = params.query;
    
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
    const address = params.query;

    return (
        <main>
            <h1 className="sr-only">Address {address}</h1>
            <AddressClient address={address} />
        </main>
    );
}
