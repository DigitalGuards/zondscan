jest.mock('server-only', () => ({}));

import { GET } from './route';

describe('public faucet runtime status', () => {
  const original = { ...process.env };
  beforeEach(() => {
    process.env.FAUCET_SEED = 'private-seed-sentinel';
    process.env.DATABASE_URL = 'mongodb://private-database.invalid';
    process.env.FAUCET_RPC_URL = 'https://private-rpc.invalid';
    process.env.TURNSTILE_SECRET = 'private-secret-sentinel';
    process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY = 'runtime-key-first';
    process.env.FAUCET_DRIP_QUANTA = '10';
    process.env.FAUCET_COOLDOWN_HOURS = '24';
  });
  afterEach(() => {
    process.env = { ...original };
  });

  it('serializes only explicitly allowed public fields with no caching', async () => {
    const response = GET();
    expect(response.headers.get('Cache-Control')).toBe('no-store');
    expect(await response.json()).toEqual({
      configured: true,
      captchaEnabled: true,
      turnstileSiteKey: 'runtime-key-first',
      dripQuanta: '10',
      cooldownHours: 24,
    });
  });

  it('reads changed runtime keys on every request after the module is loaded', async () => {
    expect((await GET().json()).turnstileSiteKey).toBe('runtime-key-first');
    process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY = 'runtime-key-second';
    expect((await GET().json()).turnstileSiteKey).toBe('runtime-key-second');
  });

  it.each(['', 'invalid key', '<script>'])(
    'returns null for an unavailable public key %p',
    async (key) => {
      process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY = key;
      const warn = jest.spyOn(console, 'warn').mockImplementation(() => undefined);
      try {
        const result = await GET().json();
        expect(result.turnstileSiteKey).toBeNull();
        expect(result.captchaEnabled).toBe(Boolean(key));
      } finally {
        warn.mockRestore();
      }
    }
  );
});
