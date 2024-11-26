'use client';

import { useRouter } from 'next/navigation';
import { decodeBase64ToHexadecimal, formatAmount } from "../../lib/helpers";
import { SendIcon, ReceiveIcon } from './TransactionIcons';
import type { TransactionCardProps } from './types';

export default function TransactionCard({ transaction }: TransactionCardProps): JSX.Element {
  const router = useRouter();
  const isSending = transaction.InOut === 0;
  const date = new Date(transaction.TimeStamp * 1000).toLocaleString('en-GB', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });

  const txHash = "0x" + decodeBase64ToHexadecimal(transaction.TxHash);
  
  // Parse amount and handle potential string values
  const amount = typeof transaction.Amount === 'string' 
    ? parseFloat(transaction.Amount) 
    : transaction.Amount || 0;

  // Get formatted amount and unit
  const [formattedAmount, unit] = formatAmount(amount);

  const handleClick = (): void => {
    router.push(`/tx/${txHash}`);
  };

  return (
    <div 
      onClick={handleClick}
      className='relative overflow-hidden rounded-2xl 
                bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                border border-[#3d3d3d] shadow-xl
                hover:border-[#ffa729] transition-all duration-300
                group mb-4 cursor-pointer'
    >
      <div className="flex items-center p-6">
        {/* Left Section - Icon and Status */}
        <div className="flex flex-col items-center w-48">
          <div className="mb-2">
            {isSending ? <SendIcon /> : <ReceiveIcon />}
          </div>
          <div className="text-center">
            <p className="text-lg font-semibold text-[#ffa729] mb-1">Transfer</p>
            <p className="text-sm text-gray-400 mb-1">Confirmed</p>
            <p className="text-sm text-gray-400">{date}</p>
          </div>
        </div>

        {/* Middle Section - Hash */}
        <div className="flex-1 px-8 border-l border-r border-[#3d3d3d]">
          <p className="text-sm font-medium text-gray-400 mb-2">Transaction Hash</p>
          <p className="text-gray-300 hover:text-[#ffa729] transition-colors break-all font-mono">
            {txHash}
          </p>
        </div>

        {/* Right Section - Amount */}
        <div className="w-48 text-right">
          <p className="text-sm font-medium text-gray-400 mb-2">Amount</p>
          <p className="text-2xl font-semibold text-[#ffa729]">
            {formattedAmount}
            <span className="text-sm text-gray-400 ml-2">{unit}</span>
          </p>
        </div>
      </div>
    </div>
  );
}
