import { resolveHandlerUrl } from './config.js';

describe('resolveHandlerUrl', () => {
  test('keeps browser traffic on the same-origin API proxy by default', () => {
    expect(resolveHandlerUrl({ isBrowser: true })).toBe('/api');
  });

  test('uses an explicitly built browser API URL', () => {
    expect(
      resolveHandlerUrl({
        isBrowser: true,
        publicHandlerUrl: 'https://explorer.example/api',
        serverHandlerUrl: 'http://backend:8080',
      }),
    ).toBe('https://explorer.example/api');
  });

  test('uses the runtime-only backend URL for server components', () => {
    expect(
      resolveHandlerUrl({
        isBrowser: false,
        publicHandlerUrl: '/api',
        serverHandlerUrl: 'http://backend:8080',
      }),
    ).toBe('http://backend:8080');
  });

  test('does not send server fetches to a relative browser route', () => {
    expect(resolveHandlerUrl({ isBrowser: false, publicHandlerUrl: '/api' })).toBe(
      'http://127.0.0.1:8080',
    );
  });
});
