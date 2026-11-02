import { describe, expect, it } from "@jest/globals";

import { makeContractDataV2 } from "./storedVerificationV2.fixture";
import { storedVerificationV3Vector } from "./storedVerificationV3.fixture";
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

describe("classifyVerificationJobArtifact for source trees and via-IR", () => {
  function successfulV3Job(): VerificationJobArtifactInput {
    const record = storedVerificationV3Vector(0);
    return {
      jobId: "fedcba9876543210",
      status: "success",
      address: record.address,
      target: {
        address: record.address,
        creationTransaction: record.creationTransaction,
        creationBlockNumber: record.creationBlockNumber,
        creationBlockHash: record.creationBlockHash,
        chainId: record.chainId,
        deployedCodeSha256: record.contractCodeSha256,
      },
      payload: {
        sourceCode: record.sourceCode,
        contractName: record.contractName,
        compilerVersion: record.compilerVersion,
        compilerProvenance: record.compilerProvenance,
        optimizationEnabled: record.optimizationEnabled,
        optimizationRuns: record.optimizationRuns,
        imports: record.imports,
        primarySourcePath: record.primarySourcePath,
        viaIR: record.viaIR,
        license: record.license,
        verificationMethod: record.verificationMethod,
      },
      result: {
        abi: record.abi,
        bytecodeHash: record.contractCodeSha256,
        artifactDigest: record.verificationArtifactDigest,
        compilerProvenance: record.compilerProvenance,
        verifiedAt: "2026-11-02T00:00:00Z",
      },
    };
  }

  it("accepts a successful schema-v3 job artifact", () => {
    expect(classifyVerificationJobArtifact(successfulV3Job())).toBe("digest-backed");
  });

  it.each([
    ["via-IR flag dropped", { viaIR: undefined }],
    ["primary path changed", { primarySourcePath: "contracts/Counter.hyp" }],
  ] as const)("rejects a job whose %s", (_label, override) => {
    const job = successfulV3Job();
    job.payload = { ...job.payload, ...override };
    expect(classifyVerificationJobArtifact(job)).toBe("invalid-recorded");
  });
});
