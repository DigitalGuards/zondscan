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


// Render on demand (don't prerender at build, which would require backend
// availability during `npm run build`). The fetch revalidate window below
// is what does the heavy lifting under traffic.
export const dynamic = 'force-dynamic';

export default async function RichlistPage(): Promise<JSX.Element> {
  // Richlist changes slowly (balance rankings), 30 s revalidate is plenty
  // fresh and keeps a single backend call serving N concurrent pageloads.
  const response = await fetch(config.handlerUrl + "/richlist", { next: { revalidate: 30 } });
  if (!response.ok) {
    throw new Error('Failed to load richlist data');
  }
  const data = await response.json();
  return <RichlistClient richlist={data.richlist ?? []} />;
}
