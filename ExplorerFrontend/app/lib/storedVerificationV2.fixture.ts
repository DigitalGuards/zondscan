import {
  sourceBundleDigestV1,
  verificationArtifactDigestV2,
  VERIFICATION_RECORD_SCHEMA_V2,
} from "./storedVerification";
import {
  COMPILER_PROVENANCE_V2_BUILD_ID,
  COMPILER_PROVENANCE_V2_FIXTURE,
} from "./compilerProvenanceV2.fixture";
import type { ContractData } from "../types/address";

const sourceCode = "contract Main {}";
const imports: Record<string, string> = {};

export const STORED_VERIFICATION_V2_BASE = {
  verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V2,
  address: `Q${"a".repeat(128)}`,
  creationTransaction: `0x${"b".repeat(64)}`,
  creationBlockNumber: "0x10",
  creationBlockHash: `0x${"c".repeat(64)}`,
  chainId: "0x539",
  contractCodeSha256: "d".repeat(64),
  genesisContract: false,
  compilerProvenance: COMPILER_PROVENANCE_V2_FIXTURE,
  compilerVersion: COMPILER_PROVENANCE_V2_BUILD_ID,
  contractName: "Main",
  sourceCode,
  imports,
  sourceBundleDigest: sourceBundleDigestV1("Main", sourceCode, imports),
  abi: '[{"type":"function","name":"value"}]',
  optimizationEnabled: true,
  optimizationRuns: 200,
  evmVersion: "",
  constructorArguments: "",
  libraries: {
    Z: "Qz",
    A: "Qa",
  } as Record<string, string>,
  license: "",
  verificationMethod: "full-source",
};

export const STORED_VERIFICATION_V2_FIXTURE = {
  ...STORED_VERIFICATION_V2_BASE,
  verificationArtifactDigest:
    "qrl.contract-verification-artifact.v2:sha256:cf2043bd0715e9b395aee59b3b5cde6ccea131b2f72cd348e980b0d9b541c944",
};

export function makeContractDataV2(
  overrides: Partial<ContractData> = {},
): ContractData {
  const candidate: ContractData = {
    creatorAddress: `Q${"0".repeat(128)}`,
    contractCode: "00",
    isToken: false,
    status: "0x1",
    decimals: 0,
    name: "",
    symbol: "",
    updatedAt: "",
    verified: true,
    ...STORED_VERIFICATION_V2_BASE,
    ...overrides,
  };

  if (
    !Object.prototype.hasOwnProperty.call(overrides, "sourceBundleDigest") &&
    candidate.contractName &&
    candidate.sourceCode
  ) {
    candidate.sourceBundleDigest = sourceBundleDigestV1(
      candidate.contractName,
      candidate.sourceCode,
      candidate.imports ?? {},
    );
  }

  if (
    Object.prototype.hasOwnProperty.call(
      overrides,
      "verificationArtifactDigest",
    )
  ) {
    return candidate;
  }
  const candidateDigest = verificationArtifactDigestV2(candidate);
  return candidateDigest === null
    ? candidate
    : { ...candidate, verificationArtifactDigest: candidateDigest };
}
