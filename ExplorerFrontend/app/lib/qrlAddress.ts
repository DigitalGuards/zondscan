import { shake256 } from '@noble/hashes/sha3.js';
import { utf8ToBytes } from '@noble/hashes/utils.js';

export const QRL_ADDRESS_HEX_LENGTH = 128;

const QRL_ADDRESS_BODY_PATTERN = new RegExp(
  `^[0-9a-fA-F]{${QRL_ADDRESS_HEX_LENGTH}}$`,
);

export type CanonicalQrlAddress = `Q${string}`;

function addressBody(input: string): string | null {
  const value = input.trim();
  let body: string;

  if (value.startsWith('Q') || value.startsWith('q')) {
    body = value.slice(1);
  } else if (value.startsWith('0x') || value.startsWith('0X')) {
    body = value.slice(2);
  } else {
    body = value;
  }

  return QRL_ADDRESS_BODY_PATTERN.test(body) ? body : null;
}

function checksummedBody(lowerBody: string): string {
  const hash = shake256
    .create({ dkLen: QRL_ADDRESS_HEX_LENGTH / 2 })
    .update(utf8ToBytes(lowerBody))
    .digest();

  let result = '';
  for (let index = 0; index < lowerBody.length; index += 1) {
    const character = lowerBody[index];
    if (character >= 'a' && character <= 'f') {
      const byte = hash[index >> 1];
      const nibble = (index & 1) === 0 ? byte >> 4 : byte & 0x0f;
      if (nibble >= 8) {
        result += character.toUpperCase();
        continue;
      }
    }
    result += character;
  }
  return result;
}

function hasValidCase(body: string): boolean {
  const lowerBody = body.toLowerCase();
  const upperBody = body.toUpperCase();
  if (body === lowerBody || body === upperBody) return true;
  return body === checksummedBody(lowerBody);
}

/**
 * Reports whether a value has one supported QRL address shape. This check only
 * covers prefix, width, and hexadecimal characters. Use canonicalizeQrlAddress
 * at trust boundaries because mixed-case input also carries a checksum.
 */
export function hasQrlAddressShape(input: string | undefined | null): boolean {
  return typeof input === 'string' && addressBody(input) !== null;
}

/**
 * Accepts Q/q, 0x/0X, and bare 128-hex aliases. Uniform-case bodies remain
 * compatible. Mixed-case bodies must carry the QIP-55 SHAKE256 checksum.
 * Successful output always uses uppercase Q and the canonical checksum body.
 */
export function canonicalizeQrlAddress(
  input: string | undefined | null,
): CanonicalQrlAddress | null {
  if (typeof input !== 'string') return null;
  const body = addressBody(input);
  if (!body || !hasValidCase(body)) return null;
  return `Q${checksummedBody(body.toLowerCase())}`;
}

export function isValidQrlAddress(input: string | undefined | null): boolean {
  return canonicalizeQrlAddress(input) !== null;
}

/**
 * Builds the stable first, middle, and final visual fingerprint for passive UI.
 * Invalid and legacy-width values are returned unchanged so callers do not
 * accidentally disguise malformed data.
 */
export function compactQrlAddress(address: string | undefined | null): string {
  if (!address) return '';
  const canonical = canonicalizeQrlAddress(address);
  if (!canonical) return address;

  const body = canonical.slice(1);
  const middleStart = Math.floor((body.length - 8) / 2);
  return `Q${body.slice(0, 8)}...${body.slice(middleStart, middleStart + 8)}...${body.slice(-8)}`;
}
