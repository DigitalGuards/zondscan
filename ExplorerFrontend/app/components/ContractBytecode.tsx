'use client';

import { useMemo } from 'react';
import CollapsibleHex from './CollapsibleHex';
import { decodeToHex } from '../lib/helpers';

interface ContractBytecodeProps {
    contractCode: string;
}

// Thin wrapper: decodes the base64-encoded contract code from the API and
// hands the resulting hex to the shared CollapsibleHex viewer.
export default function ContractBytecode({ contractCode }: ContractBytecodeProps): JSX.Element | null {
    const hex = useMemo(() => decodeToHex(contractCode), [contractCode]);

    if (!hex) return null;

    return <CollapsibleHex label="Contract Bytecode" hex={hex} copyLabel="Copy bytecode" />;
}
