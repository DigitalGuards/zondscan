import React from 'react';
import { decodeToHex, formatAddress } from '../../lib/helpers';
import CopyAddressButton from '../../components/CopyAddressButton';

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
    <div className="rounded-xl bg-[#2d2d2d] border border-[#3d3d3d] p-4 md:p-6 space-y-4">
      <h3 className="text-lg font-semibold text-[#ffa729]">Contract Information</h3>
      
      <div className="space-y-3">
        {/* Creator Address */}
        <div>
          <div className="text-sm text-gray-400 mb-1">Creator Address</div>
          <div className="flex items-center space-x-2">
            <span className="text-sm font-mono text-gray-300 break-all">{creatorAddress}</span>
            <CopyAddressButton address={creatorAddress} />
          </div>
        </div>

        {/* Contract Size */}
        <div>
          <div className="text-sm text-gray-400 mb-1">Contract Size</div>
          <div className="text-sm text-gray-300">
            {Math.ceil(contractCode.contractCode.length * 3 / 4)} bytes
          </div>
        </div>
      </div>
    </div>
  );
}
