import {
  mountFaucetCaptcha,
  parseFaucetStatus,
  publicTurnstileSiteKey,
  type TurnstileApi,
} from './captcha';

const status = {
  configured: true,
  captchaEnabled: true,
  turnstileSiteKey: '1x00000000000000000000AA',
  dripQuanta: '10',
  cooldownHours: 24,
};

describe('runtime faucet status', () => {
  it('accepts the runtime public key with the build-time environment absent', () => {
    const previous = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY;
    delete process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY;
    try {
      expect(parseFaucetStatus(status)).toEqual(status);
    } finally {
      if (previous === undefined) delete process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY;
      else process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY = previous;
    }
  });

  it.each([undefined, null, '', '<script>', 'a'.repeat(101), 123])(
    'rejects invalid site key %p',
    (key) => {
      expect(publicTurnstileSiteKey(key)).toBeNull();
      expect(parseFaucetStatus({ ...status, turnstileSiteKey: key })?.turnstileSiteKey).toBeNull();
    }
  );

  it.each([
    null,
    [],
    {},
    { ...status, configured: 'true' },
    { ...status, captchaEnabled: undefined },
    { ...status, dripQuanta: '0' },
    { ...status, cooldownHours: -1 },
  ])('fails closed for malformed status %p', (value) => {
    expect(parseFaucetStatus(value)).toBeNull();
  });

  it('copies only public fields and retains supported whole-number drip formatting', () => {
    expect(parseFaucetStatus({ ...status, dripQuanta: '0010', privateField: 'ignored' })).toEqual({
      ...status,
      dripQuanta: '0010',
    });
  });
});

describe('faucet widget ownership', () => {
  const element = {} as HTMLElement;
  let options: Parameters<TurnstileApi['render']>[1];
  let api: TurnstileApi;
  let onToken: jest.Mock;
  let onError: jest.Mock;

  beforeEach(() => {
    api = {
      render: jest.fn((_element, value) => {
        options = value;
        return 'widget';
      }),
      remove: jest.fn(),
    };
    onToken = jest.fn();
    onError = jest.fn();
  });

  it('uses only the runtime key and clears solved tokens on expiry/error', () => {
    mountFaucetCaptcha(api, element, status.turnstileSiteKey, onToken, onError);
    expect(options.sitekey).toBe(status.turnstileSiteKey);
    options.callback('solved-token');
    expect(onToken).toHaveBeenLastCalledWith('solved-token');
    options['expired-callback']();
    expect(onToken).toHaveBeenLastCalledWith('');
    options.callback('new-token');
    options['error-callback']();
    expect(onToken).toHaveBeenLastCalledWith('');
    expect(onError).toHaveBeenCalledTimes(1);
  });

  it('removes exactly once and ignores every stale callback after disposal', () => {
    const dispose = mountFaucetCaptcha(api, element, status.turnstileSiteKey, onToken, onError);
    dispose();
    dispose();
    options.callback('late-token');
    options['expired-callback']();
    options['error-callback']();
    expect(api.remove).toHaveBeenCalledTimes(1);
    expect(api.remove).toHaveBeenCalledWith('widget');
    expect(onToken).not.toHaveBeenCalled();
    expect(onError).not.toHaveBeenCalled();
  });

  it('allows a fresh remount while ignoring the old instance', () => {
    const dispose = mountFaucetCaptcha(api, element, status.turnstileSiteKey, onToken, onError);
    const oldOptions = options;
    dispose();
    mountFaucetCaptcha(api, element, status.turnstileSiteKey, onToken, onError);
    oldOptions.callback('old-token');
    expect(onToken).not.toHaveBeenCalled();
    options.callback('current-token');
    expect(onToken).toHaveBeenCalledWith('current-token');
  });

  it('invalidates callbacks before the provider removes a widget', () => {
    api.remove = jest.fn(() => {
      options.callback('late-token');
      throw new Error('provider error');
    });
    const dispose = mountFaucetCaptcha(api, element, status.turnstileSiteKey, onToken, onError);
    expect(dispose).not.toThrow();
    expect(onToken).not.toHaveBeenCalled();
  });

  it.each(['', ' ', 'a'.repeat(2049)])('rejects invalid callback token length %i', (token) => {
    mountFaucetCaptcha(api, element, status.turnstileSiteKey, onToken, onError);
    options.callback(token);
    expect(onToken).toHaveBeenLastCalledWith('');
    expect(onError).toHaveBeenCalledTimes(1);
  });

  it('fails closed when the script API throws or the runtime key is missing', () => {
    api.render = jest.fn(() => {
      throw new Error('render failed');
    });
    mountFaucetCaptcha(api, element, status.turnstileSiteKey, onToken, onError);
    expect(onToken).toHaveBeenLastCalledWith('');
    expect(onError).toHaveBeenCalledTimes(1);
    jest.clearAllMocks();
    mountFaucetCaptcha(api, element, '', onToken, onError);
    expect(api.render).not.toHaveBeenCalled();
    expect(onError).toHaveBeenCalledTimes(1);
  });
});
