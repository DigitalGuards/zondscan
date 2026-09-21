import * as installedAbi from '@theqrl/web3-qrl-abi';

interface AbiCapabilityCodec {
  encodeParameter(type: string, value: unknown): string;
  decodeParameter(type: string, encoded: string): unknown;
}

const PROBE_ADDRESS = `Q${'12'.repeat(64)}`;
const PROBE_WORD = `0x${PROBE_ADDRESS.slice(1)}`;

/**
 * Detect the QRVM 64-byte ABI layout through observable codec behavior.
 * Keep this observable gate even with the published QIP-55 package so a
 * stale or substituted dependency cannot construct legacy-width calldata.
 */
export function isVm64AbiCodec(codec: AbiCapabilityCodec): boolean {
  try {
    const encoded = codec.encodeParameter('address', PROBE_ADDRESS);
    if (encoded.toLowerCase() !== PROBE_WORD.toLowerCase()) return false;
    const decoded = codec.decodeParameter('address', encoded);
    return typeof decoded === 'string' && decoded.toLowerCase() === PROBE_ADDRESS.toLowerCase();
  } catch {
    return false;
  }
}

let installedCodecSupportsVm64: boolean | undefined;

/** Fail closed before a legacy 32-byte ABI codec can construct calldata. */
export function assertVm64AbiSupport(): void {
  installedCodecSupportsVm64 ??= isVm64AbiCodec(installedAbi);
  if (!installedCodecSupportsVm64) {
    throw new Error('Contract interaction requires a QIP-55 64-byte QRVM ABI build');
  }
}
