import { compactQrlAddress } from '../lib/qrlAddress';

interface AddressFingerprintProps {
  address: string;
  className?: string;
}

/**
 * Compact QIP-55 identity for passive UI. The complete value remains the
 * accessible name and browser tooltip, while malformed values stay visible.
 */
export default function AddressFingerprint({
  address,
  className = '',
}: AddressFingerprintProps): JSX.Element {
  const fingerprint = compactQrlAddress(address);
  const classes = `inline-block min-w-0 max-w-full break-words [overflow-wrap:anywhere] ${className}`.trim();

  if (fingerprint === address) {
    return (
      <span className={classes} title={address}>
        {address}
      </span>
    );
  }

  return (
    <span className={classes} title={address}>
      <span className="sr-only">{address}</span>
      <span aria-hidden="true">{fingerprint}</span>
    </span>
  );
}
