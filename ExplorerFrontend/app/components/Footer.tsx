import Link from 'next/link';

export default function Footer(): JSX.Element {
  const year = new Date().getFullYear();

  return (
    <footer className="border-t border-border mt-16 bg-background-tertiary/40">
      <div className="page-content py-10 flex flex-col md:flex-row justify-between gap-10">
        {/* Brand / About */}
        <div className="flex-1">
          <h2 className="font-display text-xl font-semibold text-text-primary mb-1">
            Zond<span className="text-accent">Scan</span>
          </h2>
          <p className="eyebrow mb-4">QRL 2.0 Explorer</p>
          <p className="text-sm leading-relaxed text-text-secondary max-w-sm">
            ZondScan is your gateway to the QRL 2.0 network. Explore blocks, transactions, smart contracts, and more.
          </p>
        </div>

        {/* Navigation */}
        <nav aria-label="Footer navigation" className="flex-1 grid grid-cols-2 gap-x-8 gap-y-8 sm:flex sm:flex-row">
          <div>
            <h3 className="eyebrow mb-4">Explore</h3>
            <ul className="space-y-2 text-sm text-text-secondary">
              <li><Link href="/blocks/1" className="hover:text-accent transition-colors">Blocks</Link></li>
              <li><Link href="/transactions/1" className="hover:text-accent transition-colors">Transactions</Link></li>
              <li><Link href="/contracts" className="hover:text-accent transition-colors">Contracts</Link></li>
              <li><Link href="/validators" className="hover:text-accent transition-colors">Validators</Link></li>
            </ul>
          </div>

          <div>
            <h3 className="eyebrow mb-4">Tools</h3>
            <ul className="space-y-2 text-sm text-text-secondary">
              <li><Link href="/faucet" className="hover:text-accent transition-colors">Testnet Faucet</Link></li>
              <li><Link href="/checker" className="hover:text-accent transition-colors">Balance Checker</Link></li>
              <li><Link href="/converter" className="hover:text-accent transition-colors">Quanta ↔ Shor</Link></li>
            </ul>
          </div>
          <div>
            <h3 className="eyebrow mb-4">Insights</h3>
            <ul className="space-y-2 text-sm text-text-secondary">
              <li><Link href="/richlist" className="hover:text-accent transition-colors">Richlist</Link></li>
            </ul>
          </div>
          <div>
            <h3 className="eyebrow mb-4">Resources</h3>
            <ul className="space-y-2 text-sm text-text-secondary">
              <li><Link href="/learn" className="hover:text-accent transition-colors">Learn</Link></li>
              <li><Link href="/api-explorer" className="hover:text-accent transition-colors">API Docs</Link></li>
              <li><Link href="https://docs.theqrl.org" target="_blank" rel="noopener noreferrer" className="hover:text-accent transition-colors">QRL Docs</Link></li>
              <li><Link href="https://github.com/theQRL" target="_blank" rel="noopener noreferrer" className="hover:text-accent transition-colors">GitHub</Link></li>
              <li><Link href="https://myqrlwallet.com" target="_blank" rel="noopener noreferrer" className="hover:text-accent transition-colors">MyQRLWallet Ecosystem</Link></li>
            </ul>
          </div>
        </nav>
      </div>

      <div className="border-t border-border">
        <div className="page-content py-6 text-sm text-text-muted flex flex-col md:flex-row items-center justify-between gap-2">
          <span>&copy; {year} ZondScan. Built for the Quantum Resistant Ledger (QRL) Network.</span>
          <Link href="/sitemap.xml" className="hover:text-accent transition-colors">
            Sitemap
          </Link>
        </div>
      </div>
    </footer>
  );
}
