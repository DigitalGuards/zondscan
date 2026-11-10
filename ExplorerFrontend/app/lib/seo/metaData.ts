// lib/seo/metaData.ts
import { BRAND, SITE_NAME, SITE_ORIGIN } from '../site';
import { Metadata } from 'next';

// Shared social preview image. Purpose-made 1200x630 brand card in public/,
// matching the recommended 1.91:1 aspect for summary_large_image.
// metadataBase resolves the relative URL to the site origin.
const OG_IMAGE = {
  url: '/og-image.png',
  width: 1200,
  height: 630,
  alt: `${SITE_NAME}, the QRL 2.0 blockchain explorer`,
};

export const sharedMetadata: Partial<Metadata> = {
  metadataBase: new URL(SITE_ORIGIN),
  applicationName: SITE_NAME,
  keywords:
    'QRL, QRL 2.0, Quantum Resistant Ledger, Zond, blockchain explorer, post-quantum cryptography, Proof of Stake, Web3, EVM, quantum resistant, cryptocurrency, smart contracts, validators, transactions, blocks',
  manifest: '/manifest.json',
  icons: {
    icon: [
      { url: BRAND.favicon },
      { url: BRAND.favicon16, sizes: '16x16', type: 'image/png' },
      { url: BRAND.favicon32, sizes: '32x32', type: 'image/png' },
      { url: BRAND.favicon48, sizes: '48x48', type: 'image/png' },
    ],
    apple: [{ url: BRAND.appleTouchIcon }],
    other: [{ rel: 'mask-icon', url: '/favis/safari-pinned-tab.svg' }],
  },
  openGraph: {
    type: 'website',
    locale: 'en_US',
    siteName: SITE_NAME,
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
