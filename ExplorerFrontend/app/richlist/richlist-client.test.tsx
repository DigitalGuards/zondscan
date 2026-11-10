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

const ENTRY = { id: `0x${LOWER_BODY}`, balance: '0' };
const MOBILE_OPEN = '<div class="md:hidden"><div class="space-y-4">';
const DESKTOP_WRAPPER = '<div class="hidden md:block">';
const DESKTOP_OPEN = `${DESKTOP_WRAPPER}<div class="overflow-x-auto"><table`;

// A browser-like global for one render. The page used to read
// window.innerWidth while rendering, which is what made the browser's first
// render differ from the server's.
function renderWithWindowWidth(innerWidth: number): string {
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: {
      innerWidth,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    },
  });
  try {
    return renderToStaticMarkup(<RichlistClient richlist={[ENTRY]} />);
  } finally {
    Reflect.deleteProperty(globalThis, 'window');
  }
}

describe('RichlistClient layout', () => {
  it('puts the cards in the small-screen wrapper and the table in the wide-screen wrapper', () => {
    const html = renderToStaticMarkup(<RichlistClient richlist={[ENTRY]} />);

    expect(html).toContain(MOBILE_OPEN);
    expect(html).toContain(DESKTOP_OPEN);

    const [smallScreenPart, wideScreenPart] = html.split(DESKTOP_WRAPPER);
    expect(smallScreenPart).not.toContain('<table');
    expect(wideScreenPart).toContain('<table');
    // Each layout carries the row once.
    const link = `href="/address/${CHECKSUMMED_ADDRESS}"`;
    expect(smallScreenPart.split(link)).toHaveLength(2);
    expect(wideScreenPart.split(link)).toHaveLength(2);
  });

  it('renders the same markup on the server and in a browser of any width', () => {
    const server = renderToStaticMarkup(<RichlistClient richlist={[ENTRY]} />);

    expect(renderWithWindowWidth(1400)).toBe(server);
    expect(renderWithWindowWidth(390)).toBe(server);
  });
});
