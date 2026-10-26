import { expect, test } from '@playwright/test';

const epoch = {
  epoch: '2',
  timestamp: 1700000960,
  status: 'finalized',
  validatorsCount: 64,
  activeCount: 64,
  pendingCount: 0,
  exitedCount: 0,
  slashedCount: 0,
  totalStaked: '64000000000000',
  startSlot: 16,
  endSlot: 24,
  finalizedEpoch: '4',
  justifiedEpoch: '5',
  headEpoch: '6',
  proposedCount: 2,
  missedCount: 6,
  blocks: [
    {
      slot: 16,
      status: 'proposed',
      proposerIndex: '0',
      proposerPubkey: '0xaabb',
      blockNumber: '9',
      blockHash: `0x${'a'.repeat(64)}`,
      timestamp: '1700000960',
      transactions: 1,
      gasUsed: '0x5208',
    },
    {
      slot: 17,
      status: 'missed',
      proposerIndex: '42',
      proposerPubkey: '0xccdd',
      transactions: 0,
    },
    {
      slot: 18,
      status: 'proposed',
      blockNumber: '10',
      blockHash: `0x${'b'.repeat(64)}`,
      timestamp: '1700001080',
      transactions: 0,
      gasUsed: '0x0',
    },
    ...[19, 20, 21, 22, 23].map((slot) => ({
      slot,
      status: 'missed',
      transactions: 0,
    })),
  ],
};

for (const theme of ['light', 'dark']) {
  for (const width of [390, 768, 1440]) {
    test(`epoch slot links at ${width}px in ${theme}`, async ({
      page,
    }, testInfo) => {
      const errors: string[] = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.setViewportSize({ width, height: 1000 });
      await page.addInitScript((theme) => {
        localStorage.setItem(
          'zondscan.preferences.v1',
          JSON.stringify({ theme }),
        );
      }, theme);
      await page.route('**/epoch/2', (route) => {
        if (route.request().resourceType() === 'document')
          return route.continue();
        return route.fulfill({ json: epoch });
      });
      await page.goto('/epoch/2');
      const rows = page.locator('tbody tr');
      await expect(rows).toHaveCount(8);
      await expect(
        rows.nth(0).getByRole('link', { name: '16', exact: true }),
      ).toHaveAttribute('href', '/block/9');
      await expect(rows.nth(1)).toContainText('Missed');
      await expect(rows.nth(1).locator('a[href^="/block/"]')).toHaveCount(0);
      await expect(rows.nth(0).locator('a[href="/validators/0"]')).toHaveText(
        '0',
      );
      await expect(rows.nth(1).locator('a[href="/validators/42"]')).toHaveText(
        '42',
      );
      await expect(rows.nth(2).locator('td').nth(4)).toHaveText('-');
      await expect(rows.nth(3).locator('td').nth(4)).toHaveText('-');
      if (width >= 640) {
        await expect(
          rows.nth(1).locator('a[href="/validators/42"]'),
        ).toBeVisible();
      }
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      expect(errors).toEqual([]);
      await page.screenshot({
        path: testInfo.outputPath(`epoch-${theme}-${width}.png`),
        fullPage: true,
      });
    });
  }
}

for (const width of [390, 1440]) {
  test(`unknown epoch summary at ${width}px fills on refresh`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    let available = false;
    await page.route('**/epoch/2', (route) => {
      if (route.request().resourceType() === 'document') return route.continue();
      return route.fulfill({ json: available ? epoch : {
        ...epoch, summaryAvailable: false,
        validatorsCount: null, activeCount: null, pendingCount: null,
        exitedCount: null, slashedCount: null, totalStaked: null,
      } });
    });
    await page.goto('/epoch/2');
    const summary = page.locator('.card-simple').first();
    await expect(summary).toContainText('Validators:');
    for (const label of ['Validators:', 'Active:', 'Pending:', 'Exited:', 'Slashed:', 'Total Staked:']) {
      await expect(summary.locator('div').filter({ has: page.locator('span', { hasText: new RegExp(`^${label}$`) }) }).last().locator('span').last()).toHaveText('-');
    }
    await page.screenshot({ path: testInfo.outputPath(`epoch-unknown-${width}.png`), fullPage: true });
    available = true;
    await page.reload();
    await expect(summary).toContainText('64');
    await expect(summary).toContainText('64,000 Quanta');
  });
}

test('homepage epochs and Learn guide use the network clock', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'Latest blocks' })).toContainText('epoch 5,000');
  await page.goto('/learn/validators-and-epochs');
  await expect(page.getByText('8 consecutive slots', { exact: true })).toBeVisible();
  const networkTiming = page.locator('p').filter({ hasText: "The explorer's network uses its own values:" });
  await expect(networkTiming.getByText('60 seconds per slot', { exact: true })).toBeVisible();
  await expect(page.getByText('128 consecutive slots', { exact: true })).toBeVisible();
  await page.goto('/epochs/1');
  await expect(page.locator('tbody')).toContainText('4,998');
  await expect(page.locator('tbody')).toContainText('2,560,000 Quanta');
});
