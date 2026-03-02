'use client';

import TradingViewWidget from './TradingViewWidget';

export default function Charts(): JSX.Element {
  return (
    <div className="w-full mb-4">
      <div className="bg-[#1f1f1f] border border-[#3d3d3d] rounded-2xl hover:border-[#ffa729] transition-colors duration-300">
        <div className="p-4">
          <p className="text-[#ffa729] text-sm sm:text-xl font-bold mb-0">
            MEXC QRL/USDT Chart
          </p>
          <div className="h-[400px] mt-2">
            <TradingViewWidget />
          </div>
        </div>
      </div>
    </div>
  );
}
