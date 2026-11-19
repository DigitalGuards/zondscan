import { browserGcTime } from './queryCache';

const scope: { window?: unknown } = globalThis;

afterEach(() => {
  delete scope.window;
});

it('starts no cache timer on the server', () => {
  expect(browserGcTime(5 * 60 * 1000)).toBe(Infinity);
});

it('keeps the given lifetime in the browser', () => {
  scope.window = {};

  expect(browserGcTime(5 * 60 * 1000)).toBe(300_000);
});
