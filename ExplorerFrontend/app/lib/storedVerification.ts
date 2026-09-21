import { sha256 } from "@noble/hashes/sha256.js";
import { bytesToHex, utf8ToBytes } from "@noble/hashes/utils.js";

import { classifyCompilerProvenance } from "./compilerProvenance";

const SOURCE_BUNDLE_VERSION = "qrl.verified-source-bundle.v1";
const SOURCE_BUNDLE_PREFIX = `${SOURCE_BUNDLE_VERSION}:sha256:`;
export const VERIFICATION_RECORD_SCHEMA_V1 =
  "qrl.contract-verification-record.v1";
export const VERIFICATION_RECORD_SCHEMA_V2 =
  "qrl.contract-verification-record.v2";
export const VERIFICATION_ARTIFACT_DIGEST_PREFIX_V2 =
  "qrl.contract-verification-artifact.v2:sha256:";

const SHA256_PATTERN = /^[0-9a-f]{64}$/;
const Q128_ADDRESS_PATTERN = /^Q[0-9a-f]{128}$/;
const HASH_PATTERN = /^0x[0-9a-f]{64}$/;
const HEX_QUANTITY_PATTERN = /^0x(?:0|[1-9a-f][0-9a-f]*)$/;

export type StoredVerificationStatus =
  "digest-backed" | "legacy-unrecorded" | "invalid-recorded";

export type VerifiedImport = readonly [filename: string, source: string];

export interface VerifiedImportsState {
  valid: boolean;
  files: VerifiedImport[];
}

export interface StoredVerificationInput {
  verificationRecordSchema?: unknown;
  address?: unknown;
  creationTransaction?: unknown;
  creationBlockNumber?: unknown;
  creationBlockHash?: unknown;
  chainId?: unknown;
  contractCodeSha256?: unknown;
  genesisContract?: unknown;
  compilerProvenance?: unknown;
  compilerVersion?: unknown;
  contractName?: unknown;
  sourceCode?: unknown;
  imports?: unknown;
  sourceBundleDigest?: unknown;
  abi?: unknown;
  optimizationEnabled?: unknown;
  optimizationRuns?: unknown;
  evmVersion?: unknown;
  constructorArguments?: unknown;
  libraries?: unknown;
  license?: unknown;
  verificationMethod?: unknown;
  verificationArtifactDigest?: unknown;
}

type HashWriter = { update(data: Uint8Array): HashWriter };

export function classifyVerifiedImports(value: unknown): VerifiedImportsState {
  if (value === undefined) return { valid: true, files: [] };
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return { valid: false, files: [] };
  }
  const prototype = Object.getPrototypeOf(value);
  if (prototype !== Object.prototype && prototype !== null) {
    return { valid: false, files: [] };
  }

  const entries = Object.entries(value);
  if (entries.some(([, source]) => typeof source !== "string")) {
    return { valid: false, files: [] };
  }
  const files = (entries as Array<[string, string]>).sort(([left], [right]) =>
    compareUTF8(left, right),
  );
  return { valid: true, files };
}

export function sourceBundleDigestV1(
  contractName: string,
  sourceCode: string,
  imports: Record<string, string>,
): string {
  const writer = sha256.create();
  writer.update(utf8ToBytes(SOURCE_BUNDLE_VERSION));
  writer.update(new Uint8Array([0]));

  const files = classifyVerifiedImports(imports).files;
  writeUint64(writer, 1 + files.length);
  writeDigestFile(
    writer,
    "primary",
    contractName ? `${contractName}.hyp` : "primary.hyp",
    sourceCode,
  );
  for (const [filename, source] of files) {
    writeDigestFile(writer, "import", filename, source);
  }
  return SOURCE_BUNDLE_PREFIX + bytesToHex(writer.digest());
}

