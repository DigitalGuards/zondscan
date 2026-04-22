'use client';

import { useMemo, useState } from 'react';
import CopyButton from './CopyButton';

interface ContractBytecodeProps {
    contractCode: string;
}

function base64ToHex(b64: string): string {
    if (!b64) return '';
    try {
        const binary = atob(b64);
        let hex = '';
        for (let i = 0; i < binary.length; i++) {
            const byte = binary.charCodeAt(i).toString(16);
            hex += byte.length === 1 ? '0' + byte : byte;
        }
        return hex;
    } catch {
        return '';
    }
}

export default function ContractBytecode({ contractCode }: ContractBytecodeProps): JSX.Element | null {
    const [expanded, setExpanded] = useState(false);

    const hex = useMemo(() => base64ToHex(contractCode), [contractCode]);

    if (!hex) return null;

    const prefixed = `0x${hex}`;
    const byteLength = hex.length / 2;

    return (
        <div>
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-2">
                <div className="text-xs md:text-sm text-gray-400">
                    Contract Bytecode <span className="text-gray-500">({byteLength.toLocaleString()} bytes)</span>
                </div>
                <div className="flex items-center gap-2">
                    <button
                        type="button"
                        onClick={() => setExpanded(e => !e)}
                        aria-expanded={expanded}
                        className="inline-flex items-center px-3 py-1.5 rounded-lg
                                   bg-card-gradient border border-border hover:border-accent
                                   text-sm text-gray-300 hover:text-accent transition-colors"
                    >
                        {expanded ? 'Collapse' : 'Expand'}
                    </button>
                    <CopyButton value={prefixed} label="Copy bytecode" />
                </div>
            </div>
            <div
                className={`rounded-lg bg-black/40 border border-border p-3 font-mono text-xs text-gray-300
                            break-all overflow-y-auto transition-[max-height] duration-200
                            ${expanded ? 'max-h-[32rem]' : 'max-h-24'}`}
            >
                {prefixed}
            </div>
        </div>
    );
}
