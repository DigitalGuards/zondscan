// Deployment identity is fixed for the lifetime of a build. Upgrade this
// capability alongside the reviewed address, ABI, and wallet port.
const NATIVE_ADDRESS_BYTES = 20;

function networkId(value) {
  const network = (value || '').trim() || 'v2';
  if (network !== 'v2' && network !== 'v3') {
    throw new Error('EXPLORER_NETWORK must be v2 or v3');
  }
  return network;
}

function explorerOrigin(value, name) {
  if (!value) return null;
  let url;
  try {
    url = new URL(value);
  } catch {
    throw new Error(`${name} must be an absolute explorer origin`);
  }
  const loopback = ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname);
  if (
    (url.protocol !== 'https:' && !(url.protocol === 'http:' && loopback)) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash ||
    url.pathname !== '/'
  ) {
    throw new Error(`${name} must be an HTTPS origin without a path, credentials, or query`);
  }
  return `${url.origin}/`;
}

function readNetworkConfig(env) {
  const network = networkId(env.EXPLORER_NETWORK || env.NEXT_PUBLIC_EXPLORER_NETWORK);
  if (
    env.EXPLORER_NETWORK &&
    env.NEXT_PUBLIC_EXPLORER_NETWORK &&
    networkId(env.EXPLORER_NETWORK) !== networkId(env.NEXT_PUBLIC_EXPLORER_NETWORK)
  ) {
    throw new Error('Server and public explorer network must match');
  }
  const v2Url = explorerOrigin(
    env.NEXT_PUBLIC_V2_EXPLORER_URL || 'https://zondscan.com',
    'NEXT_PUBLIC_V2_EXPLORER_URL'
  );
  const v3Url = explorerOrigin(env.NEXT_PUBLIC_V3_EXPLORER_URL, 'NEXT_PUBLIC_V3_EXPLORER_URL');
  const availability = env.NEXT_PUBLIC_V3_EXPLORER_AVAILABLE || 'false';
  if (availability !== 'true' && availability !== 'false') {
    throw new Error('NEXT_PUBLIC_V3_EXPLORER_AVAILABLE must be true or false');
  }
  if (v2Url === v3Url) throw new Error('Testnet v2 and v3 require separate explorer origins');
  if (availability === 'true' && !v3Url) {
    throw new Error('An available Testnet v3 requires NEXT_PUBLIC_V3_EXPLORER_URL');
  }
  if (network === 'v3' && (availability !== 'true' || !v3Url)) {
    throw new Error('A Testnet v3 deployment requires its available public origin');
  }
  return { network, v2Url, v3Url, v3Available: availability === 'true' };
}

function assertNetworkCapability(network, addressBytes = NATIVE_ADDRESS_BYTES) {
  const expected = network === 'v3' ? 64 : 20;
  if (addressBytes !== expected) {
    throw new Error(
      `This ${addressBytes}-byte frontend cannot serve Testnet ${network}; the reviewed ${expected}-byte port is required`
    );
  }
}

module.exports = {
  NATIVE_ADDRESS_BYTES,
  networkId,
  explorerOrigin,
  readNetworkConfig,
  assertNetworkCapability,
};
