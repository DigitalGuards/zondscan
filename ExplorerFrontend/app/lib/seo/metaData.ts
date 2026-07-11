// lib/seo/metaData.ts
import { Metadata } from 'next';

// Shared social preview image. 1200x800 PNG in public/, well above the
// summary_large_image minimum (300x157) and close to the recommended
// ~1.91:1 aspect. metadataBase resolves the relative URL to zondscan.com.
const OG_IMAGE = {
  url: '/ZondScan_Logo.png',
  width: 1200,
  height: 800,
  alt: 'ZondScan, the QRL 2.0 blockchain explorer',
};

export const sharedMetadata: Partial<Metadata> = {
  metadataBase: new URL('https://zondscan.com'),
  applicationName: 'ZondScan',
  keywords:
    'QRL, QRL 2.0, Quantum Resistant Ledger, Zond, blockchain explorer, post-quantum cryptography, Proof of Stake, Web3, EVM, quantum resistant, cryptocurrency, smart contracts, validators, transactions, blocks',
  manifest: '/manifest.json',
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
    siteName: 'ZondScan',
    images: [OG_IMAGE],
  },
  twitter: {
    card: 'summary_large_image',
    creator: '@QRLedger',
    site: '@QRLedger',
    images: [OG_IMAGE.url],
  },
  authors: [{ name: 'DigitalGuards', url: 'https://digitalguards.nl' }],
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
    },
  },
};
