'use client';

import { Popover, PopoverButton, PopoverPanel } from '@headlessui/react';
import { NATIVE_UNIT } from '../lib/helpers';
import { formatTransactionAmount } from '../lib/transactionAmount';
import CopyButton from './CopyButton';

export default function TransactionAmount({
  amount,
}: {
  amount: string | number | null | undefined;
}) {
  const value = formatTransactionAmount(amount);
  if (!value) return <span title="Amount unavailable">-</span>;

  const fullAmount = `${value.quanta} ${NATIVE_UNIT}`;

  return (
    <Popover as="span" className="inline-flex min-w-0">
      <PopoverButton
        className="inline-flex min-h-6 items-baseline gap-1 rounded-sm whitespace-nowrap tabular-nums text-xs sm:text-sm cursor-help hover:text-text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        title={`${fullAmount} (${value.planck} Planck)`}
        aria-label={`Show exact amount: ${fullAmount}`}
      >
        <span className="max-w-[10ch] sm:max-w-[14ch] truncate decoration-dotted underline underline-offset-4 decoration-text-muted/50">
          {value.compact}
        </span>
        <span className="text-text-muted text-xs">{NATIVE_UNIT}</span>
      </PopoverButton>
      <PopoverPanel
        anchor={{ to: 'bottom end', gap: 8, padding: 12 }}
        role="dialog"
        aria-label="Exact transaction amount"
        className="z-50 w-80 max-w-[calc(100vw-24px)] rounded-lg border border-border bg-background-secondary p-3 text-sm text-text-primary shadow-lg whitespace-normal"
      >
        <p className="mb-2 text-xs text-text-muted">Exact amount</p>
        <div className="flex items-start justify-between gap-3">
          <span className="min-w-0 break-all tabular-nums">{fullAmount}</span>
          <CopyButton text={value.quanta} label="Copy amount in Quanta" size="sm" />
        </div>
        <p className="mt-2 break-all text-xs text-text-muted tabular-nums">{value.planck} Planck</p>
      </PopoverPanel>
    </Popover>
  );
}
