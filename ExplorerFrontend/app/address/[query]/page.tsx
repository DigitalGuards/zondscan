import React from "react";
import { Metadata } from 'next';
import AddressClient from "./address-client";

interface PageProps {
    params: Promise<{ query: string }>;
    searchParams?: Promise<Record<string, string | string[]>>;
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
    const address = resolvedParams.query;

    return (
        <main>
            <h1 className="sr-only">Address {address}</h1>
            <AddressClient address={address} />
        </main>
    );
}
