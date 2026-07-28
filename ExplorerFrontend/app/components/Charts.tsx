'use client';

import TradingViewWidget from './TradingViewWidget';

export default function Charts(): JSX.Element {
  return (
    <div className="w-full mb-4">
      <div className="card overflow-hidden">
        <div className="panel-header">
          <h2 className="text-[15px] font-display font-semibold text-text-primary">
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
