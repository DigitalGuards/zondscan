// Jest config for ExplorerFrontend. Uses next/jest so SWC handles TS +
// JSX transpilation with the same toolchain the dev server uses,
// avoiding TS/Babel drift. Tests run in node by default since the
// decoder tests don't touch the DOM; widen to jsdom on a per-file
// basis with `// @jest-environment jsdom` if any later tests need it.
//
// ESM (.mjs) so the project's lint rule against `require()`-style
// imports doesn't fire on the config file.
import nextJest from 'next/jest.js';

const createJestConfig = nextJest({
  // Path to the Next app, used by next/jest to load next.config.js + .env.
  dir: './',
});

/** @type {import('jest').Config} */
const customConfig = {
  testEnvironment: 'node',
  testMatch: ['**/*.test.{ts,tsx,js,jsx}'],
  // Skip:
  // - .dapp-example-cache: the dapp-example submodule prebuild artefact,
  //   which carries Vitest tests that use `vi` (incompatible with Jest).
  //   We don't run those here, they're owned by myqrlwallet-connect.
  // - build / dev artefacts and node_modules (node_modules excluded by
  //   default but kept explicit for clarity).
  testPathIgnorePatterns: ['/node_modules/', '/.dapp-example-cache/', '/.next/', '/build/'],
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json'],
};

export default createJestConfig(customConfig);
