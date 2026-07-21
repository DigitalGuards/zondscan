'use client';

import { useState } from 'react';
import CopyButton from './CopyButton';

interface CollapsibleHexProps {
    label: string;
    hex: string;
    copyLabel?: string;
}

// Generic expand/collapse viewer for large hex blobs (contract bytecode,
// validator public keys). Takes already-decoded hex, with or without a
// leading 0x, and renders/copies it 0x-prefixed to match the explorer's
// hex display convention. Extracted from ContractBytecode.
export default function CollapsibleHex({ label, hex, copyLabel = 'Copy hex' }: CollapsibleHexProps): JSX.Element | null {
    const [expanded, setExpanded] = useState(false);

    const normalized = hex.startsWith('0x') ? hex.slice(2) : hex;
    if (!normalized) return null;

    const prefixed = `0x${normalized}`;
    const byteLength = normalized.length / 2;

    return (
        <div>
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-2">
                <div className="text-xs md:text-sm text-text-secondary">
                    {label} <span className="text-text-muted">({byteLength.toLocaleString()} bytes)</span>
                </div>
                <div className="flex items-center gap-2">
                    <button
                        type="button"
                        onClick={() => setExpanded(e => !e)}
                        aria-expanded={expanded}
                        className="inline-flex items-center px-3 py-1.5 rounded-lg
                                   bg-card-gradient border border-border hover:border-accent
                                   text-sm text-text-secondary hover:text-accent transition-colors"
                    >
                        {expanded ? 'Collapse' : 'Expand'}
                    </button>
                    <CopyButton value={prefixed} label={copyLabel} />
                </div>
            </div>
            <div
                className={`rounded-lg bg-black/40 border border-border p-3 font-mono text-xs text-text-secondary
                            break-all overflow-y-auto transition-[max-height] duration-200
                            ${expanded ? 'max-h-[32rem]' : 'max-h-24'}`}
            >
                {prefixed}
            </div>
        </div>
    );
}
