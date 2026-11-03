import { isArray } from './guards';

// A lowercase q in front of a uniform-case body. @theqrl/web3 1.0.3 encoded
// such an address, and 2.0.0 accepts only a capital Q. A mixed-case body
// carries its checksum, and 1.0.3 refused it behind a lowercase q, so that
// spelling is passed on as typed.
const LOWERCASE_Q_ADDRESS = /^q(?:[0-9a-f]{128}|[0-9A-F]{128})$/;

function isAddressType(type: string): boolean {
  return type.replace(/(?:\[\d*\])+$/, '') === 'address';
}

function withCapitalQ(value: unknown): unknown {
  if (typeof value === 'string') {
    return LOWERCASE_Q_ADDRESS.test(value) ? `Q${value.slice(1)}` : value;
  }
  return isArray(value) ? value.map(withCapitalQ) : value;
}

/**
 * Converts a user-typed string into the JS shape the installed ABI codec's
 * encodeFunctionCall takes. The Read and Write tabs share it, so both accept
 * identical input formats. Keep this minimal, anything fancy (tuples, fixed
 * arrays) the user can express as JSON.
 */
export function parseContractArg(raw: string, type: string): unknown {
  const t = raw.trim();
  if (t === '') {
    if (type === 'bool') return false;
    if (type.startsWith('uint') || type.startsWith('int')) return '0';
    return '';
  }
  if (type === 'bool') return t.toLowerCase() === 'true' || t === '1';
  if (type.endsWith(']') || type === 'tuple') {
    let parsed: unknown;
    try {
      parsed = JSON.parse(t);
    } catch {
      throw new Error(`expected JSON for ${type}: e.g. [1,2,3]`);
    }
    return isAddressType(type) ? withCapitalQ(parsed) : parsed;
  }
  // Integers stay strings, because the ABI codec accepts strings for big ints.
  return isAddressType(type) ? withCapitalQ(t) : t;
}
