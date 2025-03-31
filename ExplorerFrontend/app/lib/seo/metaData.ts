// lib/seo/metaData.ts
import { Metadata } from 'next';

export const sharedMetadata: Partial<Metadata> = {
  metadataBase: new URL('https://zondscan.com'),
  keywords:
    'QRL, Proof of Stake, ZOND, blockchain explorer, Web3, EVM, quantum resistant, cryptocurrency, blockchain, smart contracts, validators, transactions, blocks',
  alternates: {
    languages: {
      'en-US': 'https://zondscan.com',
    },
  },
  icons: {
    icon: [
      { url: '/favis/favicon.ico' },
      { url: '/favis/favicon-16x16.png', sizes: '16x16', type: 'image/png' },
      { url: '/favis/favicon-32x32.png', sizes: '32x32', type: 'image/png' },
      { url: '/favis/favicon-48x48.png', sizes: '48x48', type: 'image/png' },
    ],
    apple: [{ url: '/favis/apple-touch-icon.png' }],
    other: [{ rel: 'mask-icon', url: '/favis/safari-pinned-tab.svg' }],
  },
  openGraph: {
    type: 'website',
    locale: 'en_US',
    siteName: 'QRL Zond Explorer',
    images: [
      {
        url: '/QRL.png',
        width: 512,
        height: 512,
        alt: 'QRL Logo',
      },
    ],
  },
  twitter: {
    card: 'summary_large_image',
    creator: '@QRLedger',
    site: '@QRLedger',
    images: ['/QRL.png'],
  },
  authors: [{ name: 'DigitalGuards' }],
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
    },
  }, 
};
