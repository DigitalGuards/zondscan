import nextConfig from "eslint-config-next";
import coreWebVitals from "eslint-config-next/core-web-vitals";
import tsConfig from "eslint-config-next/typescript";

/** @type {import('eslint').Linter.Config[]} */
export default [
  ...nextConfig,
  ...coreWebVitals,
  ...tsConfig,
  {
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
      "@typescript-eslint/no-require-requires": "off",

      // General
      "prefer-const": "warn",
      "import/no-anonymous-default-export": "warn",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
  {
    ignores: ["build/", "node_modules/", ".next/"],
  },
];
