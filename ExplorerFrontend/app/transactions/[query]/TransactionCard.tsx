'use client';

import { useRouter } from 'next/navigation';
import { decodeBase64ToHexadecimal, formatAmount, truncateHash } from "../../lib/helpers";
import { SendIcon, ReceiveIcon } from './TransactionIcons';
import CopyHashButton from "../../components/CopyHashButton";
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
      className='relative overflow-hidden rounded-xl md:rounded-2xl 
                bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                border border-[#3d3d3d] shadow-lg md:shadow-xl
                hover:border-[#ffa729] transition-all duration-300
                group mb-3 md:mb-4 cursor-pointer'
    >
      <div className="md:flex md:flex-row items-center p-2.5 md:p-4 lg:p-5 md:space-y-0">
        {/* Mobile Layout */}
        <div className="flex md:hidden w-full items-center min-h-[40px] justify-between">
          <div className="flex items-center gap-2.5">
            <div className="flex-shrink-0 w-[24px] h-[24px]">
              <div className="w-full h-full text-gray-400">
                {isSending ? <SendIcon /> : <ReceiveIcon />}
              </div>
            </div>
            <div className="flex flex-col min-w-[130px]">
              <p className="text-[10px] leading-tight text-gray-500">{date}</p>
              <p className="text-[10px] leading-tight text-gray-500">Confirmed</p>
            </div>
          </div>
          <div className="flex flex-col items-end justify-center">
            <div className="flex items-center gap-1.5">
              <span className="text-[10px] text-gray-500">Amount:</span>
              <p className="text-sm font-semibold text-[#ffa729] leading-none">
              {formattedAmount}
              <span className="text-[10px] text-gray-400 ml-1">{unit}</span>
              </p>
            </div>
            <div className="flex items-center gap-1">
              <p className="text-[11px] text-gray-300 font-mono leading-none">{truncateHash(txHash, 8, 6)}</p>
              <CopyHashButton hash={txHash} size="small" />
            </div>
          </div>
        </div>

        {/* Desktop Layout */}
        <div className="hidden md:flex md:flex-row w-full items-center">
          <div className="flex items-center gap-2 lg:gap-3 w-[140px] lg:w-[160px] xl:w-[180px] flex-shrink-0">
            <div className="flex-shrink-0 w-[28px] lg:w-[28px]">
              <div className="w-full h-full text-gray-400">
                {isSending ? <SendIcon /> : <ReceiveIcon />}
              </div>
            </div>
            <div className="flex flex-col min-w-0">
              <div className="flex items-center gap-1.5 lg:gap-2">
                <p className="text-xs lg:text-sm font-medium text-[#ffa729]">Transfer</p>
                <p className="text-[10px] lg:text-xs text-gray-400">Confirmed</p>
              </div>
              <p className="text-[10px] lg:text-xs text-gray-500 truncate">{date}</p>
            </div>
          </div>

          <div className="flex-1 px-3 lg:px-4 min-w-0">
            <div className="flex items-center gap-1.5 lg:gap-2">
              <span className="text-[10px] lg:text-xs text-gray-400 flex-shrink-0">Hash:</span>
              <div className="flex items-center gap-2 min-w-0">
                <p className="text-xs lg:text-sm text-gray-300 hover:text-[#ffa729] transition-colors font-mono truncate">
                  {truncateHash(txHash, 12, 8)}
                </p>
                <div className="flex-shrink-0">
                  <CopyHashButton hash={txHash} />
                </div>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-1.5 lg:gap-2 w-[120px] lg:w-[140px] xl:w-[160px] flex-shrink-0 justify-end">
            <span className="text-[10px] lg:text-xs text-gray-400">Amount:</span>
            <p className="text-sm lg:text-base font-semibold text-[#ffa729]">
              {formattedAmount}
              <span className="text-[10px] lg:text-xs text-gray-400 ml-1">{unit}</span>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
