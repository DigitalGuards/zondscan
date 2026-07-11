'use client';

import { useMemo, useState } from 'react';
import CopyButton from './CopyButton';
import { decodeToHex } from '../lib/helpers';

interface ContractBytecodeProps {
    contractCode: string;
}

export default function ContractBytecode({ contractCode }: ContractBytecodeProps): JSX.Element | null {
    const [expanded, setExpanded] = useState(false);

    const hex = useMemo(() => decodeToHex(contractCode), [contractCode]);

    if (!hex) return null;

    const prefixed = `0x${hex}`;
    const byteLength = hex.length / 2;

    return (
        <div>
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-2">
                <div className="text-xs md:text-sm text-text-secondary">
                    Contract Bytecode <span className="text-text-muted">({byteLength.toLocaleString()} bytes)</span>
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
                    <CopyButton value={prefixed} label="Copy bytecode" />
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