export function classifyStoredVerification(
  contract: StoredVerificationInput,
): StoredVerificationStatus {
  if (contract.verificationRecordSchema === undefined) {
    return contract.compilerProvenance === undefined &&
      contract.sourceBundleDigest === undefined &&
      contract.verificationArtifactDigest === undefined
      ? "legacy-unrecorded"
      : "invalid-recorded";
  }
  if (contract.verificationRecordSchema === VERIFICATION_RECORD_SCHEMA_V1) {
    if (contract.verificationArtifactDigest !== undefined) {
      return "invalid-recorded";
    }
    return hasValidSourceAndCompilerRecord(contract)
      ? "legacy-unrecorded"
      : "invalid-recorded";
  }
  if (contract.verificationRecordSchema !== VERIFICATION_RECORD_SCHEMA_V2) {
    return "invalid-recorded";
  }

  if (!hasValidSourceAndCompilerRecord(contract)) {
    return "invalid-recorded";
  }
  if (
    typeof contract.verificationArtifactDigest !== "string" ||
    !contract.verificationArtifactDigest.startsWith(
      VERIFICATION_ARTIFACT_DIGEST_PREFIX_V2,
    ) ||
    !SHA256_PATTERN.test(
      contract.verificationArtifactDigest.slice(
        VERIFICATION_ARTIFACT_DIGEST_PREFIX_V2.length,
      ),
    )
  ) {
    return "invalid-recorded";
  }

  const expectedArtifactDigest = verificationArtifactDigestV2(contract);
  return expectedArtifactDigest !== null &&
    contract.verificationArtifactDigest === expectedArtifactDigest
    ? "digest-backed"
    : "invalid-recorded";
}

/** Mirrors backendAPI/models.VerificationArtifactDigestV2. */
export function verificationArtifactDigestV2(
  contract: StoredVerificationInput,
): string | null {
  if (contract.verificationRecordSchema !== VERIFICATION_RECORD_SCHEMA_V2) {
    return null;
  }

  const compilerState = classifyCompilerProvenance(
    contract.compilerProvenance,
    typeof contract.compilerVersion === "string"
      ? contract.compilerVersion
      : undefined,
  );
  if (compilerState.status !== "digest-backed") return null;

  const importsState = classifyVerifiedImports(contract.imports);
  if (
    typeof contract.contractName !== "string" ||
    contract.contractName.length === 0 ||
    typeof contract.sourceCode !== "string" ||
    contract.sourceCode.length === 0 ||
    typeof contract.sourceBundleDigest !== "string" ||
    !importsState.valid
  ) {
    return null;
  }

  const imports = Object.fromEntries(importsState.files);
  if (
    contract.sourceBundleDigest !==
    sourceBundleDigestV1(contract.contractName, contract.sourceCode, imports)
  ) {
    return null;
  }

  const address = requiredMatchingString(contract.address, Q128_ADDRESS_PATTERN);
  const creationBlockNumber = requiredMatchingString(
    contract.creationBlockNumber,
    HEX_QUANTITY_PATTERN,
  );
  const creationBlockHash = requiredMatchingString(
    contract.creationBlockHash,
    HASH_PATTERN,
  );
  const chainId = requiredMatchingString(contract.chainId, HEX_QUANTITY_PATTERN);
  const deployedCodeSHA256 = requiredMatchingString(
    contract.contractCodeSha256,
    SHA256_PATTERN,
  );
  const genesisContract = optionalBoolean(contract.genesisContract);
  const creationTransaction = optionalString(contract.creationTransaction);
  const abi = requiredString(contract.abi);
  const optimizationEnabled = requiredBoolean(contract.optimizationEnabled);
  const optimizationRuns = requiredSafeInteger(contract.optimizationRuns);
  const evmVersion = optionalString(contract.evmVersion);
  const constructorArguments = optionalString(contract.constructorArguments);
  const license = optionalString(contract.license);
  const verificationMethod = requiredString(contract.verificationMethod);
  const librariesState = classifyVerifiedImports(contract.libraries);

  if (
    address === null ||
    creationBlockNumber === null ||
    creationBlockHash === null ||
    chainId === null ||
    deployedCodeSHA256 === null ||
    genesisContract === null ||
    creationTransaction === null ||
    abi === null ||
    optimizationEnabled === null ||
    optimizationRuns === null ||
    evmVersion === null ||
    constructorArguments === null ||
    license === null ||
    verificationMethod !== "full-source" ||
    !librariesState.valid ||
    (!genesisContract && !HASH_PATTERN.test(creationTransaction)) ||
    (genesisContract && creationTransaction !== "")
  ) {
    return null;
  }

  const writer = sha256.create();
  for (const value of [
    VERIFICATION_RECORD_SCHEMA_V2,
    address,
    creationTransaction,
    creationBlockNumber,
    creationBlockHash,
    chainId,
    deployedCodeSHA256,
    String(genesisContract),
    contract.sourceBundleDigest,
    abi,
    contract.contractName,
    compilerState.provenance.buildId,
  ]) {
    writeDigestField(writer, value);
  }

  writeUint64(writer, 1);
  for (const value of [
    compilerState.provenance.schema,
    compilerState.provenance.kind,
    compilerState.provenance.buildId,
    compilerState.provenance.executionDigest,
  ]) {
    writeDigestField(writer, value);
  }
  writeUint64(writer, compilerState.provenance.components.length);
  for (const component of compilerState.provenance.components) {
    writeDigestField(writer, component.name);
    writeDigestField(writer, component.sha256);
  }

  for (const value of [
    String(optimizationEnabled),
    String(optimizationRuns),
    evmVersion,
    constructorArguments,
    license,
    verificationMethod,
  ]) {
    writeDigestField(writer, value);
  }
  writeUint64(writer, librariesState.files.length);
  for (const [key, value] of librariesState.files) {
    writeDigestField(writer, key);
    writeDigestField(writer, value);
  }

  return (
    VERIFICATION_ARTIFACT_DIGEST_PREFIX_V2 + bytesToHex(writer.digest())
  );
}

