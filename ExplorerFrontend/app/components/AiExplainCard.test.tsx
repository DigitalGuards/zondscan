import { describe, expect, it } from "@jest/globals";
import { renderToStaticMarkup } from "react-dom/server";

jest.mock("../lib/qrlConnect", () => ({
  getQrlConnect: () => ({
    getAccounts: () => [],
    on: () => undefined,
    off: () => undefined,
  }),
}));

import type { ContractData } from "../types/address";
import { sourceBundleDigestV1 } from "../lib/storedVerification";
import { makeContractDataV2 } from "../lib/storedVerificationV2.fixture";
import AiExplainCard, { aiExplainRecordIdentity } from "./AiExplainCard";

const SOURCE = "contract Main {}";
const SOURCE_DIGEST = sourceBundleDigestV1("Main", SOURCE, {});

function contractData(overrides: Partial<ContractData> = {}): ContractData {
  return makeContractDataV2({
    creatorAddressProvenance: "direct-deployment",
    ...overrides,
  });
}

describe("AiExplainCard provenance gate", () => {
  it("offers generation only for a digest-backed current record", () => {
    const html = renderToStaticMarkup(
      <AiExplainCard contractData={contractData()} />,
    );

    expect(html).toContain("Explain with AI");
    expect(html).not.toContain("data-ai-explain-status");
  });

  it("serves only cache content bound to the exact source digest", () => {
    const trusted = renderToStaticMarkup(
      <AiExplainCard
        contractData={contractData({
          aiExplanation: "Trusted explanation",
          aiExplanationSourceDigest: SOURCE_DIGEST,
        })}
      />,
    );
    const stale = renderToStaticMarkup(
      <AiExplainCard
        contractData={contractData({
          aiExplanation: "Stale explanation",
          aiExplanationSourceDigest: `qrl.verified-source-bundle.v1:sha256:${"c".repeat(64)}`,
        })}
      />,
    );

    expect(trusted).toContain("Trusted explanation");
    expect(stale).not.toContain("Stale explanation");
    expect(stale).toContain("Explain with AI");
  });

  it("hides cache and generation for legacy verification records", () => {
    const html = renderToStaticMarkup(
      <AiExplainCard
        contractData={contractData({
          verificationRecordSchema: undefined,
          compilerProvenance: undefined,
          sourceBundleDigest: undefined,
          verificationArtifactDigest: undefined,
          aiExplanation: "Unbound legacy explanation",
        })}
      />,
    );

    expect(html).toContain('data-ai-explain-status="legacy-unrecorded"');
    expect(html).toContain("AI explanation unavailable");
    expect(html).not.toContain("Unbound legacy explanation");
    expect(html).not.toContain("Explain with AI");
  });

  it("hides cache and generation for invalid verification records", () => {
    const html = renderToStaticMarkup(
      <AiExplainCard
        contractData={contractData({
          sourceBundleDigest: undefined,
          aiExplanation: "Invalid explanation",
        })}
      />,
    );

    expect(html).toContain('data-ai-explain-status="invalid-recorded"');
    expect(html).toContain("deployment provenance is invalid");
    expect(html).not.toContain("Invalid explanation");
    expect(html).not.toContain("Explain with AI");
  });

  it("changes the remount key when the source identity changes", () => {
    const first = contractData({
      aiExplanation: "First explanation",
      aiExplanationSourceDigest: SOURCE_DIGEST,
    });
    const secondSource = "contract Second {}";
    const secondDigest = sourceBundleDigestV1("Second", secondSource, {});
    const second = contractData({
      contractName: "Second",
      sourceCode: secondSource,
      sourceBundleDigest: secondDigest,
      aiExplanation: "Second explanation",
      aiExplanationSourceDigest: secondDigest,
    });
    expect(aiExplainRecordIdentity(first)).not.toBe(
      aiExplainRecordIdentity(second),
    );
    expect(
      renderToStaticMarkup(<AiExplainCard contractData={first} />),
    ).toContain("First explanation");
    const secondHTML = renderToStaticMarkup(
      <AiExplainCard contractData={second} />,
    );
    expect(secondHTML).not.toContain("First explanation");
    expect(secondHTML).toContain("Second explanation");
  });

  it("does not offer creator regeneration for heuristic identity", () => {
    const html = renderToStaticMarkup(
      <AiExplainCard
        contractData={contractData({
          creatorAddressProvenance: "mint-heuristic",
          aiExplanation: "Cached explanation",
          aiExplanationSourceDigest: SOURCE_DIGEST,
        })}
      />,
    );
    expect(html).toContain("Creator authorization requires deployment evidence.");
    expect(html).not.toContain(">Regenerate<");
  });
});
