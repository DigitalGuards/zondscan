import { describe, expect, it } from "@jest/globals";

import { makeContractDataV2 } from "./storedVerificationV2.fixture";
import {
  classifyVerificationJobArtifact,
  type VerificationJobArtifactInput,
} from "./verificationJobArtifact";

function successfulJob(): VerificationJobArtifactInput {
  const contract = makeContractDataV2();
  return {
    jobId: "0123456789abcdef",
    status: "success",
    address: contract.address,
    target: {
      address: contract.address,
      creationTransaction: contract.creationTransaction,
      creationBlockNumber: contract.creationBlockNumber,
      creationBlockHash: contract.creationBlockHash,
      chainId: contract.chainId,
      deployedCodeSha256: contract.contractCodeSha256,
      genesisContract: contract.genesisContract,
    },
    payload: {
      sourceCode: contract.sourceCode,
      contractName: contract.contractName,
      compilerVersion: contract.compilerVersion,
      compilerProvenance: contract.compilerProvenance,
      optimizationEnabled: contract.optimizationEnabled,
      optimizationRuns: contract.optimizationRuns,
      evmVersion: contract.evmVersion,
      constructorArguments: contract.constructorArguments,
      libraries: contract.libraries,
      imports: contract.imports,
      license: contract.license,
      verificationMethod: contract.verificationMethod,
    },
    result: {
      abi: contract.abi,
      bytecodeHash: contract.contractCodeSha256,
      artifactDigest: contract.verificationArtifactDigest,
      compilerProvenance: contract.compilerProvenance,
      verifiedAt: "2026-08-28T00:00:00Z",
    },
  };
}

describe("classifyVerificationJobArtifact", () => {
  it("recomputes and accepts a complete successful job artifact", () => {
    expect(classifyVerificationJobArtifact(successfulJob())).toBe(
      "digest-backed",
    );
  });

  it("ignores nonterminal jobs", () => {
    expect(
      classifyVerificationJobArtifact({
        ...successfulJob(),
        status: "compiling",
      }),
    ).toBeNull();
  });

  it("rejects a success result whose target changed", () => {
    const job = successfulJob();
    job.target = { ...job.target, creationBlockNumber: "0x11" };
    expect(classifyVerificationJobArtifact(job)).toBe("invalid-recorded");
  });

  it("rejects a success result whose artifact digest is missing", () => {
    const job = successfulJob();
    job.result = { ...job.result, artifactDigest: undefined };
    expect(classifyVerificationJobArtifact(job)).toBe("invalid-recorded");
  });

  it("rejects payload and result compiler identity disagreement", () => {
    const job = successfulJob();
    job.result = {
      ...job.result,
      compilerProvenance: {
        ...(job.result?.compilerProvenance as Record<string, unknown>),
        executionDigest: "f".repeat(64),
      },
    };
    expect(classifyVerificationJobArtifact(job)).toBe("invalid-recorded");
  });
});
