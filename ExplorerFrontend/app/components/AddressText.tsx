'use client';

import { displayAddress } from '../lib/preferences';
import { usePreferences } from './PreferencesProvider';

/**
 * Preference-aware address label. The default middle mode renders the QIP-55
 * fingerprint of a canonical 64-byte address. The complete value remains the
 * accessible name and browser tooltip, and malformed values stay visible.
 */
export default function AddressText({
  address,
  leading = 8,
  trailing = 6,
}: {
  address: string;
  leading?: number;
  trailing?: number;
}) {
  const { preferences } = usePreferences();
  const label = displayAddress(address, preferences.addressDisplay, leading, trailing);
  return (
    <span
      title={address}
      data-explorer-address={address.toLowerCase()}
      className="inline-block min-w-0 max-w-full break-words [overflow-wrap:anywhere]"
    >
      {label === address ? (
        address
      ) : (
        <>
          <span className="sr-only">{address}</span>
          <span aria-hidden="true">{label}</span>
        </>
      )}
    </span>
  );
}
