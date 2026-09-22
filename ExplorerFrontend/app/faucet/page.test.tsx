import { renderToStaticMarkup } from 'react-dom/server';
import FaucetPage, { metadata } from './page';

jest.mock('./faucet-client', () => () => null);

it('labels page and social metadata with the active v3 testnet', () => {
  expect(metadata.title).toBe('QRL Testnet v3 Faucet | ZondScan');
  expect(metadata.openGraph?.title).toBe(metadata.title);
  expect(metadata.twitter?.title).toBe(metadata.title);
  expect(renderToStaticMarkup(<FaucetPage />)).toContain('QRL Testnet v3 Faucet');
  expect(JSON.stringify(metadata)).not.toContain('QRL 2.0 testnet');
});
