import { isNavigationActive, NAVIGATION_GROUPS } from './navigation';

it.each([
  ['/transactions/7', '/transactions/1'],
  ['/tx/0x123', '/transactions/1'],
  ['/block/23', '/blocks/1'],
  ['/epoch/7', '/epochs/1'],
  ['/validators/7', '/validators'],
  ['/learn/gas', '/learn'],
  ['/settings', '/settings'],
])('keeps %s in the %s navigation section', (path, link) => {
  expect(isNavigationActive(path, link)).toBe(true);
});
it('avoids prefix collisions and duplicate destinations', () => {
  expect(isNavigationActive('/blockade', '/blocks/1')).toBe(false);
  expect(isNavigationActive('/learned', '/learn')).toBe(false);
  const links = NAVIGATION_GROUPS.flatMap((group) => group.items.map((item) => item.href));
  expect(new Set(links).size).toBe(links.length);
});
