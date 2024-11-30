'use client';

import { toFixed } from '../../lib/helpers';
import { STAKING_QUANTA } from '../../lib/constants';
import type { BalanceDisplayProps } from './types';

export default function BalanceDisplay({ balance }: BalanceDisplayProps): JSX.Element {
  return (
    <div className="relative overflow-hidden rounded-xl 
                  bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                  border border-[#3d3d3d] p-4 md:p-6">
      <h2 className="text-sm font-semibold text-gray-400 mb-3 md:mb-4">Balance</h2>
      <div className="flex items-baseline">
        <span className="text-2xl md:text-3xl font-bold text-[#ffa729] break-all">{toFixed(balance)}</span>
        <span className="ml-2 text-xs md:text-sm text-gray-400">QUANTA</span>
      </div>
      {balance > STAKING_QUANTA && (
        <div className="mt-3 md:mt-4 px-2 md:px-3 py-1 bg-green-500/10 text-green-400 text-xs md:text-sm rounded-lg inline-block">
          Qualified for Staking
        </div>
      )}
    </div>
  );
}
