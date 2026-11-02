import { isStringRecord } from '../lib/guards';

/** Raw verify-form values, exactly as the inputs hold them. */
export interface VerifyFormValues {
  address: string;
  contractName: string;
  sourceCode: string;
  primarySourcePath: string;
  importsText: string;
  selectedBuild: string;
  optimizerEnabled: boolean;
  optimizerRuns: number;
  viaIR: boolean;
  evmVersion: string;
  constructorArgs: string;
  license: string;
}

/** JSON body for POST /contract/verify. Undefined fields are omitted. */
export interface VerifyRequestBody {
  address: string;
  sourceCode: string;
  contractName: string;
  primarySourcePath?: string;
  compilerVersion?: string;
  optimizerEnabled: boolean;
  optimizerRuns: number;
  viaIR?: true;
  evmVersion?: string;
  constructorArguments?: string;
  imports?: Record<string, string>;
  license?: string;
}

export type VerifyRequestBuild =
  | { ok: true; body: VerifyRequestBody }
  | { ok: false; error: string };

/**
 * Builds the verify request from the form. A single-file submission without
 * a primary path or via-IR produces the same body the form always sent; the
 * new fields are only present when the user sets them.
 */
export function buildVerifyRequestBody(values: VerifyFormValues): VerifyRequestBuild {
  let imports: Record<string, string> | undefined;
  if (values.importsText.trim()) {
    try {
      const parsed: unknown = JSON.parse(values.importsText);
      if (!isStringRecord(parsed)) {
        return {
          ok: false,
          error: 'Imports must be a JSON object: { "path/to/File.hyp": "<source>" }',
        };
      }
      imports = parsed;
    } catch (e) {
      return {
        ok: false,
        error: 'Imports field is not valid JSON: ' + (e instanceof Error ? e.message : String(e)),
      };
    }
  }

  return {
    ok: true,
    body: {
      address: values.address.trim(),
      sourceCode: values.sourceCode,
      contractName: values.contractName.trim(),
      primarySourcePath: values.primarySourcePath.trim() || undefined,
      compilerVersion: values.selectedBuild || undefined,
      optimizerEnabled: values.optimizerEnabled,
      // Pass the raw optimizerRuns through, 0 is a legitimate configuration
      // distinct from the default, so don't fall back via `|| 200`.
      optimizerRuns: values.optimizerRuns,
      viaIR: values.viaIR ? true : undefined,
      evmVersion: values.evmVersion.trim() || undefined,
      constructorArguments: values.constructorArgs.trim() || undefined,
      imports,
      license: values.license || undefined,
    },
  };
}
