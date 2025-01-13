'use client';

import React, { useState } from 'react';
import QRCodeModal from './QRCodeModal';

interface QRCodeButtonProps {
  address: string;
}

export default function QRCodeButton({ address }: QRCodeButtonProps) {
  const [isModalOpen, setIsModalOpen] = useState(false);

  return (
    <div className="inline-block">
      <button
        onClick={() => setIsModalOpen(true)}
        className="inline-flex items-center px-3 py-1.5 rounded-lg
                  bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                  border border-[#3d3d3d] hover:border-[#ffa729]
                  transition-all duration-300 group ml-2"
        title="Show QR Code"
      >
        <svg 
          className="h-4 w-4 mr-1.5 text-[#ffa729]" 
          fill="none" 
          stroke="currentColor" 
          viewBox="0 0 24 24"
        >
          <path 
            strokeLinecap="round" 
            strokeLinejoin="round" 
            strokeWidth={2} 
            d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z"
          />
        </svg>
        <span className="text-sm text-gray-300 group-hover:text-[#ffa729] transition-colors">
          QR Code
        </span>
      </button>

      <QRCodeModal
        address={address}
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
      />
    </div>
  );
}
