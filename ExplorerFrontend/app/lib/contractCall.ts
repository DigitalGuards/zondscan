import * as zondAbi from '@theqrl/web3-qrl-abi';
import { qNormaliseAbiValue } from './helpers';
import { errorMessage, InvalidInputError, isArray, isRecord } from './guards';

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

export type ContractReadResult =
  { decoded: true; text: string } | { decoded: false; error: string; raw: string };

/**
 * Decodes a read call's reply for display. @theqrl/web3-qrl-abi 2.x refuses a
 * reply that is not canonical ABI, such as a bool word other than 0 or 1, an
 * integer with high bits set or bytesN with non-zero padding, where 1.x showed
 * a value read from part of the word. Such a reply comes back undecoded with
 * the codec's reason and the raw hex, so the node's answer stays visible.
 */
export function decodeContractRead(
  outputs: readonly { name: string; type: string }[],
  raw: string
): ContractReadResult {
  const types = outputs.map((o) => o.type);
  let decoded: unknown;
  try {
    decoded = zondAbi.decodeParameters(types, raw);
  } catch (e) {
    return {
      decoded: false,
      error: `The reply does not decode as (${types.join(', ')}): ${errorMessage(e)}`,
      raw,
    };
  }
  return { decoded: true, text: formatDecoded(decoded, outputs) };
}

// formatDecoded renders the ABI codec's decodeParameters output for
// human consumption. The Result object carries both `0,1,…` index keys
// and any named keys, plus a `__length__`. We flatten to a stable shape,
// and qNormaliseAbiValue (in app/lib/helpers.ts) maps any address-typed
// value to the canonical Q prefix used by the rest of ZondScan.
function formatDecoded(
  decoded: unknown,
  outputs: readonly { name: string; type: string }[]
): string {
  if (!isRecord(decoded)) throw new InvalidInputError('Invalid decoded contract response');
  const out: Record<string, unknown> = {};
  outputs.forEach((o, i) => {
    const v = decoded[String(i)];
    out[o.name && o.name !== '' ? o.name : `_${i}`] = qNormaliseAbiValue(v, o.type);
  });
  return JSON.stringify(out, replacer, 2);
}

function replacer(_k: string, v: unknown): unknown {
  return typeof v === 'bigint' ? v.toString() : v;
}
