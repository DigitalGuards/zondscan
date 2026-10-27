import { expect, test } from '@playwright/test';

test('gas and pending transaction data stay visible when timing is unknown', async ({ page, request }) => {
  await page.route('**/gas/summary', (route) => route.fulfill({ json: {
    avgGasPriceHex: '0x3b9aca00',
    avgGasUsedHex: '0x5208',
    avgGasLimitHex: '0x100000',
    avgBlockTimeSec: null,
    pendingCount: 7,
    lastBlockNumberHex: '0x2a',
    lastGasUsedHex: '0x5208',
    lastGasLimitHex: '0x100000',
    gasPriceHistogram: [],
  } }));
  await page.goto('/gas');
  const blockTime = page.locator('.card').filter({ has: page.getByText('Avg Block Time', { exact: true }) });
  await expect(blockTime.locator('p').nth(1)).toHaveText('-');
  const gasPrice = page.locator('.card').filter({ has: page.getByText('Avg Gas Price', { exact: true }) });
  await expect(gasPrice.locator('p').nth(1)).toHaveText('1.00 Shor');
  const mempool = page.locator('.card').filter({ has: page.getByText('Mempool Size', { exact: true }) });
  await expect(mempool.locator('p').nth(1)).toHaveText('7');

  const hash = `0x${(21).toString(16).padStart(64, '0')}`;
  const fixtureURL = new URL(`/__fixture/pending/${hash}?status=pending`, page.url());
  fixtureURL.port = '18091';
  const response = await request.post(fixtureURL.href);
  expect(response.status()).toBe(204);
  let etaSec: number | null = 60;
  await page.route('**/pending-tx-eta/*', (route) => route.fulfill({ json: {
    etaSec,
    avgBlockTimeSec: etaSec,
    avgGasUsedHex: '0x5208',
    pendingCount: 7,
    gasAheadHex: '0x0',
    medianGasPriceHex: '0x3b9aca00',
    yourGasPriceHex: '0x3b9aca00',
  } }));
  await page.goto(`/pending/tx/${hash}`);
  await expect(page.getByText(/Next inclusion ~/)).toBeVisible();
  etaSec = null;
  await expect(page.getByText(/Next inclusion ~/)).toHaveCount(0, { timeout: 12000 });
  await expect(page.getByText(/Any moment now/)).toHaveCount(0);
  await expect(page.getByText('Mempool: 7', { exact: true })).toBeVisible();
  await expect(page.getByText('Gas: at median', { exact: true })).toBeVisible();
  await expect(page.getByRole('region', { name: 'Transaction flow', exact: true })).toBeVisible();
});
