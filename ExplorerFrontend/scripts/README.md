# ExplorerFrontend build scripts

## `build-dapp-example.sh`

Stages the [QRL Connect](https://github.com/DigitalGuards/myqrlwallet-connect) test dApp into `public/dapp-example/` so ZondScan serves it as a static SPA at [`/dapp-example`](https://zondscan.com/dapp-example). Invoked automatically via the `prebuild` npm hook, so every `npm run build` (and by extension every `update-frontend.sh` / PM2 restart) refreshes the hosted example against the latest connect repo.

### What @qrlwallet/connect is

`@qrlwallet/connect` is the self-hosted, end-to-end encrypted protocol MyQRLWallet uses to pair a dApp with the mobile wallet — the QRL equivalent of WalletConnect, but tailored to Q-addresses and designed for an eventual post-quantum transport migration.

```
┌─────────────────────┐                  ┌──────────────────────────┐
│  External dApp      │    Socket.IO     │  MyQRLWallet App         │
│  @qrlwallet/connect │  (E2E encrypted) │  (WebView handles ECIES  │
│  → QR / deep link   │ <=============> │   + approval UI)          │
└─────────┬───────────┘                  └──────────────────────────┘
          │
   ┌──────▼──────┐
   │ Relay Server │  stateless Socket.IO router (qrlwallet.com)
   └──────────────┘
```

The hosted `/dapp-example` is a live demo of that protocol: a dApp that generates a QR code, walks through the SYN/SYNACK/ACK handshake, and exercises `qrl_sendTransaction` / `personal_sign` / read-only RPC calls — all while streaming every event to an on-page log so developers can see the wire format. Deep architecture lives in the connect repo's [`CLAUDE.md`](https://github.com/DigitalGuards/myqrlwallet-connect/blob/dev/CLAUDE.md) and [`example/README.md`](https://github.com/DigitalGuards/myqrlwallet-connect/blob/dev/example/README.md).

### What the script does

1. Clones (or fetches) `DigitalGuards/myqrlwallet-connect` into `.dapp-example-cache/` (gitignored).
2. `npm install && npm run build` in the SDK root — tsup emits the `dist/` that `example/` consumes via `file:..`.
3. `npm install && npx vite build --base=/dapp-example/` in `example/` — rewrites asset paths so `/dapp-example/assets/*.js` resolves correctly when mounted under ZondScan.
4. Copies `example/dist/*` into `ExplorerFrontend/public/dapp-example/`.

A Next.js rewrite in `next.config.js` maps `/dapp-example` (no trailing slash) to `/dapp-example/index.html` so the directory request doesn't 404.

### Env overrides

| Variable              | Default                                                       | Purpose                                    |
| --------------------- | ------------------------------------------------------------- | ------------------------------------------ |
| `QRL_CONNECT_REPO`    | `https://github.com/DigitalGuards/myqrlwallet-connect.git`    | Alternate remote.                          |
| `QRL_CONNECT_REF`     | `dev`                                                         | Branch or tag to clone. Pin to a tag for stable deploys. |
| `QRL_CONNECT_LOCAL`   | _(unset)_                                                     | Absolute path to a local connect checkout. Rsyncs instead of cloning — useful when developing against uncommitted SDK changes. |
| `SKIP_DAPP_EXAMPLE`   | `0`                                                           | Set to `1` to bypass entirely (fast local builds). |

### Examples

```bash
# Pin to a specific SDK release tag
QRL_CONNECT_REF=v0.2.0 npm run build

# Build against a local connect checkout (no git required)
QRL_CONNECT_LOCAL=/home/you/myqrlwallet-connect npm run build

# Skip during a fast iteration loop
SKIP_DAPP_EXAMPLE=1 npm run build
```

### Troubleshooting

- **Vite build fails with `Rollup failed to resolve "vite-plugin-node-polyfills/shims/buffer"`** — you're on an old connect revision predating [connect#5](https://github.com/DigitalGuards/myqrlwallet-connect/pull/5). Bump `QRL_CONNECT_REF` or update the local checkout.
- **`/dapp-example` returns 404 in dev** — `next dev` doesn't re-run `prebuild`. Either run `npm run build` once first, or bounce PM2 against a fresh production build.
- **Stale bundle after connect changes** — the cache dir is retained between runs. `rm -rf .dapp-example-cache` to force a clean clone.
