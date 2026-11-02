import { describe, expect, it } from '@jest/globals';

import { buildVerifyRequestBody, type VerifyFormValues } from './verifyRequest';

const singleFile: VerifyFormValues = {
  address: ` Q${'a'.repeat(128)} `,
  contractName: ' Counter ',
  sourceCode: 'contract Counter {}',
  primarySourcePath: '',
  importsText: '',
  selectedBuild: 'build-1',
  optimizerEnabled: true,
  optimizerRuns: 0,
  viaIR: false,
  evmVersion: '',
  constructorArgs: '',
  license: 'MIT',
};

describe('buildVerifyRequestBody', () => {
  it('sends the unchanged single-file body when no tree or via-IR is set', () => {
    const result = buildVerifyRequestBody(singleFile);
    if (!result.ok) throw new Error(result.error);
    expect(JSON.parse(JSON.stringify(result.body))).toEqual({
      address: `Q${'a'.repeat(128)}`,
      sourceCode: 'contract Counter {}',
      contractName: 'Counter',
      compilerVersion: 'build-1',
      optimizerEnabled: true,
      optimizerRuns: 0,
      license: 'MIT',
    });
  });

  it('sends the primary path, the source tree, and via-IR', () => {
    const imports = {
      'contracts/lib/MathLib.hyp': 'library MathLib {}',
      'contracts/base/Base.hyp': 'abstract contract Base {}',
    };
    const result = buildVerifyRequestBody({
      ...singleFile,
      primarySourcePath: ' contracts/core/Counter.hyp ',
      importsText: JSON.stringify(imports),
      viaIR: true,
    });
    if (!result.ok) throw new Error(result.error);
    expect(result.body.primarySourcePath).toBe('contracts/core/Counter.hyp');
    expect(result.body.viaIR).toBe(true);
    expect(result.body.imports).toEqual(imports);
  });

  it.each([
    ['not JSON', '{', 'not valid JSON'],
    ['not an object of strings', '{"a.hyp": 1}', 'must be a JSON object'],
    ['an array', '["a.hyp"]', 'must be a JSON object'],
  ])('rejects imports that are %s', (_label, importsText, message) => {
    const result = buildVerifyRequestBody({ ...singleFile, importsText });
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toContain(message);
  });
});
