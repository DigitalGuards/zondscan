import nextConfig from "eslint-config-next";
import coreWebVitals from "eslint-config-next/core-web-vitals";
import tsConfig from "eslint-config-next/typescript";

/** @type {import('eslint').Linter.Config[]} */
export default [
  ...nextConfig,
  ...coreWebVitals,
  ...tsConfig,
  {
    // Same file glob as the upstream `next` config object so the `import`
    // plugin (declared by next at the same scope) resolves here too.
    // Without an explicit `files`, eslint v9+ flat config treats this entry
    // as global and refuses to start with:
    //   "could not find plugin 'import'"
    // because the plugin isn't in scope for files outside the next glob.
    files: ["**/*.{js,jsx,mjs,ts,tsx,mts,cts}"],
    settings: {
      // Pin React version to bypass eslint-plugin-react@7.37's
      // detectReactVersion() helper, which calls context.getFilename() — a
      // method ESLint 10 removed. Without this pin, every load of a
      // react/* rule crashes:
      //   "Error while loading rule 'react/no-direct-mutation-state':
      //    contextOrFilename.getFilename is not a function"
      // We're on React 19 (see package.json), so this is honest, not a
      // workaround value.
      react: { version: "19" },
    },
    rules: {
      // React 19 doesn't need import React
      "react/react-in-jsx-scope": "off",
      "react/prop-types": "off",
      "react/display-name": "off",

      // TypeScript
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      "@typescript-eslint/ban-ts-comment": "warn",
      "@typescript-eslint/no-var-requires": "off",

      // General
      "prefer-const": "warn",
      "import/no-anonymous-default-export": "warn",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
  {
    ignores: [
      "build/",
      "node_modules/",
      ".next/",
      ".dapp-example-cache/",
      "public/dapp-example/",
    ],
  },
];
