'use client';

import { useMemo, useState } from 'react';
import Link from 'next/link';
import CopyButton from './CopyButton';
import ContractBytecode from './ContractBytecode';
import ReadContract from './ReadContract';
import WriteContract from './WriteContract';
import AiExplainCard from './AiExplainCard';
import type { ContractData } from '../types/address';
import { useUrlParam } from '../lib/use-url-param';

interface ContractTabsProps {
  contractData: ContractData;
}

const TAB_KEYS = ['code', 'read', 'write'] as const;
type TabKey = (typeof TAB_KEYS)[number];

// ?ctab values outside the known set (hand-edited URLs) fall back to the
// first tab.
function parseTab(raw: string): TabKey {
  return (TAB_KEYS as readonly string[]).includes(raw) ? (raw as TabKey) : 'code';
}

/**
 * Replacement for the old standalone ContractBytecode block on the address
 * page. Three tabs:
 *
 *  - Code , for verified contracts: source + compiler settings + ABI +
 *            bytecode (as a sub-section). For unverified: bytecode + a
 *            "Verify & Publish" CTA linking to /verify-contract.
 *  - Read , `view`/`pure` function dispatcher (M4).
 *  - Write, `nonpayable`/`payable` function dispatcher via the QRL Connect
 *            session (M5).
 */
export default function ContractTabs({ contractData }: ContractTabsProps): JSX.Element {
  // URL-backed (?ctab) so browser Back restores the selected sub-tab.
  // Distinct param from the page-level ?tab since both live on address
  // pages; replace keeps in-page tab clicks out of browser history.
  const [rawTab, setTab] = useUrlParam('ctab', 'code');
  const tab = parseTab(rawTab);

  const parsedAbi = useMemo(() => {
    if (!contractData.abi) return null;
    try {
      return JSON.parse(contractData.abi) as unknown;
    } catch {
      return null;
    }
  }, [contractData.abi]);

  return (
    <div>
      <div role="tablist" aria-label="Contract sections" className="flex gap-1 border-b border-border mb-3 md:mb-4">
        <TabButton active={tab === 'code'} onClick={() => setTab('code')}>Code</TabButton>
        <TabButton active={tab === 'read'} onClick={() => setTab('read')}>Read</TabButton>
        <TabButton active={tab === 'write'} onClick={() => setTab('write')}>Write</TabButton>
      </div>

      {tab === 'code' && <CodeTab contractData={contractData} parsedAbi={parsedAbi} />}
      {tab === 'read' && <ReadContract contractData={contractData} />}
      {tab === 'write' && <WriteContract contractData={contractData} />}
    </div>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      role="tab"
      aria-selected={active}
      onClick={onClick}
      type="button"
      className={`px-3 md:px-4 py-2 text-xs md:text-sm border-b-2 -mb-px transition-colors ${
        active
          ? 'border-accent text-accent'
          : 'border-transparent text-text-secondary hover:text-text-primary'
      }`}
    >
      {children}
    </button>
  );
}

function CodeTab({ contractData, parsedAbi }: { contractData: ContractData; parsedAbi: unknown | null }) {
  if (!contractData.verified) {
    return (
      <div className="space-y-3 md:space-y-4">
        <div className="rounded-lg border border-border bg-card-gradient p-3 md:p-4 flex flex-col sm:flex-row gap-2 sm:items-center sm:justify-between">
          <div>
            <div className="text-sm md:text-base font-medium text-text-primary">
              This contract isn&apos;t verified yet
            </div>
            <div className="text-xs md:text-sm text-text-secondary mt-0.5">
              Upload the source to publish the contract&apos;s ABI and source code, enabling read / write interaction.
            </div>
          </div>
          <Link
            href={`/verify-contract?address=${contractData.address}`}
            className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-accent text-background text-sm font-medium hover:bg-accent-hover transition-colors"
          >
            Verify &amp; Publish
          </Link>
        </div>
        <ContractBytecode contractCode={contractData.contractCode} />
      </div>
    );
  }

  return (
    <div className="space-y-3 md:space-y-4">
      <CompilerSettings contractData={contractData} />
      <AiExplainCard contractData={contractData} />
      <SourcePanel contractData={contractData} />
      <AbiPanel abi={parsedAbi} raw={contractData.abi ?? ''} />
      <ContractBytecode contractCode={contractData.contractCode} />
    </div>
  );
}

