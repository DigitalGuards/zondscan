# Dependency security

Reviewed against npm registry releases and audit data on 2026-10-09.

| Package                                 | Locked version | Update                                                                    |
| --------------------------------------- | -------------- | ------------------------------------------------------------------------- |
| `next`, `eslint-config-next`, `@next/*` | 16.3.8         | Security fixes; Next and its ESLint config are pinned together.           |
| `sharp`                                 | 0.35.5         | Native packages and `@img/sharp-libvips-*` 1.3.4 include the librsvg fix. |
| `axios`                                 | 1.20.0         | Request handling security fixes.                                          |
| `postcss`                               | 8.5.29         | Parent release requires patched `source-map-js` 1.2.2.                    |
| `source-map-js`                         | 1.2.2          | Shared patched version for PostCSS and Tailwind.                          |
| `brace-expansion`                       | 1.1.21         | Compatible lockfile update through the existing minimatch range.          |

Application source has no `next/og`, `@vercel/og`, or `ImageResponse` imports.
`next/image` is used for NFT images through the remote patterns in `next.config.js`.
The image optimizer remains enabled with the existing allowlist.

`eslint-config-next` is used by `eslint.config.mjs` and belongs in
`devDependencies`. Moving it there places its linting dependency tree in the
development scope. The build still requires development dependencies for linting
and type checking. No new dependency overrides were needed.

Next 16.3.8 regenerates `next-env.d.ts` to import its generated root-parameter
types. This is the only declaration change required by the upgrade.

`npm audit --omit=dev` reports zero vulnerabilities. The full `npm audit` reports
27 affected development packages (8 high, 19 moderate), propagated from these
two unresolved advisories:

- `braces` 3.0.3: [GHSA-vfj7-8cjw-p6xm](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm),
  stack exhaustion from deeply nested patterns. npm has no patched release.
  Paths include the Next ESLint plugin through `fast-glob` and `micromatch`, and
  Jest types through `expect`, `jest-message-util`, and `micromatch`.
- `sprintf-js` 1.0.3: [GHSA-hp3w-g68c-fv3c](https://github.com/advisories/GHSA-hp3w-g68c-fv3c),
  denial of service from unbounded precision specifiers. npm has no patched
  release; the latest 1.1.3 is also affected. It is used by the Jest coverage
  tooling through `@istanbuljs/load-nyc-config`, `js-yaml`, and `argparse`.

Both packages are marked development-only in the lockfile. Their upstream fixes
remain pending. npm's suggested major downgrades of Next lint tooling and Jest
fall outside this security update's supported versions.
