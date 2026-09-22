import Link from 'next/link';
import type { TransactionDetails } from '../types';
import AddressFingerprint from './AddressFingerprint';
import CopyButton from './CopyButton';
import InterfaceText from './InterfaceText';

interface FlowAddressProps {
  label: 'From' | 'To';
  address?: string;
  copyLabel: string;
  description?: string;
  placeholder: string;
}

function FlowAddress({ label, address, copyLabel, description, placeholder }: FlowAddressProps) {
  return (
    <div className="min-w-0 rounded-xl border border-border bg-surface-2/50 p-4">
      <p className="mb-2 text-xs font-medium text-text-muted">
        <InterfaceText text={label} />
      </p>
      {description && (
        <p className="mb-2 text-xs text-text-secondary">
          <InterfaceText text={description} />
        </p>
      )}
      {address ? (
        <div className="flex min-w-0 items-start gap-2">
          <Link
            href={`/address/${address}`}
            className="min-w-0 font-mono text-[13px] text-text-primary transition-colors hover:text-accent sm:text-sm"
          >
            <AddressFingerprint address={address} />
          </Link>
          <span className="shrink-0">
            <CopyButton value={address} label={copyLabel} size="sm" />
          </span>
        </div>
      ) : (
        <p className="text-sm text-text-muted">
          <InterfaceText text={placeholder} />
        </p>
      )}
    </div>
  );
}

type FlowTransaction = Pick<
  TransactionDetails,
  'from' | 'to' | 'contractCreated' | 'receiptStatus'
>;

export default function TransactionFlow({
  transaction,
  pending = false,
}: {
  transaction: FlowTransaction;
  pending?: boolean;
}) {
  const createdAddress = transaction.contractCreated?.address;
  const isCreation = transaction.to === '';
  const isReverted = transaction.receiptStatus === '0x0';
  const recipient = isCreation ? (isReverted ? undefined : createdAddress) : transaction.to;

  return (
    <section
      aria-label="Transaction flow"
      className="mb-3 grid min-w-0 grid-cols-1 gap-2 lg:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] lg:items-center lg:gap-3"
    >
      <FlowAddress
        label="From"
        address={transaction.from}
        copyLabel="Copy sender address"
        placeholder="Sender unavailable"
      />
      <svg
        aria-hidden="true"
        className="h-5 w-5 rotate-90 justify-self-center text-text-muted lg:rotate-0"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={1.5}
      >
        <path strokeLinecap="round" strokeLinejoin="round" d="M4 12h16m-6-6 6 6-6 6" />
      </svg>
      <FlowAddress
        label="To"
        address={recipient}
        copyLabel={isCreation ? 'Copy contract address' : 'Copy recipient address'}
        description={isCreation ? 'Contract creation' : undefined}
        placeholder={
          isCreation && pending
            ? 'Available after confirmation'
            : isCreation && isReverted
              ? 'No contract created'
              : 'Address unavailable'
        }
      />
    </section>
  );
}
