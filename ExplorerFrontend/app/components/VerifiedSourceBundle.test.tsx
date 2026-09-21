import { describe, expect, it } from "@jest/globals";
import { renderToStaticMarkup } from "react-dom/server";

import { VERIFICATION_RECORD_SCHEMA_V1 } from "../lib/storedVerification";
import { makeContractDataV2 } from "../lib/storedVerificationV2.fixture";
import type { ContractData } from "../types/address";
import VerifiedSourceBundle, {
  classifyVerifiedImports,
  orderedVerifiedImports,
} from "./VerifiedSourceBundle";

describe("VerifiedSourceBundle", () => {
  it("renders every canonical import filename and exact source in stable order", () => {
    const imports = {
      "src/zeta/Math.hyp": "library Math { function max() {} }",
      "src/alpha/Types.hyp": "struct Account { uint256 balance; }",
    };
    const html = renderToStaticMarkup(
      <VerifiedSourceBundle
        contractData={makeContractDataV2({
          sourceCode: "contract Main {}",
          imports,
        })}
      />,
    );

    expect(orderedVerifiedImports(imports)).toEqual([
      ["src/alpha/Types.hyp", imports["src/alpha/Types.hyp"]],
      ["src/zeta/Math.hyp", imports["src/zeta/Math.hyp"]],
    ]);
    expect(html).toContain(
      'data-source-bundle-status="digest-backed-multi-file"',
    );
    expect(html).toContain(
      "Verified source bundle includes 2 recorded import files.",
    );
    expect(html.indexOf("src/alpha/Types.hyp")).toBeLessThan(
      html.indexOf("src/zeta/Math.hyp"),
    );
    expect(html).toContain(imports["src/alpha/Types.hyp"]);
    expect(html).toContain(imports["src/zeta/Math.hyp"]);
    expect(html).toContain('aria-label="Copy import src/alpha/Types.hyp"');
    expect(html).toContain('aria-label="Copy import src/zeta/Math.hyp"');
  });

  it("labels an absent historical record as legacy", () => {
    const html = renderToStaticMarkup(
      <VerifiedSourceBundle
        contractData={makeContractDataV2({
          verificationRecordSchema: undefined,
          compilerProvenance: undefined,
          sourceBundleDigest: undefined,
          verificationArtifactDigest: undefined,
        })}
      />,
    );

    expect(html).toContain('data-source-bundle-status="legacy-unrecorded"');
    expect(html).toContain("lacks the current deployment-bound artifact");
    expect(html).not.toContain("Single-file verification");
    expect(html).not.toContain("Verified imported sources");
  });

  it("labels a complete V1 record as legacy", () => {
    const current = makeContractDataV2();
    const {
      verificationArtifactDigest: _artifactDigest,
      ...withoutArtifactDigest
    } = current;
    const contractData: ContractData = {
      ...withoutArtifactDigest,
      verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V1,
    };
    const html = renderToStaticMarkup(
      <VerifiedSourceBundle contractData={contractData} />,
    );

    expect(html).toContain('data-source-bundle-status="legacy-unrecorded"');
    expect(html).toContain("recorded source bundle remains available");
  });

  it("identifies a V2 record with an empty import map as single-file", () => {
    const html = renderToStaticMarkup(
      <VerifiedSourceBundle contractData={makeContractDataV2({ imports: {} })} />,
    );

    expect(html).toContain(
      'data-source-bundle-status="digest-backed-single-file"',
    );
    expect(html).toContain("Single-file verification: no imports recorded.");
    expect(html).not.toContain("legacy-unrecorded");
  });

  it.each([
    ["an array", ["contract NotAMap {}"]],
    ["null", null],
    ["a non-string member", { "src/Bad.hyp": 7 }],
  ])("fails closed for malformed imports stored as %s", (_label, malformed) => {
    const contractData = makeContractDataV2({
      imports: malformed as unknown as Record<string, string>,
      verificationArtifactDigest: undefined,
    });
    const html = renderToStaticMarkup(
      <VerifiedSourceBundle contractData={contractData} />,
    );

    expect(classifyVerifiedImports(malformed).valid).toBe(false);
    expect(html).toContain('data-source-bundle-status="invalid-recorded"');
    expect(html).not.toContain("digest-backed-single-file");
    expect(html).not.toContain("Verified imported sources");
  });

  it("fails closed when a V2 compiler record omits the artifact digest", () => {
    const html = renderToStaticMarkup(
      <VerifiedSourceBundle
        contractData={makeContractDataV2({
          verificationArtifactDigest: undefined,
        })}
      />,
    );

    expect(html).toContain('data-source-bundle-status="invalid-recorded"');
    expect(html).not.toContain("Single-file verification");
  });

  it("labels imports as recorded when the source-bundle digest mismatches", () => {
    const imports = { "src/Dependency.hyp": "contract Dependency {}" };
    const contractData = makeContractDataV2({
      imports,
      sourceBundleDigest: `qrl.verified-source-bundle.v1:sha256:${"c".repeat(64)}`,
      verificationArtifactDigest: undefined,
    });
    const html = renderToStaticMarkup(
      <VerifiedSourceBundle contractData={contractData} />,
    );

    expect(html).toContain('data-source-bundle-status="invalid-recorded"');
    expect(html).toContain('aria-label="Recorded imported sources"');
    expect(html).toContain("Recorded imports");
    expect(html).not.toContain('aria-label="Verified imported sources"');
  });
});
