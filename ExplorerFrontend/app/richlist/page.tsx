import config from "../../config";
import RichlistClient from "./richlist-client";
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';


export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Rich List | QRL Zond Explorer',
  description:
    'Explore the top wallets by balance on the Quantum Resistant Ledger Proof-of-Stake network. Discover which addresses hold the most value in our rich list.',
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Rich List | QRL Zond Explorer',
    description:
      'Explore the top wallets by balance on the Quantum Resistant Ledger Proof-of-Stake network. Discover which addresses hold the most value in our rich list.',
    url: 'https://zondscan.com/richlist',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Rich List | QRL Zond Explorer',
    description:
      'Explore the top wallets by balance on the Quantum Resistant Ledger Proof-of-Stake network. Discover which addresses hold the most value in our rich list.',
  },
};


export default async function RichlistPage(): Promise<JSX.Element> {
  try {
    const response = await fetch(config.handlerUrl + "/richlist", { cache: 'no-store' });
    if (!response.ok) {
      return <div className="p-8 text-center text-red-400">Failed to load richlist data. Please try again later.</div>;
    }
    const data = await response.json();
    return <RichlistClient richlist={data.richlist ?? []} />;
  } catch {
    return <div className="p-8 text-center text-red-400">Unable to connect to the server. Please try again later.</div>;
  }
}
