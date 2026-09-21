import { describe, expect, it } from '@jest/globals';
import { renderToStaticMarkup } from 'react-dom/server';

import ResolvedQnsIdentity, { resolvedQnsAddressPayloads } from './resolved-qns-identity';

const CHECKSUMMED_ADDRESS =
  'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';

describe('ResolvedQnsIdentity', () => {
  it('shows the normalized name while retaining the complete raw address payloads', () => {
    const html = renderToStaticMarkup(
      <ResolvedQnsIdentity name="moscowchill.qrl" address={CHECKSUMMED_ADDRESS} />,
    );

    expect(html).toContain('moscowchill.qrl');
    expect(html).toContain(`href="/address/${CHECKSUMMED_ADDRESS}"`);
    expect(html).toContain(`data-copy-address="${CHECKSUMMED_ADDRESS}"`);
    expect(html).toContain(`data-qr-address="${CHECKSUMMED_ADDRESS}"`);
    expect(html).toContain(`<span class="sr-only">${CHECKSUMMED_ADDRESS}</span>`);
    expect(html).toContain('QaaaAAaaa...AAaaaaAa...AaAaaaAA');
    expect(resolvedQnsAddressPayloads(CHECKSUMMED_ADDRESS)).toEqual({
      href: `/address/${CHECKSUMMED_ADDRESS}`,
      copyAddress: CHECKSUMMED_ADDRESS,
      qrAddress: CHECKSUMMED_ADDRESS,
    });
  });
});
