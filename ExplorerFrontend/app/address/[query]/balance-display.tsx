'use client';

import { formatAmount } from '../../lib/helpers';
import { STAKING_QUANTA } from '../../lib/constants';
import type { BalanceDisplayProps } from './types';

export default function BalanceDisplay({ balance }: BalanceDisplayProps): JSX.Element {
  const [formattedBalance, unit] = formatAmount(balance);

  return (
    <div className="relative overflow-hidden rounded-xl 
                  bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                  border border-[#3d3d3d] p-4 lg:p-6">
      <h2 className="text-sm font-semibold text-gray-400 mb-3 lg:mb-4">Balance</h2>
      <div className="flex items-baseline flex-wrap gap-2">
        <span className="text-xl sm:text-2xl lg:text-3xl font-bold text-[#ffa729] break-all">{formattedBalance}</span>
        <span className="text-xs sm:text-sm text-gray-400">{unit}</span>
      </div>
      {balance > STAKING_QUANTA && (
        <div className="mt-3 lg:mt-4 px-2 lg:px-3 py-1 bg-green-500/10 text-green-400 text-xs sm:text-sm rounded-lg inline-block">
          Qualified for Staking
        </div>
      )}
    </div>
  );
}
