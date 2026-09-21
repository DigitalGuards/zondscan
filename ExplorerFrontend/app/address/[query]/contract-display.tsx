import React from 'react';
import { decodeToHex, formatAddress } from '../../lib/helpers';
import CopyButton from '../../components/CopyButton';
import AddressFingerprint from '../../components/AddressFingerprint';

interface ContractDisplayProps {
  contractCode: {
    contractCreatorAddress: string;
    contractAddress: string;
    contractCode: string;
  };
}

export default function ContractDisplay({ contractCode }: ContractDisplayProps): JSX.Element {
  // Decode base64 contract addresses to hex
  const rawCreatorAddress = `0x${decodeToHex(contractCode.contractCreatorAddress)}`;
  const creatorAddress = formatAddress(rawCreatorAddress);
  
  return (
    <div className="rounded-xl bg-surface-2 border border-border p-4 md:p-6 space-y-4">
      <h3 className="font-display text-lg font-semibold text-text-primary">Contract Information</h3>
      
      <div className="space-y-3">
        {/* Creator Address */}
        <div>
          <div className="text-sm text-text-secondary mb-1">Creator Address</div>
          <div className="flex items-center space-x-2">
            <AddressFingerprint
              address={creatorAddress}
              className="text-sm font-mono text-text-secondary"
            />
            <CopyButton value={creatorAddress} label="Copy address" />
          </div>
        </div>

        {/* Contract Size */}
        <div>
          <div className="text-sm text-text-secondary mb-1">Contract Size</div>
          <div className="text-sm text-text-secondary">
            {Math.ceil(contractCode.contractCode.length * 3 / 4)} bytes
          </div>
        </div>
      </div>
    </div>
  );
}
