# Horizontal explorer UI

ZondScan keeps the top market/search bar sticky while the logo and horizontal navigation scroll with the page. Desktop dropdowns open on hover and support clicks, touch activation, and keyboard navigation. Blockchain, Consensus, Tools, Resources, and Ecosystem menus group existing destinations. Smaller screens expand navigation inline below the logo, with compact accordion rows and direct settings, appearance, and network controls. The page remains scrollable below the menu. Escape and selecting a mobile menu link return focus to the toggle. Desktop hover preserves a menu being used with the keyboard. The header search appears when a page has no visible search field, including after scrolling past the homepage search. Ctrl+K or Cmd+K focuses the visible field.

Appearance supports Light, Dim, Dark, and Auto (system). English, Simplified Chinese, Spanish, and Russian cover navigation, settings, common explorer labels, and date formatting; guides and contract-provided content retain their original language. Display currency supports USD, EUR, GBP, CHF, CAD, AUD, JPY, and CNY. QRL market price, market cap, transaction fees, and gas estimates use validated ECB reference rates through Frankfurter, refreshed hourly. Conversion failures explicitly display USD; exchange trading pairs retain their quote currency. `/settings` also controls shortened address display, UTC or local timestamps, matching-address highlighting, technical-panel expansion, and hiding zero-quantity QRC-20 transfers. Preferences apply immediately, persist in browser storage, and synchronize between tabs. Storage failures fall back to the current tab. Hidden transfers include a notice and a control to show them; NFT transfers and native transactions remain visible. Filtering applies to the records loaded on a page, while totals retain their API meaning.

The network menu identifies QRL Testnet v2 as the current network and shows disabled entries for upcoming Testnet v3 and future mainnet support. This change preserves the existing address validation and network connections. It contains no 64-byte address migration work.

## Local preview with live data

From `ExplorerFrontend`, install dependencies with `npm ci`, then run:

```sh
HANDLER_URL=https://zondscan.com/api NEXT_PUBLIC_HANDLER_URL=https://zondscan.com/api npm run dev -- --hostname 127.0.0.1 --port 18100
```

Open `http://localhost:18100`. This serves the UI locally and reads the existing public backend. QRL price uses USD or the selected display currency; testnet Quanta has no market value. Gas displays the average recent transaction price from the gas summary endpoint. Unavailable summaries display a dash.

## Repeatable browser checks

```sh
npx playwright install chromium
npm run test:e2e
```

The browser suite starts a loopback fixture API and local frontend on ports 18091 and 18090. Fixtures cover exact zero quantities, NFT token ID zero, stable timestamps, and API errors. They are isolated from the live-data preview. `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` can select an existing Chromium installation.

Coverage includes sticky search and routing, hover and keyboard menus, mobile navigation and desktop resizing, responsive overflow, all appearance modes, system changes, cross-tab synchronization, storage failures, address and timestamp preferences, four languages, currency conversion/fallbacks, footer spacing, precise address highlighting, and token filtering. To test a production build, build with both handler environment variables set to `http://127.0.0.1:18091`, then run with `E2E_SERVER_COMMAND='npm run start -- --hostname 127.0.0.1 --port 18090'`.

The embedded TradingView script is stubbed in automated tests. Its real integration is checked separately in the live-data preview.

## Review screenshots

Captured from the local preview connected to the public backend:

- [Desktop home](images/horizontal-ui-home.png)
- [Light settings](images/horizontal-ui-settings-light.png)
- [Spanish settings in Dim appearance](images/horizontal-ui-settings-spanish.png)
- [Mobile navigation](images/horizontal-ui-mobile-menu.png)
