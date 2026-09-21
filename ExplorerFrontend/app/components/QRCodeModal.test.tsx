import { describe, expect, it, jest } from '@jest/globals';
import { renderToStaticMarkup } from 'react-dom/server';

import QRCodeModal, { copyQrlAddress } from './QRCodeModal';

const LOWER_BODY = 'a'.repeat(128);
const CHECKSUMMED_ADDRESS =
  'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';

describe('QRCodeModal address payloads', () => {
  it('renders the complete canonical Q128 value into the QR and copy surface', () => {
    const html = renderToStaticMarkup(
      <QRCodeModal
        address={`0X${LOWER_BODY.toUpperCase()}`}
        isOpen={true}
        onClose={() => undefined}
      />,
    );

    expect(html).toContain(`data-qr-address="${CHECKSUMMED_ADDRESS}"`);
    expect(html).toContain(`<span class="sr-only">${CHECKSUMMED_ADDRESS}</span>`);
    expect(html).toContain('QaaaAAaaa...AAaaaaAa...AaAaaaAA');
    expect(html).toContain('aria-label="Copy full address"');
  });

  it('passes the complete canonical address to the clipboard writer', async () => {
    const writeText = jest.fn(async (_value: string) => undefined);

    await copyQrlAddress(CHECKSUMMED_ADDRESS, { writeText });

    expect(writeText).toHaveBeenCalledTimes(1);
    expect(writeText).toHaveBeenCalledWith(CHECKSUMMED_ADDRESS);
  });

  it('fails closed instead of encoding an invalid mixed-case address', () => {
    const html = renderToStaticMarkup(
      <QRCodeModal
        address={`Q${'Ab'.repeat(64)}`}
        isOpen={true}
        onClose={() => undefined}
      />,
    );

    expect(html).toBe('');
  });
});