function hasValidSourceAndCompilerRecord(
  contract: StoredVerificationInput,
): boolean {
  const compilerState = classifyCompilerProvenance(
    contract.compilerProvenance,
    typeof contract.compilerVersion === "string"
      ? contract.compilerVersion
      : undefined,
  );
  if (compilerState.status !== "digest-backed") return false;

  const importsState = classifyVerifiedImports(contract.imports);
  if (
    typeof contract.contractName !== "string" ||
    contract.contractName.length === 0 ||
    typeof contract.sourceCode !== "string" ||
    contract.sourceCode.length === 0 ||
    typeof contract.sourceBundleDigest !== "string" ||
    !importsState.valid
  ) {
    return false;
  }

  return (
    contract.sourceBundleDigest ===
    sourceBundleDigestV1(
      contract.contractName,
      contract.sourceCode,
      Object.fromEntries(importsState.files),
    )
  );
}

function requiredString(value: unknown): string | null {
  return typeof value === "string" ? value : null;
}

function optionalString(value: unknown): string | null {
  return value === undefined ? "" : requiredString(value);
}

function requiredMatchingString(
  value: unknown,
  pattern: RegExp,
): string | null {
  return typeof value === "string" && pattern.test(value) ? value : null;
}

function requiredBoolean(value: unknown): boolean | null {
  return typeof value === "boolean" ? value : null;
}

function optionalBoolean(value: unknown): boolean | null {
  return value === undefined ? false : requiredBoolean(value);
}

function requiredSafeInteger(value: unknown): number | null {
  return typeof value === "number" && Number.isSafeInteger(value)
    ? value
    : null;
}

function compareUTF8(left: string, right: string): number {
  const leftBytes = utf8ToBytes(left);
  const rightBytes = utf8ToBytes(right);
  const length = Math.min(leftBytes.length, rightBytes.length);
  for (let index = 0; index < length; index += 1) {
    if (leftBytes[index] !== rightBytes[index]) {
      return (leftBytes[index] ?? 0) - (rightBytes[index] ?? 0);
    }
  }
  return leftBytes.length - rightBytes.length;
}

function writeDigestFile(
  writer: HashWriter,
  role: string,
  filename: string,
  source: string,
): void {
  writeDigestField(writer, role);
  writeDigestField(writer, filename);
  writeDigestField(writer, source);
}

function writeDigestField(writer: HashWriter, value: string): void {
  const bytes = utf8ToBytes(value);
  writeUint64(writer, bytes.length);
  writer.update(bytes);
}

function writeUint64(writer: HashWriter, value: number): void {
  const bytes = new Uint8Array(8);
  new DataView(bytes.buffer).setBigUint64(0, BigInt(value), false);
  writer.update(bytes);
}
