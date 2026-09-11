import { describe, expect, it } from "@jest/globals";
import { renderToStaticMarkup } from "react-dom/server";

import {
  VERIFICATION_RECORD_SCHEMA_V1,
} from "../lib/storedVerification";
import { makeContractDataV2 } from "../lib/storedVerificationV2.fixture";
import VerifiedBadge from "./VerifiedBadge";

describe("VerifiedBadge", () => {
  it("renders green Verified only for a complete V2 artifact binding", () => {
    const html = renderToStaticMarkup(
      <VerifiedBadge record={makeContractDataV2()} />,
    );

    expect(html).toContain('data-verification-status="digest-backed"');
    expect(html).toContain("text-success");
    expect(html).toContain(">Verified<");
    expect(html).toContain("canonical deployment identity");
  });

  it("renders a complete V1 record as amber legacy verification", () => {
    const current = makeContractDataV2();
    const {
      verificationArtifactDigest: _artifactDigest,
      ...withoutArtifactDigest
    } = current;
    const html = renderToStaticMarkup(
      <VerifiedBadge
        record={{
          ...withoutArtifactDigest,
          verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V1,
        }}
      />,
    );

    expect(html).toContain('data-verification-status="legacy-unrecorded"');
    expect(html).toContain("text-warning");
    expect(html).toContain(">Legacy verified<");
    expect(html).toContain("deployment-bound artifact digest");
  });

  it("renders an absent historical marker as amber legacy verification", () => {
    const html = renderToStaticMarkup(<VerifiedBadge record={{}} />);

    expect(html).toContain('data-verification-status="legacy-unrecorded"');
    expect(html).toContain("text-warning");
  });

  it("renders a red invalid badge for inconsistent provenance", () => {
    const current = makeContractDataV2();
    const html = renderToStaticMarkup(
      <VerifiedBadge
        record={{ ...current, compilerVersion: "different-build" }}
      />,
    );

    expect(html).toContain('data-verification-status="invalid-recorded"');
    expect(html).toContain("text-error");
    expect(html).toContain(">Invalid verification record<");
  });

  it("renders a red invalid badge for an artifact target mismatch", () => {
    const current = makeContractDataV2();
    const html = renderToStaticMarkup(
      <VerifiedBadge
        record={{ ...current, creationBlockNumber: "0x11" }}
      />,
    );

    expect(html).toContain('data-verification-status="invalid-recorded"');
    expect(html).toContain("deployment artifact binding is invalid");
  });

  it("renders explicit null trust fields as an invalid record", () => {
    const html = renderToStaticMarkup(
      <VerifiedBadge
        record={{
          verificationRecordSchema: null,
          compilerProvenance: null,
          sourceBundleDigest: null,
          verificationArtifactDigest: null,
        }}
      />,
    );

    expect(html).toContain('data-verification-status="invalid-recorded"');
    expect(html).toContain("text-error");
  });
});
