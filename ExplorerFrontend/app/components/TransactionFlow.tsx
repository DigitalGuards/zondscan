import Link from 'next/link';
import type { TransactionDetails } from '../types';
import AddressFingerprint from './AddressFingerprint';
import CopyButton from './CopyButton';
import DetailRow from './DetailRow';
import InterfaceText from './InterfaceText';

interface FlowAddressProps {
  label: 'From' | 'To';
  address?: string;
  copyLabel: string;
  description?: string;
  placeholder: string;
}

// The full address from the `sm` breakpoint up, where a QIP-55 address fits a
// detail row; the fingerprint on phones, as the hash row truncates there.
function FlowAddress({ label, address, copyLabel, description, placeholder }: FlowAddressProps) {
  return (
    <DetailRow label={label} mono>
      {description && (
        <p className="mb-1 font-sans text-xs text-text-secondary">
          <InterfaceText text={description} />
        </p>
      )}
      {address ? (
        <div className="flex min-w-0 items-start gap-2">
          <Link
            href={`/address/${address}`}
            className="min-w-0 text-text-primary transition-colors hover:text-accent"
          >
            <span className="hidden break-all sm:inline" title={address}>
              {address}
            </span>
            <span className="sm:hidden">
              <AddressFingerprint address={address} />
            </span>
          </Link>
          <span className="shrink-0">
            <CopyButton value={address} label={copyLabel} size="sm" />
          </span>
        </div>
      ) : (
        <span className="font-sans text-text-muted">
          <InterfaceText text={placeholder} />
        </span>
      )}
    </DetailRow>
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
    // The last row of a group drops its own divider, so the section carries it.
    <section aria-label="Transaction flow" className="min-w-0 border-b border-border/70">
      <FlowAddress
        label="From"
        address={transaction.from}
        copyLabel="Copy sender address"
        placeholder="Sender unavailable"
      />
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
