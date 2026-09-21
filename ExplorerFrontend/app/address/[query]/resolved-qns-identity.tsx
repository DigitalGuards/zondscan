import Link from 'next/link';
import AddressFingerprint from '../../components/AddressFingerprint';
import CopyButton from '../../components/CopyButton';
import QRCodeButton from '../../components/QRCodeButton';

interface ResolvedQnsIdentityProps {
  name: string;
  address: string;
}

export function resolvedQnsAddressPayloads(address: string) {
  return {
    href: `/address/${address}`,
    copyAddress: address,
    qrAddress: address,
  };
}

/**
 * Keep the friendly QNS identity visible while every actionable address
 * surface carries the complete canonical QIP-55 value.
 */
export default function ResolvedQnsIdentity({
  name,
  address,
}: ResolvedQnsIdentityProps): JSX.Element {
  const payloads = resolvedQnsAddressPayloads(address);
  return (
    <div className="mt-1 space-y-1" data-qns-resolution={name} data-qns-address={address}>
      <div className="flex flex-wrap items-baseline gap-x-2">
        <span className="text-xs text-text-secondary">QNS Name</span>
        <span className="font-medium text-text-primary break-all">{name}</span>
      </div>
      <div className="flex flex-col lg:flex-row lg:items-center gap-2">
        <span className="text-xs text-text-secondary shrink-0">Resolves to</span>
        <Link
          href={payloads.href}
          className="font-mono text-sm lg:text-base text-accent hover:text-accent-hover min-w-0 max-w-full"
          data-qns-address-link={address}
        >
          <AddressFingerprint address={address} />
        </Link>
        <div
          className="flex items-center gap-2 shrink-0"
          data-copy-address={payloads.copyAddress}
          data-qr-address={payloads.qrAddress}
        >
          <CopyButton value={payloads.copyAddress} label="Copy resolved address" />
          <QRCodeButton address={payloads.qrAddress} />
        </div>
      </div>
    </div>
  );
}
