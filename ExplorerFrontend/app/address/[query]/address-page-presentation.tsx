import type { Metadata } from 'next';
import type { AddressPageLoadResult } from './address-page-data';
import { sharedMetadata } from '../../lib/seo/metaData';
import { canonicalizeQrlAddress, compactQrlAddress } from '../../lib/qrlAddress';
import { normalizeQnsName } from '../../lib/qns';

export function generateAddressPageMetadata(query: string): Metadata {
  const qnsName = normalizeQnsName(query);
  const address = canonicalizeQrlAddress(query);
  const identity = qnsName ?? address ?? query;
  const displayIdentity = address ? compactQrlAddress(address) : identity;
  const canonicalUrl = `https://zondscan.com/address/${encodeURIComponent(identity)}`;
  const title = qnsName
    ? `${qnsName} QNS Address | ZondScan`
    : `Address ${displayIdentity} | ZondScan`;
  const description = qnsName
    ? `View the QRL address resolved by QNS name ${qnsName}, including balance, transactions, and other blockchain data.`
    : `View details for QRL address ${displayIdentity}. See balance, transactions, and other blockchain data.`;

  return {
    ...sharedMetadata,
    title,
    description,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: canonicalUrl,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title,
      description,
      url: canonicalUrl,
      siteName: 'ZondScan',
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title,
      description,
    },
  };
}

type AddressPageFailure = Extract<AddressPageLoadResult, { ok: false }>;

export function addressPageFailureHeading(kind: AddressPageFailure['kind']): string {
  switch (kind) {
    case 'qns-invalid':
      return 'Invalid QNS name';
    case 'qns-unconfigured':
      return 'QNS resolution is unavailable';
    case 'qns-missing':
      return 'QNS record not found';
    case 'qns-upstream':
      return 'QNS resolution failed';
    case 'invalid-query':
      return 'Invalid address or QNS name';
    case 'address-data':
      return 'Address data unavailable';
  }
}

export function AddressPageFailureView({ failure }: { failure: AddressPageFailure }): JSX.Element {
  return (
    <main className="detail-content">
      <section className="card p-6 md:p-8 text-center" aria-labelledby="address-error-heading">
        <h1 id="address-error-heading" className="text-xl font-semibold text-error">
          {addressPageFailureHeading(failure.kind)}
        </h1>
        {failure.name && (
          <p className="mt-3 font-mono text-sm text-text-primary break-all">{failure.name}</p>
        )}
        <p className="mt-3 text-sm text-text-secondary">{failure.detail}</p>
      </section>
    </main>
  );
}