function CompilerSettings({ contractData }: { contractData: ContractData }) {
  const rows: Array<[string, React.ReactNode]> = [
    ['Contract name', contractData.contractName ?? '…'],
    ['Compiler', contractData.compilerVersion ?? '…'],
    ['Optimizer', contractData.optimizationEnabled ? `enabled (${contractData.optimizationRuns ?? 0} runs)` : 'disabled'],
    ['EVM version', contractData.evmVersion ?? '…'],
    ['License', contractData.license ?? '…'],
    ['Verified at', contractData.verifiedAt ?? '…'],
  ];
  return (
    <div className="rounded-lg border border-border bg-card-gradient p-3 md:p-4 grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-2 text-xs md:text-sm">
      {rows.map(([label, value]) => (
        <div key={label}>
          <div className="text-text-secondary">{label}</div>
          <div className="text-text-primary font-mono break-all">{value}</div>
        </div>
      ))}
    </div>
  );
}

function SourcePanel({ contractData }: { contractData: ContractData }) {
  const [expanded, setExpanded] = useState(true);
  if (!contractData.sourceCode) return null;
  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <div className="text-xs md:text-sm text-text-secondary">Contract source</div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setExpanded(e => !e)}
            aria-expanded={expanded}
            className="inline-flex items-center px-3 py-1.5 rounded-lg bg-card-gradient border border-border hover:border-accent text-sm text-text-secondary hover:text-accent transition-colors"
          >
            {expanded ? 'Collapse' : 'Expand'}
          </button>
          <CopyButton value={contractData.sourceCode} label="Copy source" />
        </div>
      </div>
      <pre
        className={`rounded-lg bg-black/40 border border-border p-3 font-mono text-xs text-text-secondary overflow-x-auto whitespace-pre transition-[max-height] duration-200 ${
          expanded ? 'max-h-[36rem] overflow-y-auto' : 'max-h-24 overflow-hidden'
        }`}
      >
        {contractData.sourceCode}
      </pre>
    </div>
  );
}

function AbiPanel({ abi, raw }: { abi: unknown | null; raw: string }) {
  const [expanded, setExpanded] = useState(false);
  // Hooks must be called unconditionally, the early return below cannot
  // precede useMemo. raw='' is a cheap input for JSON.stringify so the
  // computed value is harmless when the panel is about to bail.
  const pretty = useMemo(() => {
    if (abi === null) return raw;
    try {
      return JSON.stringify(abi, null, 2);
    } catch {
      return raw;
    }
  }, [abi, raw]);
  if (!raw) return null;

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <div className="text-xs md:text-sm text-text-secondary">Contract ABI</div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setExpanded(e => !e)}
            aria-expanded={expanded}
            className="inline-flex items-center px-3 py-1.5 rounded-lg bg-card-gradient border border-border hover:border-accent text-sm text-text-secondary hover:text-accent transition-colors"
          >
            {expanded ? 'Collapse' : 'Expand'}
          </button>
          <CopyButton value={raw} label="Copy ABI" />
        </div>
      </div>
      <pre
        className={`rounded-lg bg-black/40 border border-border p-3 font-mono text-xs text-text-secondary overflow-x-auto whitespace-pre transition-[max-height] duration-200 ${
          expanded ? 'max-h-[24rem] overflow-y-auto' : 'max-h-24 overflow-hidden'
        }`}
      >
        {pretty}
      </pre>
    </div>
  );
}

