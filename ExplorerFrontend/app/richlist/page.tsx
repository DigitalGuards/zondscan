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
  const response = await fetch(config.handlerUrl + "/richlist", {
    cache: 'no-store'
  });
  const data = await response.json();
  return <RichlistClient richlist={data.richlist} />;
}
