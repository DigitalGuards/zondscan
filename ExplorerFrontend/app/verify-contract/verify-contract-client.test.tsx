import { renderToStaticMarkup } from 'react-dom/server';

import VerifyContractClient from './verify-contract-client';

jest.mock('axios');
jest.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(),
}));

it('offers the standard JSON loader, a primary source path, and via-IR', () => {
  const html = renderToStaticMarkup(<VerifyContractClient />);
  for (const text of [
    'Standard JSON input (optional)',
    'aria-label="Standard JSON input file"',
    'Primary source path (optional)',
    'placeholder="contracts/core/MyContract.hyp"',
    'Via IR',
    'Compile via IR',
    'Source code (primary file)',
  ]) {
    expect(html).toContain(text);
  }
});
