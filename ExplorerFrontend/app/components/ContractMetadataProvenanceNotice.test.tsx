import { describe, expect, it } from "@jest/globals";
import { renderToStaticMarkup } from "react-dom/server";

import type { ContractMeta } from "../types/transaction";
import ContractMetadataProvenanceNotice, {
  classifyContractMetadata,
  trustedContractMetadataABI,
} from "./ContractMetadataProvenanceNotice";

const ABI = '[{"type":"function","name":"read"}]';

describe("compact contract metadata provenance", () => {
  it("uses current digest-backed ABI without a warning", () => {
    const contract: ContractMeta = {
      verified: true,
      abi: ABI,
      provenanceStatus: "digest-backed",
    };

    expect(classifyContractMetadata(contract)).toBe("digest-backed");
    expect(trustedContractMetadataABI(contract)).toBe(ABI);
    expect(
      renderToStaticMarkup(
        <ContractMetadataProvenanceNotice contract={contract} />,
      ),
    ).toBe("");
  });

  it("allows legacy ABI with an explicit amber warning", () => {
    const contract: ContractMeta = { verified: true, abi: ABI };
    const html = renderToStaticMarkup(
      <ContractMetadataProvenanceNotice contract={contract} />,
    );

    expect(classifyContractMetadata(contract)).toBe("legacy-unrecorded");
    expect(trustedContractMetadataABI(contract)).toBe(ABI);
    expect(html).toContain(
      'data-contract-metadata-provenance-status="legacy-unrecorded"',
    );
    expect(html).toContain("Decoding is informational");
  });

  it.each(["invalid-recorded", "future-unknown"])(
    "blocks ABI and renders a red warning for %s",
    (provenanceStatus) => {
      const contract: ContractMeta = {
        verified: true,
        abi: ABI,
        provenanceStatus,
      };
      const html = renderToStaticMarkup(
        <ContractMetadataProvenanceNotice contract={contract} />,
      );

      expect(classifyContractMetadata(contract)).toBe("invalid-recorded");
      expect(trustedContractMetadataABI(contract)).toBeUndefined();
      expect(html).toContain(
        'data-contract-metadata-provenance-status="invalid-recorded"',
      );
      expect(html).toContain("Stored ABI blocked");
    },
  );

  it("does not trust or warn for unverified metadata", () => {
    const contract: ContractMeta = { verified: false, abi: ABI };

    expect(classifyContractMetadata(contract)).toBeNull();
    expect(trustedContractMetadataABI(contract)).toBeUndefined();
    expect(
      renderToStaticMarkup(
        <ContractMetadataProvenanceNotice contract={contract} />,
      ),
    ).toBe("");
  });
});
