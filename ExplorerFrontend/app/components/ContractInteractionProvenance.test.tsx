import { describe, expect, it } from "@jest/globals";
import { renderToStaticMarkup } from "react-dom/server";

jest.mock("./ConnectButton", () => ({
  __esModule: true,
  default: () => null,
}));

import ReadContract from "./ReadContract";
import WriteContract from "./WriteContract";
import { VERIFICATION_RECORD_SCHEMA_V1 } from "../lib/storedVerification";
import { makeContractDataV2 } from "../lib/storedVerificationV2.fixture";
import type { ContractData } from "../types/address";

const interactionABI = JSON.stringify([
  {
    type: "function",
    name: "sensitiveRead",
    stateMutability: "view",
    inputs: [],
    outputs: [{ name: "value", type: "uint256" }],
  },
  {
    type: "function",
    name: "sensitiveWrite",
    stateMutability: "nonpayable",
    inputs: [],
    outputs: [],
  },
]);

function contractData(overrides: Partial<ContractData> = {}): ContractData {
  return makeContractDataV2({ abi: interactionABI, ...overrides });
}

describe("contract interaction provenance gates", () => {
  it("exposes Read and Write cards for a complete V2 record", () => {
    const current = contractData();
    const readHTML = renderToStaticMarkup(
      <ReadContract contractData={current} />,
    );
    const writeHTML = renderToStaticMarkup(
      <WriteContract contractData={current} />,
    );

    expect(readHTML).toContain("sensitiveRead");
    expect(writeHTML).toContain("sensitiveWrite");
    expect(writeHTML).toContain("Wallet pairing");
  });

  it("blocks Read before exposing ABI function cards when V2 provenance is invalid", () => {
    const current = contractData();
    const html = renderToStaticMarkup(
      <ReadContract
        contractData={{ ...current, creationBlockHash: `0x${"f".repeat(64)}` }}
      />,
    );

    expect(html).toContain(
      'data-interaction-provenance-status="invalid-recorded"',
    );
    expect(html).toContain("Read interactions blocked");
    expect(html).not.toContain("sensitiveRead");
    expect(html).not.toContain(">Call<");
  });

  it("blocks Write before exposing wallet pairing or signing paths when V2 provenance is invalid", () => {
    const current = contractData();
    const html = renderToStaticMarkup(
      <WriteContract
        contractData={{ ...current, contractCodeSha256: "f".repeat(64) }}
      />,
    );

    expect(html).toContain(
      'data-interaction-provenance-status="invalid-recorded"',
    );
    expect(html).toContain("Write interactions blocked");
    expect(html).not.toContain("sensitiveWrite");
    expect(html).not.toContain("Wallet pairing");
    expect(html).not.toContain("Connect MyQRLWallet");
  });

  it("keeps legacy Read informational and blocks legacy Write", () => {
    const legacy = contractData({
      verificationRecordSchema: undefined,
      compilerProvenance: undefined,
      sourceBundleDigest: undefined,
      verificationArtifactDigest: undefined,
    });
    const readHTML = renderToStaticMarkup(
      <ReadContract contractData={legacy} />,
    );
    const writeHTML = renderToStaticMarkup(
      <WriteContract contractData={legacy} />,
    );

    expect(readHTML).toContain(
      'data-interaction-provenance-status="legacy-unrecorded"',
    );
    expect(readHTML).toContain("sensitiveRead");
    expect(readHTML).toContain("informational data");
    expect(writeHTML).toContain(
      'data-interaction-provenance-status="legacy-unrecorded"',
    );
    expect(writeHTML).toContain("Write interactions blocked");
    expect(writeHTML).not.toContain("sensitiveWrite");
    expect(writeHTML).not.toContain("Wallet pairing");
  });

  it("keeps V1 Read informational and blocks V1 Write", () => {
    const current = contractData();
    const {
      verificationArtifactDigest: _artifactDigest,
      ...withoutArtifactDigest
    } = current;
    const legacyV1: ContractData = {
      ...withoutArtifactDigest,
      verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V1,
    };
    const readHTML = renderToStaticMarkup(
      <ReadContract contractData={legacyV1} />,
    );
    const writeHTML = renderToStaticMarkup(
      <WriteContract contractData={legacyV1} />,
    );

    expect(readHTML).toContain("sensitiveRead");
    expect(readHTML).toContain("Legacy verification");
    expect(writeHTML).toContain("Write interactions blocked");
    expect(writeHTML).not.toContain("sensitiveWrite");
  });

  it("blocks Read and Write when V2 omits the artifact digest", () => {
    const incomplete = contractData({ verificationArtifactDigest: undefined });
    const readHTML = renderToStaticMarkup(
      <ReadContract contractData={incomplete} />,
    );
    const writeHTML = renderToStaticMarkup(
      <WriteContract contractData={incomplete} />,
    );

    expect(readHTML).toContain(
      'data-interaction-provenance-status="invalid-recorded"',
    );
    expect(readHTML).not.toContain("sensitiveRead");
    expect(writeHTML).toContain(
      'data-interaction-provenance-status="invalid-recorded"',
    );
    expect(writeHTML).not.toContain("sensitiveWrite");
  });

  it("blocks Read and Write for explicit null verification trust fields", () => {
    const explicitNull = contractData({
      verificationRecordSchema: null,
      compilerProvenance: null,
      sourceBundleDigest: null,
      verificationArtifactDigest: undefined,
    });
    const readHTML = renderToStaticMarkup(
      <ReadContract contractData={explicitNull} />,
    );
    const writeHTML = renderToStaticMarkup(
      <WriteContract contractData={explicitNull} />,
    );

    expect(readHTML).toContain(
      'data-interaction-provenance-status="invalid-recorded"',
    );
    expect(readHTML).not.toContain("sensitiveRead");
    expect(writeHTML).toContain(
      'data-interaction-provenance-status="invalid-recorded"',
    );
    expect(writeHTML).not.toContain("sensitiveWrite");
    expect(writeHTML).not.toContain("Wallet pairing");
  });
});
