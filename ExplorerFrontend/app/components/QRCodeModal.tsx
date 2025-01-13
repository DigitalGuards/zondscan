'use client';

import React from 'react';
import { QRCodeSVG } from 'qrcode.react';

interface QRCodeModalProps {
  address: string;
  isOpen: boolean;
  onClose: () => void;
}

export default function QRCodeModal({ address, isOpen, onClose }: QRCodeModalProps) {
  if (!isOpen) return null;
  
  // Generate the full zondscan URL
  const zondscanUrl = `https://zondscan.com/address/${address.toLowerCase()}`;
  
  // Format address for display (first 6 and last 4 chars)
  const displayAddress = `${address.slice(0, 6)}...${address.slice(-4)}`;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div 
        className="fixed inset-0 bg-black bg-opacity-50 transition-opacity"
        onClick={onClose}
      />
      
      {/* Modal */}
      <div className="relative bg-gradient-to-r from-[#2d2d2d] to-[#1f1f1f] rounded-xl p-6 max-w-[340px] w-full mx-4 shadow-2xl border border-[#3d3d3d]">
        <button
          onClick={onClose}
          className="absolute top-2 right-2 text-gray-400 hover:text-white"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
        
        <div className="text-center">
          <h3 className="text-lg font-medium text-[#ffa729] mb-4">Scan Address</h3>
          <div className="bg-white p-4 rounded-lg inline-block mb-4">
            <QRCodeSVG
              value={zondscanUrl}
              size={240}
              level="H"
              includeMargin={true}
            />
          </div>
          <div className="text-sm text-gray-300 mb-2">
            <span className="inline-block">{displayAddress}</span>
            <button 
              onClick={() => navigator.clipboard.writeText(address)}
              className="ml-2 text-[#ffa729] hover:text-[#ffb952] transition-colors"
              title="Copy full address"
            >
              <svg className="w-4 h-4 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
            </button>
          </div>
          <p className="text-xs text-gray-400">Scan to view on ZondScan</p>
        </div>
      </div>
    </div>
  );
}
