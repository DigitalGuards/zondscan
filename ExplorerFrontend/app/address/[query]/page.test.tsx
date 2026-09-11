import { describe, expect, it } from '@jest/globals';
import { renderToStaticMarkup } from 'react-dom/server';

import {
  AddressPageFailureView,
  generateAddressPageMetadata,
} from './address-page-presentation';

describe('QNS address route presentation', () => {
  const address = `Q${'01234567'.repeat(16)}`;

  it('keeps the normalized .qrl route as the canonical URL', () => {
    const metadata = generateAddressPageMetadata('MoscowChill.QRL');

    expect(metadata.title).toBe('moscowchill.qrl QNS Address | ZondScan');
    expect(metadata.alternates?.canonical).toBe(
      'https://zondscan.com/address/moscowchill.qrl',
    );
  });

  it('keeps the full address route while compacting direct-address metadata', () => {
    const metadata = generateAddressPageMetadata(address);

    expect(metadata.title).toBe(
      'Address Q01234567...45670123...01234567 | ZondScan',
    );
    expect(metadata.description).toContain(
      'Q01234567...45670123...01234567',
    );
    expect(metadata.alternates?.canonical).toBe(
      `https://zondscan.com/address/${address}`,
    );
  });

  it.each([
    [
      'qns-unconfigured',
      'QNS resolution is unavailable',
      'QNS resolution is not configured',
    ],
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
      />,
    );

    expect(html).toContain(`>${heading}</h1>`);
    expect(html).toContain('moscowchill.qrl');
    expect(html).toContain(`>${detail}</p>`);
  });
});
