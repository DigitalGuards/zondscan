import { expect, test, type Page } from '@playwright/test';

const hash = (value: number) => `0x${value.toString(16).padStart(64, '0')}`;
const sender = `Q${'a'.repeat(128)}`;
const recipient = `Q${'b'.repeat(128)}`;
const createdContract = `Q${'c'.repeat(128)}`;
const pageErrors = new WeakMap<Page, string[]>();

test.beforeEach(async ({ page, baseURL }) => {
  expect(['127.0.0.1', 'localhost', '[::1]']).toContain(new URL(baseURL!).hostname);
  const errors: string[] = [];
  pageErrors.set(page, errors);
  page.on('pageerror', (error) => errors.push(error.message));
  // These cases use only the local fixture API and never reach external services.
  await page.route('**/*', (route) => {
    const url = new URL(route.request().url());
    return ['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname)
      ? route.continue()
      : route.abort('blockedbyclient');
  });
  await page.addInitScript(() => {
    const copied: string[] = [];
    Object.defineProperty(window, '__transactionFlowCopies', { value: copied });
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: async (value: string) => {
          copied.push(value);
        },
      },
    });
  });
});

test.afterEach(async ({ page }) => {
  expect(pageErrors.get(page)).toEqual([]);
});

async function copiedValues(page: Page) {
  return page.evaluate(
    () => (window as unknown as { __transactionFlowCopies: string[] }).__transactionFlowCopies
  );
}

async function expectNoOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const flow = page.getByRole('region', { name: 'Transaction flow', exact: true });
  expect(await flow.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
}

async function expectKind(page: Page, kind: string) {
  const header = page
    .getByRole('heading', { name: 'Transaction Details', exact: true })
    .locator('../..');
  await expect(header.getByText(kind, { exact: true })).toBeVisible();
}

for (const width of [390, 1280]) {
  test(`native transaction flow preserves full addresses at ${width}px`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/tx/${hash(11)}`);
    await expect(
      page.getByRole('heading', { name: 'Transaction Details', exact: true })
    ).toBeVisible();
    await expectKind(page, 'Transfer');
    await expect(page.getByText('Confirmed', { exact: true })).toBeVisible();
    const flow = page.getByRole('region', { name: 'Transaction flow', exact: true });
    await expect(flow.getByText('From', { exact: true })).toBeVisible();
    await expect(flow.getByText('To', { exact: true })).toBeVisible();
    await expect(flow.getByText(/^(IN|OUT)$/)).toHaveCount(0);

    const from = flow.getByRole('link', { name: sender, exact: true });
    const to = flow.getByRole('link', { name: recipient, exact: true });
    await expect(from).toHaveAttribute('href', `/address/${sender}`);
    await expect(to).toHaveAttribute('href', `/address/${recipient}`);
    await flow.getByRole('button', { name: 'Copy sender address', exact: true }).click();
    await flow.getByRole('button', { name: 'Copy recipient address', exact: true }).click();
    await expect.poll(() => copiedValues(page)).toEqual([sender, recipient]);

    const fromBox = await from.boundingBox();
    const toBox = await to.boundingBox();
    expect(fromBox).not.toBeNull();
    expect(toBox).not.toBeNull();
    if (width < 1024) {
      expect(toBox!.y).toBeGreaterThan(fromBox!.y + fromBox!.height);
      for (const link of [from, to]) {
        const dimensions = await link.locator('[aria-hidden="true"]').evaluate((element) => ({
          height: element.getBoundingClientRect().height,
          lineHeight: Number.parseFloat(getComputedStyle(element).lineHeight),
        }));
        expect(Number.isFinite(dimensions.lineHeight)).toBe(true);
        expect(dimensions.height).toBeLessThanOrEqual(dimensions.lineHeight + 1);
      }
    } else {
      expect(Math.abs(fromBox!.y - toBox!.y)).toBeLessThanOrEqual(2);
      expect(toBox!.x).toBeGreaterThan(fromBox!.x + fromBox!.width);
    }
    await expectNoOverflow(page);
    // The release runner sets --output to its private evidence directory.
    await page.screenshot({
      path: testInfo.outputPath(`transaction-flow-${width}.png`),
      fullPage: true,
    });
    await page.getByRole('button', { name: /^Appearance:/ }).click();
    await page.getByRole('menuitem', { name: /^Light/ }).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await expectKind(page, 'Transfer');
    await expectNoOverflow(page);
    await page.screenshot({
      path: testInfo.outputPath(`transaction-flow-${width}-light.png`),
      fullPage: true,
    });
    await to.click();
    await expect(page).toHaveURL(new RegExp(`/address/${recipient}$`));
  });

  for (const fixture of [
    { id: 1, kind: 'Contract call', target: recipient, copy: 'Copy recipient address' },
    { id: 12, kind: 'Contract creation', target: createdContract, copy: 'Copy contract address' },
    { id: 13, kind: 'Transaction', target: recipient, copy: 'Copy recipient address' },
  ]) {
    test(`${fixture.kind} type and recipient are distinct from status at ${width}px`, async ({
      page,
    }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto(`/tx/${hash(fixture.id)}`);
      await expectKind(page, fixture.kind);
      await expect(page.getByText('Confirmed', { exact: true })).toBeVisible();
      const flow = page.getByRole('region', { name: 'Transaction flow', exact: true });
      const target = flow.getByRole('link', { name: fixture.target, exact: true });
      await expect(target).toHaveAttribute('href', `/address/${fixture.target}`);
      await flow.getByRole('button', { name: fixture.copy, exact: true }).click();
      await expect.poll(() => copiedValues(page)).toEqual([fixture.target]);
      await expectNoOverflow(page);
    });
  }

  test(`reverted transfer retains its execution status at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/tx/${hash(14)}`);
    await expectKind(page, 'Transfer');
    await expect(page.getByText('Reverted', { exact: true })).toBeVisible();
    await expect(page.getByText('Confirmed', { exact: true })).toHaveCount(0);
    await expect(page.getByRole('region', { name: 'Transaction flow', exact: true })).toBeVisible();
    await expectNoOverflow(page);
  });

  test(`factory creation keeps the actual transaction recipient at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/tx/${hash(15)}`);
    await expectKind(page, 'Contract call');
    await expect(page.getByText('Confirmed', { exact: true })).toBeVisible();
    const flow = page.getByRole('region', { name: 'Transaction flow', exact: true });
    await expect(flow.getByRole('link', { name: recipient, exact: true })).toHaveAttribute(
      'href',
      `/address/${recipient}`
    );
    await expect(flow.getByRole('link', { name: createdContract, exact: true })).toHaveCount(0);
    await flow.getByRole('button', { name: 'Copy recipient address', exact: true }).click();
    await expect.poll(() => copiedValues(page)).toEqual([recipient]);
    await expect(
      page
        .getByRole('region', { name: 'Contract Created', exact: true })
        .getByRole('link', { name: createdContract, exact: true })
    ).toBeVisible();
    await expectNoOverflow(page);
  });

  test(`reverted deployment does not invent a recipient at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/tx/${hash(16)}`);
    await expectKind(page, 'Contract creation');
    await expect(page.getByText('Reverted', { exact: true })).toBeVisible();
    const flow = page.getByRole('region', { name: 'Transaction flow', exact: true });
    await expect(flow.getByText('No contract created', { exact: true })).toBeVisible();
    await expect(flow.getByRole('link')).toHaveCount(1);
    await expect(
      flow.getByRole('button', { name: 'Copy contract address', exact: true })
    ).toHaveCount(0);
    await expectNoOverflow(page);
  });
}
