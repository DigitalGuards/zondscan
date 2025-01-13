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
  const [formattedAmount, unit] = formatAmount(transaction.Amount);

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
      <div className="flex flex-col md:flex-row items-center p-4 md:p-6 space-y-4 md:space-y-0">
        {/* Left Section - Icon and Status */}
        <div className="flex flex-col items-center w-full md:w-48">
          <div className="mb-2">
            {isSending ? <SendIcon /> : <ReceiveIcon />}
          </div>
          <div className="text-center">
            <p className="text-base md:text-lg font-semibold text-[#ffa729] mb-1">Transfer</p>
            <p className="text-xs md:text-sm text-gray-400 mb-1">Confirmed</p>
            <p className="text-xs md:text-sm text-gray-400">{date}</p>
          </div>
        </div>

        {/* Middle Section - Hash */}
        <div className="flex-1 px-4 md:px-8 py-4 md:py-0 w-full md:w-auto border-t md:border-t-0 md:border-l md:border-r border-[#3d3d3d]">
          <p className="text-xs md:text-sm font-medium text-gray-400 mb-2">Transaction Hash</p>
          <p className="text-sm text-gray-300 hover:text-[#ffa729] transition-colors break-all font-mono">
            {txHash}
          </p>
        </div>

        {/* Right Section - Amount */}
        <div className="w-full md:w-48 text-center md:text-right">
          <p className="text-xs md:text-sm font-medium text-gray-400 mb-2">Amount</p>
          <p className="text-xl md:text-2xl font-semibold text-[#ffa729]">
            {formattedAmount}
            <span className="text-xs md:text-sm text-gray-400 ml-2">{unit}</span>
          </p>
        </div>
      </div>
    </div>
  );
}
