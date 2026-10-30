import { describe, expect, it } from '@jest/globals';
import { renderToStaticMarkup } from 'react-dom/server';

import { AddressPageFailureView, generateAddressPageMetadata } from './address-page-presentation';

describe('QNS address route presentation', () => {
  const address = `Q${'01234567'.repeat(16)}`;

  it('keeps the normalized .qrl route as the canonical URL', () => {
    const metadata = generateAddressPageMetadata('MoscowChill.QRL');

    expect(metadata.title).toBe('moscowchill.qrl QNS Address | ZondScan');
    expect(metadata.description).toContain('QRL address resolved by QNS name moscowchill.qrl');
    expect(metadata.alternates?.canonical).toMatch(/\/address\/moscowchill\.qrl$/);
  });

  it('keeps the full address route while compacting direct-address metadata', () => {
    const metadata = generateAddressPageMetadata(address);

    expect(metadata.title).toBe('Address Q01234567...45670123...01234567 | ZondScan');
    expect(metadata.description).toContain('Q01234567...45670123...01234567');
    expect(metadata.alternates?.canonical).toContain(`/address/${address}`);
  });

  it('uses the verified primary name in address metadata and preserves the address URL', () => {
    const original = generateAddressPageMetadata(address);
    const metadata = generateAddressPageMetadata(address, 'moscowchill.qrl');

    expect(metadata.title).toBe(
      'Address Q01234567...45670123...01234567 (moscowchill.qrl) | ZondScan'
    );
    expect(metadata.description).toContain(
      'QRL address with the verified QNS primary name moscowchill.qrl'
    );
    expect(metadata.description).not.toContain('resolved by QNS name');
    expect(metadata.openGraph?.title).toBe(metadata.title);
    expect(metadata.twitter?.title).toBe(metadata.title);
    expect(metadata.alternates?.canonical).toBe(original.alternates?.canonical);
  });

  it.each([undefined, 'alice.qrl'])(
    'presents the requested address first on aggregate failure with name=%s',
    (name) => {
      const html = renderToStaticMarkup(
        <AddressPageFailureView
          failure={{
            ok: false,
            kind: 'address-data',
            address,
            name,
            detail: 'Failed to load address data.',
          }}
        />
      );

      expect(html).toContain(`text-text-primary break-all">${address}</p>`);
      if (name) {
        expect(html).toContain(
          `text-text-secondary break-all">Verified QNS primary name: ${name}</p>`
        );
        expect(html.indexOf(address)).toBeLessThan(html.indexOf(name));
      }
    }
  );

  it.each([
    ['qns-unconfigured', 'QNS resolution is unavailable', 'QNS resolution is not configured'],
    ['qns-missing', 'QNS record not found', 'QNS name has no address record'],
    ['qns-upstream', 'QNS resolution failed', 'QNS resolution timed out.'],
  ] as const)('renders a specific %s state', (kind, heading, detail) => {
    const html = renderToStaticMarkup(
      <AddressPageFailureView
        failure={{
          ok: false,
          kind,
          name: 'moscowchill.qrl',
          detail,
        }}
      />
    );

    expect(html).toContain(`>${heading}</h1>`);
    expect(html).toContain('moscowchill.qrl');
    expect(html).toContain(`>${detail}</p>`);
  });
});
