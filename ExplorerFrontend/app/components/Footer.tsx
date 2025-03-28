'use client';

import React from 'react';
import Link from 'next/link';

export default function Footer() {
  const year = new Date().getFullYear();

  return (
    <footer className="bg-[#1a1a1a] text-gray-400 border-t border-[#2a2a2a] mt-16">
      <div className="max-w-7xl mx-auto px-6 py-10 flex flex-col md:flex-row justify-between gap-10">
        {/* Brand / About */}
        <div className="flex-1">
          <h2 className="text-xl font-bold text-[#ffa729] mb-4">ZondScan</h2>
          <p className="text-sm leading-relaxed">
            ZondScan is your gateway to the QRL Zond network — a secure, quantum-resistant,
            EVM-compatible blockchain. Explore blocks, transactions, smart contracts, and more with a developer-first interface.
          </p>
        </div>

        {/* Navigation */}
        <div className="flex-1 flex flex-col sm:flex-row gap-8">
          <div>
            <h3 className="text-sm font-semibold text-[#ffa729] uppercase mb-4">Explore</h3>
            <ul className="space-y-2 text-sm">
              <li><Link href="/blocks/1" className="hover:text-[#ffa729] transition">Blocks</Link></li>
              <li><Link href="/transactions/1" className="hover:text-[#ffa729] transition">Transactions</Link></li>
              <li><Link href="/contracts" className="hover:text-[#ffa729] transition">Contracts</Link></li>
              <li><Link href="/validators" className="hover:text-[#ffa729] transition">Validators</Link></li>
            </ul>
          </div>

          <div>
            <h3 className="text-sm font-semibold text-[#ffa729] uppercase mb-4">Tools</h3>
            <ul className="space-y-2 text-sm">
              <li><Link href="/checker" className="hover:text-[#ffa729] transition">Balance Checker</Link></li>
              <li><Link href="/converter" className="hover:text-[#ffa729] transition">Quanta ↔ Shor</Link></li>
            </ul>
          </div>
          <div>
            <h3 className="text-sm font-semibold text-[#ffa729] uppercase mb-4">Insights</h3>
            <ul className="space-y-2 text-sm">
              <li><Link href="/richlist" className="hover:text-[#ffa729] transition">Richlist</Link></li>
            </ul>
          </div>
          <div>
            <h3 className="text-sm font-semibold text-[#ffa729] uppercase mb-4">Resources</h3>
            <ul className="space-y-2 text-sm">
              <li><Link href="https://docs.theqrl.org" target="_blank" className="hover:text-[#ffa729] transition">QRL Docs</Link></li>
              <li><Link href="https://github.com/theQRL" target="_blank" className="hover:text-[#ffa729] transition">GitHub</Link></li>
            </ul>
          </div>
        </div>
      </div>

      <div className="border-t border-[#2a2a2a] py-6 px-6 text-sm text-gray-500 flex flex-col md:flex-row items-center justify-between">
        <span>&copy; {year} ZondScan. Built for the Quantum Resistant Ledger (QRL) Network.</span>
        <div className="mt-2 md:mt-0">
            <Link href="/sitemap.xml" className="hover:text-[#ffa729] transition">
            Sitemap
            </Link>
        </div>
        </div>
    </footer>
  );
}
