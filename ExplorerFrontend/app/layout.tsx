import './globals.css'
import Sidebar from "./components/Sidebar"
import Script from 'next/script'
import Providers from './providers'

export const viewport = {
  themeColor: '#1a1a1a',
}

export const metadata = {
  metadataBase: new URL('https://zondscan.com'),
  title: 'QRL Zond Explorer',
  description: 'QRL Zond Web3 EVM Compatible Blockchain Explorer - Explore transactions, blocks, smart contracts, and validators on the Quantum Resistant Ledger Proof-of-Stake network',
  keywords: 'QRL, Proof of Stake, ZOND, blockchain explorer, Web3, EVM, quantum resistant, cryptocurrency, blockchain, smart contracts, validators, transactions, blocks',
  alternates: {
    canonical: 'https://zondscan.com',
    languages: {
      'en-US': 'https://zondscan.com',
    },
    domains: [
      {
        domain: 'https://xmsscan.com',
        defaultLocale: 'en-US'
      },
      {
        domain: 'https://qrlvm.com',
        defaultLocale: 'en-US'
      }
    ]
  },
  icons: {
    icon: [
      { url: '/favis/favicon.ico' },
      { url: '/favis/favicon-16x16.png', sizes: '16x16', type: 'image/png' },
      { url: '/favis/favicon-32x32.png', sizes: '32x32', type: 'image/png' },
      { url: '/favis/favicon-48x48.png', sizes: '48x48', type: 'image/png' },
    ],
    apple: [
      { url: '/favis/apple-touch-icon.png' },
    ],
    other: [
      {
        rel: 'mask-icon',
        url: '/favis/safari-pinned-tab.svg',
      },
    ],
  },
  
  openGraph: {
    title: 'QRL Zond Explorer',
    description: 'QRL ZOND Web3/EVM Compatible Blockchain Explorer - Explore transactions, blocks, smart contracts, and validators on the Quantum Resistant Ledger Proof-of-Stake network',
    type: 'website',
    locale: 'en_US',
    url: 'https://zondscan.com',
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
    title: 'QRL Zond Explorer',
    description: 'QRL ZOND Web3/EVM Compatible Blockchain Explorer - Explore transactions, blocks, smart contracts, and validators on the Quantum Resistant Ledger Proof-of-Stake network',
    images: ['/QRL.png'],
    creator: '@QRLedger',
    site: '@QRLedger',
  },
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
    },
  },
}

interface RootLayoutProps {
  children: React.ReactNode
}

export default function RootLayout({ children }: RootLayoutProps) {
  return (
    <html lang="en" className="dark">
      <head>
        <Script id="schema-org" type="application/ld+json" strategy="beforeInteractive">
          {`
            {
              "@context": "https://schema.org",
              "@type": "WebApplication",
              "name": "QRL Zond Explorer",
              "description": "QRL Zond Web3 EVM Compatible Blockchain Explorer - Explore transactions, blocks, smart contracts, and validators on the Quantum Resistant Ledger Proof-of-Stake network",
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
                "name": "QRL Zond Web Wallet",
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
                  "description": "Explore QRL contracts",
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
      <body className="min-h-screen bg-[#1a1a1a] text-gray-300">
        <Providers>
          <div className="flex min-h-screen">
            <Sidebar />
            <main className="flex-1 lg:ml-64 min-h-screen relative transition-all duration-300 mt-[72px] lg:mt-4">
              <div className="relative">
                {children}
              </div>
            </main>
          </div>
        </Providers>
      </body>
    </html>
  )
}
