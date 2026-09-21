import { describe, expect, it } from '@jest/globals';
import { renderToStaticMarkup } from 'react-dom/server';

import RichlistClient from './richlist-client';

const LOWER_BODY = 'a'.repeat(128);
const CHECKSUMMED_ADDRESS =
  'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';
const FINGERPRINT = 'QaaaAAaaa...AAaaaaAa...AaAaaaAA';

describe('RichlistClient address links', () => {
  it('keeps the complete canonical address in the href and renders its fingerprint', () => {
    const html = renderToStaticMarkup(
      <RichlistClient
        richlist={[
          {
            id: `0x${LOWER_BODY}`,
            balance: '0',
          },
        ]}
      />,
    );

    expect(html).toContain(`href="/address/${CHECKSUMMED_ADDRESS}"`);
    expect(html).toContain(`title="${CHECKSUMMED_ADDRESS}"`);
    expect(html).toContain(`>${FINGERPRINT}</span>`);
    expect(html).toContain(`<span class="sr-only">${CHECKSUMMED_ADDRESS}</span>`);
    expect(html).not.toContain(`>${CHECKSUMMED_ADDRESS}</a>`);
  });
});
