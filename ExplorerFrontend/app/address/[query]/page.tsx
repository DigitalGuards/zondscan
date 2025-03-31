import React from "react";
import { Metadata } from 'next';
import AddressClient from "./address-client";
import { sharedMetadata } from '../../lib/seo/metaData';

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
