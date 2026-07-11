import './globals.css'
import Sidebar from "./components/Sidebar"
import Script from 'next/script'
import Providers from './providers'
import type { Metadata } from 'next';
import { Chakra_Petch, IBM_Plex_Sans, IBM_Plex_Mono } from 'next/font/google';
import { sharedMetadata } from './lib/seo/metaData';
import Footer from './components/Footer';

const displayFont = Chakra_Petch({
  weight: ['500', '600', '700'],
  subsets: ['latin'],
  variable: '--font-zs-display',
  display: 'swap',
});

const sansFont = IBM_Plex_Sans({
  weight: ['400', '500', '600'],
  subsets: ['latin'],
  variable: '--font-zs-sans',
  display: 'swap',
});

const monoFont = IBM_Plex_Mono({
  weight: ['400', '500'],
  subsets: ['latin'],
  variable: '--font-zs-mono',
  display: 'swap',
});

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'ZondScan | QRL 2.0 Explorer',
  description:
    'Blockchain explorer for QRL 2.0, an EVM-compatible blockchain secured with post-quantum cryptography. Track transactions, smart contracts, blocks, and validators.',
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'ZondScan | QRL 2.0 Explorer',
    description:
      'Blockchain explorer for QRL 2.0. Track smart contracts, blocks, and transactions on a quantum-resistant EVM-compatible chain.',
    url: 'https://zondscan.com',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'ZondScan | QRL 2.0 Explorer',
    description:
      'Blockchain explorer for QRL 2.0. Track transactions, blocks, smart contracts, and validators on a post-quantum EVM chain.',
  },


};


export const viewport = {
  themeColor: '#0b0d13',
}

interface RootLayoutProps {
  children: React.ReactNode
}

export default function RootLayout({ children }: RootLayoutProps): JSX.Element {
  return (
    <html lang="en" className={`dark ${displayFont.variable} ${sansFont.variable} ${monoFont.variable}`}>
      <head>
        <Script id="schema-org" type="application/ld+json" strategy="beforeInteractive">
          {`
            {
              "@context": "https://schema.org",
              "@type": "WebApplication",
              "name": "QRL 2.0 Explorer",
              "description": "Blockchain explorer for QRL 2.0. Track transactions, blocks, smart contracts, and validators on the quantum-resistant proof-of-stake network.",
              "url": "https://zondscan.com",
              "applicationCategory": "Blockchain Explorer",
              "operatingSystem": "All",
              "browserRequirements": "Requires JavaScript",
              "offers": {
                "@type": "Offer",
                "price": "0",
                "priceCurrency": "USD"
              },
              "relatedApplication": {
                "@type": "SoftwareApplication",
                "name": "QRL 2.0 Web Wallet",
                "url": "https://qrlwallet.com",
                "applicationCategory": "Blockchain Wallet",
                "operatingSystem": "All"
              },
              "hasPart": [
                {
                  "@type": "WebPage",
                  "name": "Latest Transactions",
                  "description": "View recent Transactions",
                  "url": "https://zondscan.com/transactions/1"
                },
                {
                  "@type": "WebPage",
                  "name": "Pending Transactions",
                  "description": "View pending transactions",
                  "url": "https://zondscan.com/pending/1"
                },
                {
                  "@type": "WebPage",
                  "name": "Latest Blocks",
                  "description": "View all Blocks",
                  "url": "https://zondscan.com/blocks/1"
                },
                {
                  "@type": "WebPage",
                  "name": "Smart Contracts",
                  "description": "View QRL smart contracts",
                  "url": "https://zondscan.com/contracts"
                },
                {
                  "@type": "WebPage",
                  "name": "Validators",
                  "description": "Network Validators",
                  "url": "https://zondscan.com/validators"
                },
                {
                  "@type": "WebPage",
                  "name": "Balance Checker",
                  "description": "Check Account balance",
                  "url": "https://zondscan.com/checker"
                },
                {
                  "@type": "WebPage",
                  "name": "Unit Converter",
                  "description": "Convert QRL currencies",
                  "url": "https://zondscan.com/converter"
                },
                {
                  "@type": "WebPage",
                  "name": "Richlist",
                  "description": "Top QRL holders",
                  "url": "https://zondscan.com/richlist"
                }
              ]
            }
          `}
        </Script>
      </head>
      <body className="min-h-screen bg-background text-text-secondary">
        <Providers>
          <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:left-4 focus:z-[100] focus:px-4 focus:py-2 focus:bg-accent focus:text-background focus:rounded-lg focus:text-sm focus:font-medium">
            Skip to main content
          </a>
          <div className="flex min-h-screen">
            <Sidebar />
            {/* min-w-0 is load-bearing: without it this flex item refuses to
                shrink below its content's intrinsic width (flexbox min-width:
                auto), so any wide table or long unbroken hash stretches the
                whole page beyond the mobile viewport instead of scrolling
                inside its own overflow-x-auto container. */}
            <div className="flex-1 min-w-0 lg:ml-64 min-h-screen relative transition-all duration-300 mt-14 lg:mt-4 flex flex-col">
              <main id="main-content" className="flex-1 relative">
                {children}
              </main>
              <Footer />
            </div>
          </div>
        </Providers>
      </body>
    </html>
  )
}
