import { resolveHandlerUrl } from './config';

test('browser uses the same-origin API even when a private backend is configured', () => {
  expect(
    resolveHandlerUrl({
      isBrowser: true,
      publicHandlerUrl: undefined,
      serverHandlerUrl: 'http://private-api:8081',
    })
  ).toBe('/api');
});
test('an explicit public API remains supported for live-data local previews', () => {
  expect(
    resolveHandlerUrl({
      isBrowser: true,
      publicHandlerUrl: 'https://zondscan.com/api',
      serverHandlerUrl: undefined,
    })
  ).toBe('https://zondscan.com/api');
});
test('server reads use their private deployment backend before any public override', () => {
  expect(
    resolveHandlerUrl({
      isBrowser: false,
      publicHandlerUrl: 'https://zondscan.com/api',
      serverHandlerUrl: 'http://private-api:8081',
    })
  ).toBe('http://private-api:8081');
});
test('relative browser proxy paths never become invalid server fetch URLs', () => {
  expect(
    resolveHandlerUrl({ isBrowser: false, publicHandlerUrl: '/api', serverHandlerUrl: undefined })
  ).toBe('http://127.0.0.1:8081');
});
