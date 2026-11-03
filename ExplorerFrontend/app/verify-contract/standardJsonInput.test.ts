import { describe, expect, it } from '@jest/globals';

import {
  parseStandardJsonInput,
  primarySourceCandidates,
  standardJsonFormFields,
} from './standardJsonInput';

// Synthetic multi-directory project, written for these tests.
const treeInput = {
  language: 'Hyperion',
  sources: {
    'contracts/lib/MathLib.hyp': { content: 'library MathLib {}' },
    'contracts/core/Counter.hyp': { content: 'import "../lib/MathLib.hyp"; contract Counter {}' },
    'contracts/base/Base.hyp': { content: 'abstract contract Base {}' },
    'docs/abi.json': { content: '[]' },
  },
  settings: {
    viaIR: true,
    optimizer: { enabled: true, runs: 1000 },
    outputSelection: { '*': { '*': ['abi'] } },
  },
};

function parse(value: unknown) {
  return parseStandardJsonInput(JSON.stringify(value));
}

describe('parseStandardJsonInput', () => {
  it('reads sources in path order and the supported settings', () => {
    const result = parse(treeInput);
    if (!result.ok) throw new Error(result.error);
    expect(Object.keys(result.input.sources)).toEqual([
      'contracts/base/Base.hyp',
      'contracts/core/Counter.hyp',
      'contracts/lib/MathLib.hyp',
      'docs/abi.json',
    ]);
    expect(result.input.sources['contracts/core/Counter.hyp']).toBe(
      'import "../lib/MathLib.hyp"; contract Counter {}',
    );
    expect(result.input.viaIR).toBe(true);
    expect(result.input.optimizerEnabled).toBe(true);
    expect(result.input.optimizerRuns).toBe(1000);
    expect(result.input.warnings).toEqual([]);
  });

  it('defaults to the compiler defaults when settings are absent', () => {
    const result = parse({ sources: { 'A.hyp': { content: 'contract A {}' } } });
    if (!result.ok) throw new Error(result.error);
    expect(result.input).toMatchObject({ viaIR: false, optimizerEnabled: false, optimizerRuns: 200 });
  });

  it('warns about settings the verifier does not send', () => {
    const result = parse({
      ...treeInput,
      settings: {
        ...treeInput.settings,
        optimizer: { enabled: true, runs: 200, details: { yul: true } },
        metadata: { bytecodeHash: 'none' },
        qrvmVersion: 'other',
      },
    });
    if (!result.ok) throw new Error(result.error);
    expect(result.input.warnings).toEqual([
      'settings.optimizer.details is not sent to the verifier.',
      'settings.qrvmVersion "other" is not sent to the verifier; the compiler default applies.',
      'settings.metadata is not sent to the verifier.',
    ]);
  });

  it('accepts empty remappings and libraries', () => {
    const result = parse({ ...treeInput, settings: { remappings: [], libraries: {} } });
    expect(result.ok).toBe(true);
  });

  it('accepts the default VM version without a warning', () => {
    const result = parse({ ...treeInput, settings: { qrvmVersion: 'zond' } });
    if (!result.ok) throw new Error(result.error);
    expect(result.input.warnings).toEqual([]);
  });

  it.each([
    ['invalid JSON', null, 'not valid JSON'],
    ['an array', [], 'must be a JSON object'],
    ['another language', { ...treeInput, language: 'Solidity' }, 'must be "Hyperion"'],
    ['no sources', { ...treeInput, sources: {} }, 'has no sources'],
    ['URL-only sources', { sources: { 'A.hyp': { urls: ['ipfs://x'] } } }, 'no inline content'],
    ['remappings', { ...treeInput, settings: { remappings: ['@lib/=lib/'] } }, 'remappings are not supported'],
    ['linked libraries', { ...treeInput, settings: { libraries: { 'A.hyp': { L: 'Q1' } } } }, 'Linked libraries'],
    ['malformed remappings', { ...treeInput, settings: { remappings: '@lib/=lib/' } }, 'remappings are not supported'],
    ['a non-boolean viaIR', { ...treeInput, settings: { viaIR: 'yes' } }, 'settings.viaIR must be'],
    ['negative runs', { ...treeInput, settings: { optimizer: { runs: -1 } } }, 'non-negative integer'],
    ['a non-object settings value', { ...treeInput, settings: [] }, 'settings must be an object'],
  ] as const)('rejects %s', (_label, value, message) => {
    const result =
      value === null ? parseStandardJsonInput('{"sources":') : parse(value);
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toContain(message);
  });
});

describe('standardJsonFormFields', () => {
  it('lists only .hyp sources as primary candidates', () => {
    const result = parse(treeInput);
    if (!result.ok) throw new Error(result.error);
    expect(primarySourceCandidates(result.input)).toEqual([
      'contracts/base/Base.hyp',
      'contracts/core/Counter.hyp',
      'contracts/lib/MathLib.hyp',
    ]);
  });

  it('keeps the primary at its path and every other source as an import', () => {
    const result = parse(treeInput);
    if (!result.ok) throw new Error(result.error);
    expect(standardJsonFormFields(result.input, 'contracts/core/Counter.hyp')).toEqual({
      sourceCode: 'import "../lib/MathLib.hyp"; contract Counter {}',
      primarySourcePath: 'contracts/core/Counter.hyp',
      imports: {
        'contracts/base/Base.hyp': 'abstract contract Base {}',
        'contracts/lib/MathLib.hyp': 'library MathLib {}',
        'docs/abi.json': '[]',
      },
      contractNameSuggestion: 'Counter',
    });
  });

  it('suggests no contract name for a file name that is not an identifier', () => {
    const result = parse({ sources: { 'src/my-token.hyp': { content: 'contract T {}' } } });
    if (!result.ok) throw new Error(result.error);
    expect(standardJsonFormFields(result.input, 'src/my-token.hyp')?.contractNameSuggestion).toBe('');
  });

  it('returns null for a path outside the input', () => {
    const result = parse(treeInput);
    if (!result.ok) throw new Error(result.error);
    expect(standardJsonFormFields(result.input, 'contracts/core/Missing.hyp')).toBeNull();
    expect(standardJsonFormFields(result.input, 'toString')).toBeNull();
  });

  it('keeps a source named __proto__ as an own import', () => {
    const result = parseStandardJsonInput(
      '{"sources":{"A.hyp":{"content":"contract A {}"},"__proto__":{"content":"x"}}}',
    );
    if (!result.ok) throw new Error(result.error);
    const fields = standardJsonFormFields(result.input, 'A.hyp');
    expect(fields && Object.prototype.hasOwnProperty.call(fields.imports, '__proto__')).toBe(true);
    expect(JSON.stringify(fields?.imports)).toBe('{"__proto__":"x"}');
  });
});
