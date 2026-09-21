import { describe, expect, it } from '@jest/globals';
import { renderToStaticMarkup } from 'react-dom/server';

import AddressFingerprint from './AddressFingerprint';

const ADDRESS = `Q${'01234567'.repeat(16)}`;

describe('AddressFingerprint', () => {
  it('renders the first, middle, and final eight characters while exposing the full address', () => {
    const html = renderToStaticMarkup(<AddressFingerprint address={ADDRESS} />);

    expect(html).toContain('Q01234567...45670123...01234567');
    expect(html).toContain(`<span class="sr-only">${ADDRESS}</span>`);
    expect(html).toContain(`title="${ADDRESS}"`);
    expect(html).toContain('[overflow-wrap:anywhere]');
  });

  it('leaves malformed and legacy-width values visible', () => {
    const legacy = `Q${'a'.repeat(40)}`;
    const html = renderToStaticMarkup(<AddressFingerprint address={legacy} />);

    expect(html).toContain(`>${legacy}</span>`);
    expect(html).not.toContain('aria-hidden="true"');
  });
});
