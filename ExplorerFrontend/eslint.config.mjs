import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import coreWebVitals from 'eslint-config-next/core-web-vitals';
import tseslint from 'typescript-eslint';

const typeScriptFiles = ['**/*.{ts,tsx,mts,cts}'];
const ratchet = JSON.parse(readFileSync(new URL('./eslint-ratchet.json', import.meta.url), 'utf8'));

export default [
  {
    ignores: [
      'build/**',
      'node_modules/**',
      '.next/**',
      '.dapp-example-cache/**',
      'public/dapp-example/**',
    ],
    linterOptions: { noInlineConfig: true, reportUnusedDisableDirectives: 'error' },
  },
  ...coreWebVitals,
  ...tseslint.configs.strictTypeChecked.map((config) => ({ ...config, files: typeScriptFiles })),
  {
    files: ['**/*.{js,jsx,mjs,ts,tsx,mts,cts}'],
    // ESLint 10 requires an explicit version with the installed React plugin.
    settings: { react: { version: '19' } },
    rules: {
      'react/react-in-jsx-scope': 'off',
      'react/prop-types': 'off',
      'react/display-name': 'off',
      'react-hooks/exhaustive-deps': 'error',
      'import/no-anonymous-default-export': 'error',
    },
  },
  {
    files: typeScriptFiles,
    languageOptions: {
      parserOptions: {
        project: './tsconfig.json',
        tsconfigRootDir: fileURLToPath(new URL('.', import.meta.url)),
      },
    },
    rules: {
      '@typescript-eslint/consistent-type-assertions': ['error', { assertionStyle: 'never' }],
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-non-null-assertion': 'error',
      '@typescript-eslint/ban-ts-comment': [
        'error',
        {
          'ts-ignore': true,
          'ts-expect-error': true,
          'ts-nocheck': true,
          'ts-check': true,
        },
      ],
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],
      'no-restricted-syntax': [
        'error',
        {
          selector: 'CallExpression[callee.object.name="Array"][callee.property.name="isArray"]',
          message: 'Use the isArray guard to keep untrusted array elements unknown.',
        },
        {
          selector:
            'CallExpression[callee.object.name="axios"][callee.property.name=/^(get|post|put|patch|delete|request|head|options)$/]:not([typeArguments.params.0.type="TSUnknownKeyword"])',
          message: 'Receive HTTP response data as unknown and validate it with runtime guards.',
        },
      ],
    },
  },
  {
    files: ['app/lib/guards.ts'],
    // The shared predicate owns the built-in array check and exposes unknown elements.
    rules: { 'no-restricted-syntax': 'off' },
  },
  {
    files: ['**/*.test.{ts,tsx}', '**/*.fixture.ts', 'e2e/**/*.ts'],
    // Malformed fixtures and mocks may describe values outside production types.
    rules: {
      '@typescript-eslint/consistent-type-assertions': 'off',
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-non-null-assertion': 'off',
      'no-restricted-syntax': 'off',
    },
  },
  ...(process.env.ESLINT_RATCHET === 'off'
    ? []
    : Object.entries(ratchet).map(([file, rules]) => ({
        files: [file.replaceAll('[', '[[]')],
        // Counts are checked by scripts/lint.mjs, including growth inside legacy files.
        rules: Object.fromEntries(Object.keys(rules).map((rule) => [rule, 'warn'])),
      }))),
];
