'use client';

import TradingViewWidget from './TradingViewWidget';

export default function Charts(): JSX.Element {
  return (
    <div className="w-full mb-4">
      <div className="bg-[#1e1e1e] border border-[#2a2a2a] rounded-xl overflow-hidden">
        <div className="px-4 py-3 border-b border-[#2a2a2a]">
          <h2 className="text-sm font-semibold text-white">
            QRL/USDT Chart
          </h2>
        </div>
        <div className="p-4">
          <div className="h-[400px] mt-2">
            <TradingViewWidget />
          </div>
        </div>
      </div>
    </div>
  );
}
