'use client';

import { formatAmount } from '../../lib/helpers';
import { STAKING_QUANTA } from '../../lib/constants';
import type { BalanceDisplayProps } from '@/app/types';
import Badge from '../../components/Badge';

export default function BalanceDisplay({ balance }: BalanceDisplayProps): JSX.Element {
  const [formattedBalance, unit] = formatAmount(balance);

  return (
    <div className="relative overflow-hidden rounded-lg md:rounded-xl 
                  bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                  border border-[#3d3d3d] p-3 md:p-4 lg:p-6">
      <h2 className="text-xs md:text-sm font-semibold text-gray-400 mb-2 md:mb-3 lg:mb-4">Balance</h2>
      <div className="flex items-baseline flex-wrap gap-1.5 md:gap-2">
        <span className="text-lg md:text-xl lg:text-2xl font-bold text-[#ffa729] break-all">{formattedBalance}</span>
        <span className="text-[10px] md:text-xs lg:text-sm text-gray-400">{unit}</span>
      </div>
      {balance > STAKING_QUANTA && (
        <div className="mt-2 md:mt-3 lg:mt-4">
          <Badge variant="success" dot>Qualified for Staking</Badge>
        </div>
      )}
    </div>
  );
}
