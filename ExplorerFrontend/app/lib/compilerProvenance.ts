import { sha256 } from "@noble/hashes/sha256.js";
import {
  bytesToHex,
  concatBytes,
  utf8ToBytes,
} from "@noble/hashes/utils.js";

import type {
  CompilerProvenance,
  CompilerProvenanceComponent,
} from "../types/address";

export const COMPILER_PROVENANCE_SCHEMA_V2 =
  "qrl.contract-compiler-provenance.v2";
const NATIVE_COMPONENT_NAMES = ["hypc", "nsjail", "policy"] as const;
const SHA256_PATTERN = /^[0-9a-f]{64}$/;

export type CompilerProvenanceState =
  | { status: "legacy-unrecorded" }
  | { status: "invalid-recorded" }
  | { status: "digest-backed"; provenance: CompilerProvenance };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function parseComponent(
  value: unknown,
  expectedName: (typeof NATIVE_COMPONENT_NAMES)[number],
): CompilerProvenanceComponent | null {
  if (!isRecord(value)) return null;
  if (
    typeof value.name !== "string" ||
    value.name !== expectedName ||
    typeof value.sha256 !== "string" ||
    !SHA256_PATTERN.test(value.sha256)
  ) {
    return null;
  }
  return { name: value.name, sha256: value.sha256 };
}

function encodeUint64BE(value: number): Uint8Array {
  if (!Number.isSafeInteger(value) || value < 0) {
    throw new RangeError("compiler provenance length must be a safe unsigned integer");
  }
  const encoded = new Uint8Array(8);
  const view = new DataView(encoded.buffer);
  view.setUint32(0, Math.floor(value / 0x1_0000_0000));
  view.setUint32(4, value >>> 0);
  return encoded;
}

function encodeField(value: string): Uint8Array {
  const bytes = utf8ToBytes(value);
  return concatBytes(encodeUint64BE(bytes.length), bytes);
}

/** Mirrors backendAPI/models.NativeSandboxCompilerExecutionDigestV2. */
export function nativeSandboxCompilerExecutionDigestV2(
  buildId: string,
  hypcSHA256: string,
  nsjailSHA256: string,
  policySHA256: string,
): string {
  const components = [
    ["hypc", hypcSHA256],
    ["nsjail", nsjailSHA256],
    ["policy", policySHA256],
  ] as const;
  return bytesToHex(
    sha256(
      concatBytes(
        encodeField(COMPILER_PROVENANCE_SCHEMA_V2),
        encodeField("native"),
        encodeField(buildId),
        encodeUint64BE(components.length),
        ...components.flatMap(([name, digest]) => [
          encodeField(name),
          encodeField(digest),
        ]),
      ),
    ),
  );
}

/**
 * Classify persisted compiler fields without consulting the current registry.
 * Only an omitted value is legacy; explicit null is malformed.
 */
export function classifyCompilerProvenance(
  value: unknown,
  compilerVersion?: string,
): CompilerProvenanceState {
  if (value === undefined) {
    return { status: "legacy-unrecorded" };
  }
  if (!isRecord(value)) return { status: "invalid-recorded" };

  const rawComponents = value.components;
  if (
    !Array.isArray(rawComponents) ||
    rawComponents.length !== NATIVE_COMPONENT_NAMES.length
  ) {
    return { status: "invalid-recorded" };
  }
  const components = NATIVE_COMPONENT_NAMES.map((name, index) =>
    parseComponent(rawComponents[index], name),
  );
  if (
    components.some((component) => component === null) ||
    value.schema !== COMPILER_PROVENANCE_SCHEMA_V2 ||
    value.kind !== "native" ||
    typeof value.buildId !== "string" ||
    value.buildId.length === 0 ||
    typeof compilerVersion !== "string" ||
    compilerVersion.length === 0 ||
    compilerVersion !== value.buildId ||
    typeof value.executionDigest !== "string" ||
    !SHA256_PATTERN.test(value.executionDigest) ||
    value.executionDigest !==
      nativeSandboxCompilerExecutionDigestV2(
        value.buildId,
        components[0]!.sha256,
        components[1]!.sha256,
        components[2]!.sha256,
      )
  ) {
    return { status: "invalid-recorded" };
  }

  return {
    status: "digest-backed",
    provenance: {
      schema: value.schema,
      kind: value.kind,
      buildId: value.buildId,
      executionDigest: value.executionDigest,
      components: components as CompilerProvenanceComponent[],
    },
  };
}
