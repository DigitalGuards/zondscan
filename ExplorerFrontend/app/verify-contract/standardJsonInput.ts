import { isArray, isRecord } from '../lib/guards';

/**
 * Helpers that turn a Hyperion standard JSON input (the file a deploy script
 * passes to `hypc --standard-json`) into verify-form fields. The keys of its
 * `sources` map are the exact source unit names the deployer compiled, so
 * loading them keeps every path, and with it the metadata hash, identical.
 *
 * The backend validates every path again; these helpers only reject inputs
 * the verify request cannot express.
 */

const HYPERION_IDENTIFIER = /^[A-Za-z_$][A-Za-z0-9_$]*$/;
const DEFAULT_OPTIMIZER_RUNS = 200;
const DEFAULT_QRVM_VERSION = 'zond';

export interface StandardJsonInput {
  /** Source unit name to exact content, ordered by path. */
  sources: Record<string, string>;
  optimizerEnabled: boolean;
  optimizerRuns: number;
  viaIR: boolean;
  /** Settings the verifier does not send to hypc. */
  warnings: string[];
}

export type StandardJsonParseResult =
  | { ok: true; input: StandardJsonInput }
  | { ok: false; error: string };

export interface StandardJsonFormFields {
  sourceCode: string;
  primarySourcePath: string;
  /** Every source except the primary, keyed by its path. */
  imports: Record<string, string>;
  /** Contract name suggested by the primary file name, or empty. */
  contractNameSuggestion: string;
}

export function parseStandardJsonInput(text: string): StandardJsonParseResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch (e) {
    return {
      ok: false,
      error: `Standard JSON input is not valid JSON: ${e instanceof Error ? e.message : String(e)}`,
    };
  }
  if (!isRecord(parsed)) {
    return { ok: false, error: 'Standard JSON input must be a JSON object.' };
  }
  if (parsed.language !== undefined && parsed.language !== 'Hyperion') {
    return {
      ok: false,
      error: `Standard JSON language must be "Hyperion", found ${JSON.stringify(parsed.language)}.`,
    };
  }

  if (!isRecord(parsed.sources) || Object.keys(parsed.sources).length === 0) {
    return { ok: false, error: 'Standard JSON input has no sources.' };
  }
  // Object.fromEntries defines own properties, so a source named
  // "__proto__" stays a source.
  const entries: Array<[string, string]> = [];
  for (const [path, entry] of Object.entries(parsed.sources).sort(([a], [b]) =>
    a < b ? -1 : a > b ? 1 : 0,
  )) {
    if (!isRecord(entry) || typeof entry.content !== 'string') {
      return {
        ok: false,
        error: `Source "${path}" has no inline content. Sources given only by URL are not supported.`,
      };
    }
    entries.push([path, entry.content]);
  }
  const sources: Record<string, string> = Object.fromEntries(entries);

  const warnings: string[] = [];
  const settings = parsed.settings === undefined ? {} : parsed.settings;
  if (!isRecord(settings)) {
    return { ok: false, error: 'Standard JSON settings must be an object.' };
  }

  let optimizerEnabled = false;
  let optimizerRuns = DEFAULT_OPTIMIZER_RUNS;
  if (settings.optimizer !== undefined) {
    if (!isRecord(settings.optimizer)) {
      return { ok: false, error: 'settings.optimizer must be an object.' };
    }
    const { enabled, runs, ...rest } = settings.optimizer;
    if (enabled !== undefined && typeof enabled !== 'boolean') {
      return { ok: false, error: 'settings.optimizer.enabled must be true or false.' };
    }
    if (runs !== undefined && !(typeof runs === 'number' && Number.isSafeInteger(runs) && runs >= 0)) {
      return { ok: false, error: 'settings.optimizer.runs must be a non-negative integer.' };
    }
    optimizerEnabled = enabled === true;
    if (typeof runs === 'number') optimizerRuns = runs;
    for (const key of Object.keys(rest).sort()) {
      warnings.push(`settings.optimizer.${key} is not sent to the verifier.`);
    }
  }

  let viaIR = false;
  if (settings.viaIR !== undefined) {
    if (typeof settings.viaIR !== 'boolean') {
      return { ok: false, error: 'settings.viaIR must be true or false.' };
    }
    viaIR = settings.viaIR;
  }

  if (
    settings.remappings !== undefined &&
    !(isArray(settings.remappings) && settings.remappings.length === 0)
  ) {
    return {
      ok: false,
      error:
        'Import remappings are not supported. List each source under the path its imports resolve to.',
    };
  }
  if (
    settings.libraries !== undefined &&
    !(isRecord(settings.libraries) && Object.keys(settings.libraries).length === 0)
  ) {
    return { ok: false, error: 'Linked libraries in settings.libraries are not supported by this form.' };
  }
  if (settings.qrvmVersion !== undefined && settings.qrvmVersion !== DEFAULT_QRVM_VERSION) {
    warnings.push(
      `settings.qrvmVersion ${JSON.stringify(settings.qrvmVersion)} is not sent to the verifier; the compiler default applies.`,
    );
  }

  const handled = new Set([
    'optimizer',
    'viaIR',
    'remappings',
    'libraries',
    'qrvmVersion',
    'outputSelection',
  ]);
  for (const key of Object.keys(settings).sort()) {
    if (!handled.has(key)) warnings.push(`settings.${key} is not sent to the verifier.`);
  }

  return { ok: true, input: { sources, optimizerEnabled, optimizerRuns, viaIR, warnings } };
}

/** Source paths that can be the primary source: every `.hyp` unit. */
export function primarySourceCandidates(input: StandardJsonInput): string[] {
  return Object.keys(input.sources).filter((path) => path.endsWith('.hyp'));
}

export function standardJsonFormFields(
  input: StandardJsonInput,
  primarySourcePath: string,
): StandardJsonFormFields | null {
  if (!Object.prototype.hasOwnProperty.call(input.sources, primarySourcePath)) return null;
  const sourceCode = input.sources[primarySourcePath];
  const imports: Record<string, string> = Object.fromEntries(
    Object.entries(input.sources).filter(([path]) => path !== primarySourcePath),
  );

  const fileName = primarySourcePath.slice(primarySourcePath.lastIndexOf('/') + 1);
  const baseName = fileName.endsWith('.hyp') ? fileName.slice(0, -'.hyp'.length) : '';
  return {
    sourceCode,
    primarySourcePath,
    imports,
    contractNameSuggestion: HYPERION_IDENTIFIER.test(baseName) ? baseName : '',
  };
}
